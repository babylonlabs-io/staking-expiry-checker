package btcclient

import (
	"fmt"

	"github.com/avast/retry-go/v4"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
)

type BtcClient struct {
	client *rpcclient.Client
	cfg    *config.BTCConfig
}

func NewBtcClient(cfg *config.BTCConfig) (*BtcClient, error) {
	connCfg, err := cfg.ToConnConfig()
	if err != nil {
		return nil, err
	}

	rpcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	return &BtcClient{
		client: rpcClient,
		cfg:    cfg,
	}, nil
}

func (b *BtcClient) GetBlockCount() (int64, error) {
	return clientCallWithRetry(b.client.GetBlockCount, b.cfg)
}

func (b *BtcClient) GetBlockTimestamp(height uint64) (int64, error) {
	return clientCallWithRetry(func() (int64, error) {
		hash, err := b.client.GetBlockHash(int64(height))
		if err != nil {
			return 0, err
		}

		header, err := b.client.GetBlockHeader(hash)
		if err != nil {
			return 0, err
		}

		return header.Timestamp.Unix(), nil
	}, b.cfg)
}

func (b *BtcClient) IsUTXOSpent(txid string, vout uint32) (bool, error) {
	return clientCallWithRetry(func() (bool, error) {
		hash, err := chainhash.NewHashFromStr(txid)
		if err != nil {
			return false, fmt.Errorf("failed to deserialize tx: %w", err)
		}

		txOut, err := b.client.GetTxOut(hash, vout, false)
		if err != nil {
			return false, fmt.Errorf("failed to get txout: %w", err)
		}

		return txOut == nil, nil
	}, b.cfg)
}

func clientCallWithRetry[T any](
	call func() (T, error),
	cfg *config.BTCConfig,
) (T, error) {
	return metrics.RecordBtcClientMetrics(func() (T, error) {
		// Convert to pointer for retry.DoWithData
		callWithPointer := func() (*T, error) {
			result, err := call()
			if err != nil {
				return nil, err
			}
			return &result, nil
		}

		result, err := retry.DoWithData(callWithPointer,
			retry.Attempts(cfg.MaxRetryTimes),
			retry.Delay(cfg.RetryInterval),
			retry.LastErrorOnly(true),
			retry.OnRetry(func(n uint, err error) {
				log.Debug().
					Uint("attempt", n+1).
					Uint("max_attempts", cfg.MaxRetryTimes).
					Err(err).
					Msg("failed to call the RPC client")
			}))

		if err != nil {
			var zero T
			return zero, err
		}
		return *result, nil
	})
}
