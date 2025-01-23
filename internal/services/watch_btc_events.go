package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/networks/parameters/parser"
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
	quitCtx, cancel := s.quitContext()
	defer cancel()

	// Get spending details
	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", stakingTxHashHex).
			Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
			Msg("staking tx has been spent")
		err := s.handleSpendingStakingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			uint32(spendDetail.SpendingHeight),
			spendDetail.SpenderInputIndex,
			stakingTxHashHex,
		)
		if err != nil {
			log.Error().
				Err(err).
				Stack().
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
	stakingTxHashHex string,
) {
	quitCtx, cancel := s.quitContext()
	defer cancel()

	// Get spending details
	select {
	case spendDetail := <-spendEvent.Spend:
		log.Debug().
			Str("staking_tx", stakingTxHashHex).
			Msg("unbonding tx has been spent")
		err := s.handleSpendingUnbondingTransaction(
			quitCtx,
			spendDetail.SpendingTx,
			spendDetail.SpenderInputIndex,
			stakingTxHashHex,
		)
		if err != nil {
			log.Error().
				Err(err).
				Stack().
				Str("staking_tx", stakingTxHashHex).
				Str("spending_tx", spendDetail.SpendingTx.TxHash().String()).
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
	spendingHeight,
	spendingInputIdx uint32,
	stakingTxHashHex string,
) error {
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err)
	}

	paramsFromStakingTxHeight, err := s.getVersionedParams(delegation.StakingTx.StartHeight)
	if err != nil {
		return fmt.Errorf("failed to get versioned params from staking tx height: %w", err)
	}

	// First try to validate as unbonding tx
	isUnbonding, err := s.IsValidUnbondingTx(
		spendingTx,
		delegation,
		paramsFromStakingTxHeight,
	)
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
		unbondingTxHashHex := spendingTx.TxHash().String()
		unbondingStartHeight := uint64(spendingHeight)
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("unbonding_tx", unbondingTxHashHex).
			Msg("staking tx has been spent through unbonding path")

		unbondingTxHex, err := utils.SerializeBtcTransaction(spendingTx)
		if err != nil {
			return fmt.Errorf("failed to serialize unbonding tx: %w", err)
		}

		unbondingTxTimestamp, err := s.btc.GetBlockTimestamp(uint64(spendingHeight))
		if err != nil {
			return fmt.Errorf("failed to get block timestamp: %w", err)
		}

		unbondingEvent := types.NewUnbondingDelegationEvent(
			delegation.StakingTxHashHex,
			unbondingStartHeight,
			unbondingTxTimestamp,
			uint64(paramsFromStakingTxHeight.UnbondingTime),
			// valid unbonding tx always has one output
			uint64(0),
			unbondingTxHex,
			unbondingTxHashHex,
		)
		utils.PushOrQuit(s.unbondingDelegationChan, unbondingEvent, s.quit)

		// Register unbonding spend notification
		return s.registerUnbondingSpendNotification(stakingTxHashHex, unbondingTxHex, uint32(unbondingStartHeight))
	}

	// Try to validate as withdrawal transaction
	withdrawalErr := s.validateWithdrawalTxFromStaking(spendingTx, spendingInputIdx, delegation, paramsFromStakingTxHeight)
	if withdrawalErr != nil {
		if errors.Is(withdrawalErr, types.ErrInvalidWithdrawalTx) {
			metrics.IncrementInvalidStakingWithdrawalTxCounter()
			log.Error().
				Err(withdrawalErr).
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("found an invalid withdrawal tx from staking")

			return nil
		}

		log.Error().
			Err(withdrawalErr).
			Str("staking_tx", delegation.StakingTxHashHex).
			Msg("failed to validate withdrawal tx from staking")
		metrics.IncrementFailedVerifyingStakingWithdrawalTxCounter()
		return err
	}

	withdrawnEvent := types.NewWithdrawnDelegationEvent(delegation.StakingTxHashHex)
	utils.PushOrQuit(s.withdrawnDelegationChan, withdrawnEvent, s.quit)

	return nil
}

func (s *Service) handleSpendingUnbondingTransaction(
	ctx context.Context,
	spendingTx *wire.MsgTx,
	spendingInputIdx uint32,
	stakingTxHashHex string,
) error {
	delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHashHex)
	if err != nil {
		return fmt.Errorf("failed to get BTC delegation by staking tx hash: %w", err)
	}

	paramsFromStakingTxHeight, err := s.getVersionedParams(delegation.StakingTx.StartHeight)
	if err != nil {
		return err
	}

	// First try to validate as withdrawal transaction
	withdrawalErr := s.validateWithdrawalTxFromUnbonding(spendingTx, delegation, spendingInputIdx, paramsFromStakingTxHeight)
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

	withdrawnEvent := types.NewWithdrawnDelegationEvent(delegation.StakingTxHashHex)
	utils.PushOrQuit(s.withdrawnDelegationChan, withdrawnEvent, s.quit)

	return nil
}

// IsValidUnbondingTx tries to identify a tx is a valid unbonding tx
// It returns error when (1) it fails to verify the unbonding tx due
// to invalid parameters, and (2) the tx spends the unbonding path
// but is invalid
func (s *Service) IsValidUnbondingTx(
	tx *wire.MsgTx,
	delegation *model.DelegationDocument,
	params *parser.ParsedVersionedGlobalParams,
) (bool, error) {
	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTx.TxHex)
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
	if tx.TxIn[0].PreviousOutPoint.Index != uint32(delegation.StakingTx.OutputIndex) {
		return false, nil
	}

	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	fpPKBIP340, err := bbn.NewBIP340PubKeyFromHex(delegation.FinalityProviderPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
	}
	fpPK := fpPKBIP340.MustToBTCPK()

	btcParams, err := utils.GetBTCParams(s.cfg.Btc.NetParams)
	if err != nil {
		return false, fmt.Errorf("invalid BTC network params: %w", err)
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingTx.OutputIndex].Value)

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		[]*btcec.PublicKey{fpPK},
		params.CovenantPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTx.TimeLock),
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
		log.Debug().
			Str("staking_tx", delegation.StakingTxHashHex).
			Str("spending_tx", tx.TxHash().String()).
			Str("unbonding_path", hex.EncodeToString(unbondingPathInfo.GetPkScriptPath())).
			Str("script_from_witness", hex.EncodeToString(scriptFromWitness)).
			Msg("pk script from witness does not match unbonding path")
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
	unbondingFee := btcutil.Amount(params.UnbondingFee)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	if expectedUnbondingOutputValue <= 0 {
		return false, fmt.Errorf("%w: staking output value is too low, got %v, unbonding fee: %v",
			types.ErrInvalidUnbondingTx, stakingValue, params.UnbondingFee)
	}
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		[]*btcec.PublicKey{fpPK},
		params.CovenantPks,
		params.CovenantQuorum,
		params.UnbondingTime,
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
	delegation *model.DelegationDocument,
	params *parser.ParsedVersionedGlobalParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	fpPKBIP340, err := bbn.NewBIP340PubKeyFromHex(delegation.FinalityProviderPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
	}
	fpPK := fpPKBIP340.MustToBTCPK()

	btcParams, err := utils.GetBTCParams(s.cfg.Btc.NetParams)
	if err != nil {
		return fmt.Errorf("invalid BTC network params: %w", err)
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTx.TxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingTx.OutputIndex].Value)

	// 3. re-build the unbonding path script and check whether the script from
	// the witness matches
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerPk.MustToBTCPK(),
		[]*btcec.PublicKey{fpPK},
		params.CovenantPks,
		params.CovenantQuorum,
		uint16(delegation.StakingTx.TimeLock),
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
	delegation *model.DelegationDocument,
	spendingInputIdx uint32,
	params *parser.ParsedVersionedGlobalParams,
) error {
	stakerPk, err := bbn.NewBIP340PubKeyFromHex(delegation.StakerPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert staker btc pkh to a public key: %w", err)
	}

	fpPKBIP340, err := bbn.NewBIP340PubKeyFromHex(delegation.FinalityProviderPkHex)
	if err != nil {
		return fmt.Errorf("failed to convert finality provider pk hex to a public key: %w", err)
	}
	fpPK := fpPKBIP340.MustToBTCPK()

	btcParams, err := utils.GetBTCParams(s.cfg.Btc.NetParams)
	if err != nil {
		return fmt.Errorf("invalid BTC network params: %w", err)
	}

	stakingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.StakingTx.TxHex)
	if err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %w", err)
	}

	// re-build the time-lock path script and check whether the script from
	// the witness matches
	stakingValue := btcutil.Amount(stakingTx.TxOut[delegation.StakingTx.OutputIndex].Value)
	unbondingFee := btcutil.Amount(params.UnbondingFee)
	expectedUnbondingOutputValue := stakingValue - unbondingFee
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerPk.MustToBTCPK(),
		[]*btcec.PublicKey{fpPK},
		params.CovenantPks,
		params.CovenantQuorum,
		params.UnbondingTime,
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

	// Launch both registration and watching in a single goroutine
	// to save time for caller
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		spendEv, err := s.btcNotifier.RegisterSpendNtfn(
			&stakingOutpoint,
			stakingTx.TxOut[stakingOutputIdx].PkScript,
			stakingStartHeight,
		)
		if err != nil {
			// TODO: Handle the error in a better way such as retrying immediately
			// If continue to fail, we could retry by sending to queue and processing
			// later again to make sure we don't miss any spend
			// Will leave it as it is for now with alerts on log
			log.Error().
				Err(err).
				Str("staking_tx", stakingTxHashHex).
				Msg("failed to register spend notification")
			return
		}

		log.Debug().
			Str("staking_tx", stakingTxHashHex).
			Msg("registered staking spend notification")

		// Watch in the same goroutine
		s.watchForSpendStakingTx(spendEv, stakingTxHashHex)
	}()

	return nil
}

func (s *Service) registerUnbondingSpendNotification(
	stakingTxHashHex string,
	unbondingTxHex string,
	spendHeightHint uint32,
) error {
	unbondingTxBytes, parseErr := hex.DecodeString(unbondingTxHex)
	if parseErr != nil {
		return fmt.Errorf("failed to decode unbonding tx: %w", parseErr)
	}

	unbondingTx, parseErr := bbn.NewBTCTxFromBytes(unbondingTxBytes)
	if parseErr != nil {
		return fmt.Errorf("failed to parse unbonding tx: %w", parseErr)
	}

	unbondingOutpoint := wire.OutPoint{
		Hash:  unbondingTx.TxHash(),
		Index: 0, // unbonding tx has only 1 output
	}

	// Launch both registration and watching in a single goroutine
	// to save time for caller
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		spendEv, btcErr := s.btcNotifier.RegisterSpendNtfn(
			&unbondingOutpoint,
			unbondingTx.TxOut[0].PkScript,
			spendHeightHint,
		)
		if btcErr != nil {
			// TODO: Handle the error in a better way such as retrying immediately
			// If continue to fail, we could retry by sending to queue and processing
			// later again to make sure we don't miss any spend
			// Will leave it as it is for now with alerts on log
			log.Error().
				Err(btcErr).
				Str("staking_tx", stakingTxHashHex).
				Msg("failed to register early unbonding spend notification")
			return
		}

		log.Debug().
			Str("staking_tx", stakingTxHashHex).
			Str("unbonding_tx", unbondingTx.TxHash().String()).
			Msg("registered early unbonding spend notification")

		// Watch in the same goroutine
		s.watchForSpendUnbondingTx(spendEv, stakingTxHashHex)
	}()

	return nil
}
