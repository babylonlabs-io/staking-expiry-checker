package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/rs/zerolog/log"
)

func (s *Service) watchForSpendStakingTx(
	spendEvent *notifier.SpendEvent,
	stakingTxHashHex string,
) {
	defer s.wg.Done()
	quitCtx, cancel := s.quitContext()
	defer cancel()

	// Get spending details
	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", stakingTxHashHex).
			Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
			Msg("staking tx has been spent")
		if err := s.handleSpendingStakingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			stakingTxHashHex,
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", stakingTxHashHex).
				Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
				Msg("failed to handle spending staking transaction")
			return
		}

	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}

}

func (s *Service) watchForSpendUnbondingTx(
	spendEvent *notifier.SpendEvent,
	delegation *model.BTCDelegationDetails,
) {
	defer s.wg.Done()
	quitCtx, cancel := s.quitContext()
	defer cancel()

	// Get spending details
	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("unbonding_tx", spendDetail.SpendingTx.TxHash().String()).
			Msg("unbonding tx has been spent")
		if err := s.handleSpendingUnbondingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			delegation,
		); err != nil {
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Str("unbonding_tx", spendDetail.SpendingTx.TxHash().String()).
				Msg("failed to handle spending unbonding transaction")
			return
		}

	case <-s.quit:
		return
	case <-quitCtx.Done():
		return
	}
}

func (s *Service) handleSpendingStakingTransaction(
	ctx context.Context,
	spendingTx *wire.MsgTx,
	spendingInputIdx uint32,
	stakingTxHashHex string,
) error {
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err)
	}

	params, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	// First try to validate as unbonding tx
	isUnbonding, err := s.IsValidUnbondingTx(spendingTx, delegation, params)
	if err != nil {
		if errors.Is(err, types.ErrInvalidUnbondingTx) {
			metrics.IncrementInvalidUnbondingTxCounter()
			log.Error().
				Err(err).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("found an invalid unbonding tx")

			return nil
		}

		metrics.IncrementFailedVerifyingUnbondingTxCounter()
		return fmt.Errorf("failed to validate unbonding tx: %w", err)
	}
	if isUnbonding {
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("unbonding_tx", spendingTx.TxHash().String()).
			Msg("staking tx has been spent through unbonding path")

		// Blocking send - will wait if channel is full
		select {
		case s.unbondingDelegationChan <- delegation:
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("sent delegation to unbonding handler")
		case <-ctx.Done():
			log.Error().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("context cancelled while waiting to send to unbonding channel")
			return ctx.Err()
		}

		// Register unbonding spend notification
		return s.registerUnbondingSpendNotification(delegation)
	}

	// Try to validate as withdrawal transaction
	withdrawalErr := s.validateWithdrawalTxFromStaking(spendingTx, spendingInputIdx, delegation, params)
	if withdrawalErr != nil {
		if errors.Is(err, types.ErrInvalidWithdrawalTx) {
			metrics.IncrementInvalidStakingWithdrawalTxCounter()
			log.Error().
				Err(withdrawalErr).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("found an invalid withdrawal tx from staking")

			return nil
		}

		metrics.IncrementFailedVerifyingStakingWithdrawalTxCounter()
		return err
	}

	// Blocking send - will wait if channel is full
	select {
	case s.withdrawnDelegationChan <- delegation:
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Msg("sent delegation to withdrawn handler")
	case <-ctx.Done():
		log.Error().
			Str("staking_tx", delegation.StakingTxHashHex).
			Msg("context cancelled while waiting to send to withdrawn channel")
		return ctx.Err()
	}

	return nil
}

func (s *Service) handleSpendingUnbondingTransaction(
	ctx context.Context,
	spendingTx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
) error {
	params, err := s.db.GetStakingParams(ctx, delegation.ParamsVersion)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	// First try to validate as withdrawal transaction
	withdrawalErr := s.validateWithdrawalTxFromUnbonding(spendingTx, delegation, spendingInputIdx, params)
	if withdrawalErr != nil {
		if errors.Is(withdrawalErr, types.ErrInvalidWithdrawalTx) {
			metrics.IncrementInvalidUnbondingWithdrawalTxCounter()
			log.Error().
				Err(withdrawalErr).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("found an invalid withdrawal tx from unbonding")

			return nil
		}

		metrics.IncrementFailedVerifyingUnbondingWithdrawalTxCounter()
		return fmt.Errorf("failed to validate withdrawal tx: %w", withdrawalErr)
	}

	// Blocking send - will wait if channel is full
	select {
	case s.withdrawnDelegationChan <- delegation:
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Msg("sent delegation to withdrawn handler")
	case <-ctx.Done():
		log.Error().
			Str("staking_tx", delegation.StakingTxHashHex).
			Msg("context cancelled while waiting to send to withdrawn channel")
		return ctx.Err()
	}

	return nil
}

// IsValidUnbondingTx tries to identify a tx is a valid unbonding tx
// It returns error when (1) it fails to verify the unbonding tx due
// to invalid parameters, and (2) the tx spends the unbonding path
// but is invalid
func (s *Service) IsValidUnbondingTx(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	params *model.StakingParams,
) (bool, error) {
	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return false, fmt.Errorf("failed to deserialize staking tx: %w", err)
	}
	stakingTxHash := stakingTx.TxHash()

	// 1. an unbonding tx must be a transfer tx
	if err := btcstaking.IsTransferTx(tx); err != nil {
		return false, nil
	}

	// 2. an unbonding tx must spend the staking output
	if !tx.TxIn[0].PreviousOutPoint.Hash.IsEqual(&stakingTxHash) {
		return false, nil
	}
	if tx.TxIn[0].PreviousOutPoint.Index != delegation.StakingOutputIdx {
		return false, nil
	}

	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams := utils.GetBTCParams(s.cfg.Btc.NetParams)

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		stakingValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the staking info: %w", err)
	}
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[0].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[0].Witness[len(tx.TxIn[0].Witness)-2]

	if !bytes.Equal(unbondingPathInfo.GetPkScriptPath(), scriptFromWitness) {
		// not unbonding tx as it does not unlock the unbonding path
		return false, nil
	}

	// 4. check whether the unbonding tx enables rbf has time lock
	if tx.TxIn[0].Sequence != wire.MaxTxInSequenceNum {
		return false, fmt.Errorf("%w: unbonding tx should not enable rbf", types.ErrInvalidUnbondingTx)
	}
	if tx.LockTime != 0 {
		return false, fmt.Errorf("%w: unbonding tx should not set lock time", types.ErrInvalidUnbondingTx)
	}

	// 5. check whether the script of an unbonding tx output is expected
	// by re-building unbonding output from params
	unbondingFee := btcutil.Amount(params.UnbondingFeeSat)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	if expectedUnbondingOutputValue <= 0 {
		return false, fmt.Errorf("%w: staking output value is too low, got %v, unbonding fee: %v",
			types.ErrInvalidUnbondingTx, stakingValue, params.UnbondingFeeSat)
	}
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.UnbondingTime),
		expectedUnbondingOutputValue,
		btcParams,
	)
	if err != nil {
		return false, fmt.Errorf("failed to rebuid the unbonding info: %w", err)
	}
	if !bytes.Equal(tx.TxOut[0].PkScript, unbondingInfo.UnbondingOutput.PkScript) {
		return false, fmt.Errorf("%w: the unbonding output is not expected", types.ErrInvalidUnbondingTx)
	}
	if tx.TxOut[0].Value != unbondingInfo.UnbondingOutput.Value {
		return false, fmt.Errorf("%w: the unbonding output value %d is not expected %d",
			types.ErrInvalidUnbondingTx, tx.TxOut[0].Value, unbondingInfo.UnbondingOutput.Value)
	}

	return true, nil
}

func (s *Service) validateWithdrawalTxFromStaking(
	tx *wire.MsgTx,
	spendingInputIdx uint32,
	delegation *model.BTCDelegationDetails,
	params *model.StakingParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams := utils.GetBTCParams(s.cfg.Btc.NetParams)

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTime),
		stakingValue,
		btcParams,
	)
	if err != nil {
		return fmt.Errorf("failed to rebuid the staking info: %w", err)
	}

	timelockPathInfo, err := stakingInfo.TimeLockPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(timelockPathInfo.GetPkScriptPath(), scriptFromWitness) {
		return fmt.Errorf("%w: the tx does not unlock the time-lock path", types.ErrInvalidWithdrawalTx)
	}

	return nil
}

func (s *Service) validateWithdrawalTxFromUnbonding(
	tx *wire.MsgTx,
	delegation *model.BTCDelegationDetails,
	spendingInputIdx uint32,
	params *model.StakingParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerBtcPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	finalityProviderPks := make([]*btcec.PublicKey, len(delegation.FinalityProviderBtcPksHex))
	for i, hex := range delegation.FinalityProviderBtcPksHex {
		fpPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		finalityProviderPks[i] = fpPk.MustToBTCPK()
	}

	covPks := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, hex := range params.CovenantPks {
		covPk, err := bbn.NewBIP340PubKeyFromHex(hex)
		if err != nil {
			return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
		}
		covPks[i] = covPk.MustToBTCPK()
	}

	btcParams := utils.GetBTCParams(s.cfg.Btc.NetParams)

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	// re-build the time-lock path script and check whether the script from
	// the witness matches
	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingOutputIdx].Value)
	unbondingFee := btcutil.Amount(params.UnbondingFeeSat)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		finalityProviderPks,
		covPks,
		params.CovenantQuorum,
		uint16(delegation.UnbondingTime),
		expectedUnbondingOutputValue,
		btcParams,
	)
	if err != nil {
		return fmt.Errorf("failed to rebuid the unbonding info: %w", err)
	}
	timelockPathInfo, err := unbondingInfo.TimeLockPathSpendInfo()
	if err != nil {
		return fmt.Errorf("failed to get the unbonding path spend info: %w", err)
	}

	witness := tx.TxIn[spendingInputIdx].Witness
	if len(witness) < 2 {
		panic(fmt.Errorf("spending tx should have at least 2 elements in witness, got %d", len(witness)))
	}

	scriptFromWitness := tx.TxIn[spendingInputIdx].Witness[len(tx.TxIn[spendingInputIdx].Witness)-2]

	if !bytes.Equal(timelockPathInfo.GetPkScriptPath(), scriptFromWitness) {
		return fmt.Errorf("%w: the tx does not unlock the time-lock path", types.ErrInvalidWithdrawalTx)
	}

	return nil
}

func (s *Service) quitContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	s.wg.Add(1)
	go func() {
		defer cancel()
		defer s.wg.Done()

		select {
		case <-s.quit:
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

func (s *Service) registerStakingSpendNotification(
	stakingTxHashHex string,
	stakingTxHex string,
	stakingOutputIdx uint32,
	stakingStartHeight uint32,
) error {
	stakingTxHash, err := chainhash.NewHashFromStr(stakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to parse staking tx hash: %w", err)
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(stakingTxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingOutpoint := wire.OutPoint{
		Hash:  *stakingTxHash,
		Index: stakingOutputIdx,
	}

	spendEv, err := s.btcNotifier.RegisterSpendNtfn(
		&stakingOutpoint,
		stakingTx.TxOut[stakingOutputIdx].PkScript,
		stakingStartHeight,
	)
	if err != nil {
		return fmt.Errorf("failed to register spend ntfn for staking tx %s: %w", stakingTxHashHex, err)
	}

	s.wg.Add(1)
	go s.watchForSpendStakingTx(spendEv, stakingTxHashHex)

	return nil
}

func (s *Service) registerUnbondingSpendNotification(
	delegation *model.BTCDelegationDetails,
) *types.Error {
	unbondingTxBytes, parseErr := hex.DecodeString(delegation.UnbondingTx)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to decode unbonding tx: %w", parseErr),
		)
	}

	unbondingTx, parseErr := bbn.NewBTCTxFromBytes(unbondingTxBytes)
	if parseErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to parse unbonding tx: %w", parseErr),
		)
	}

	log.Debug().
		Str("staking_tx", delegation.StakingTxHashHex).
		Str("unbonding_tx", unbondingTx.TxHash().String()).
		Msg("registering early unbonding spend notification")

	unbondingOutpoint := wire.OutPoint{
		Hash:  unbondingTx.TxHash(),
		Index: 0, // unbonding tx has only 1 output
	}

	spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
		&unbondingOutpoint,
		unbondingTx.TxOut[0].PkScript,
		delegation.StartHeight,
	)
	if btcErr != nil {
		return types.NewError(
			http.StatusInternalServerError,
			types.InternalServiceError,
			fmt.Errorf("failed to register spend ntfn for unbonding tx %s: %w", delegation.StakingTxHashHex, btcErr),
		)
	}

	s.wg.Add(1)
	go s.watchForSpendUnbondingTx(spendEv, delegation)

	return nil
}
