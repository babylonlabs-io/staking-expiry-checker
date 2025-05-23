package btcclient

import (
	"strings"
	"sync"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

type rpcClientWithReconnect struct {
	cfg       *rpcclient.ConnConfig
	rpcClient *rpcclient.Client
	mx        sync.RWMutex
}

func newRPCClientWithReconect(cfg *rpcclient.ConnConfig) (*rpcClientWithReconnect, error) {
	client, err := rpcclient.New(cfg, nil)
	if err != nil {
		return nil, err
	}

	return &rpcClientWithReconnect{
		cfg:       cfg,
		rpcClient: client,
	}, nil
}

func (r *rpcClientWithReconnect) GetBlockCount() (int64, error) {
	r.mx.RLock()
	block, err := r.rpcClient.GetBlockCount()
	r.mx.RUnlock()

	if err != nil {
		if r.isTimeout(err) {
			r.reconnect()
			return r.rpcClient.GetBlockCount()
		}

		return 0, err
	}

	return block, nil
}

func (r *rpcClientWithReconnect) GetBlockHash(block int64) (*chainhash.Hash, error) {
	r.mx.RLock()
	hash, err := r.rpcClient.GetBlockHash(block)
	r.mx.RUnlock()

	if err != nil {
		if r.isTimeout(err) {
			r.reconnect()
			return r.rpcClient.GetBlockHash(block)
		}

		return nil, err
	}

	return hash, nil
}

func (r *rpcClientWithReconnect) GetBlockHeader(hash *chainhash.Hash) (*wire.BlockHeader, error) {
	r.mx.RLock()
	result, err := r.rpcClient.GetBlockHeader(hash)
	r.mx.RUnlock()

	if err != nil {
		if r.isTimeout(err) {
			r.reconnect()
			return r.rpcClient.GetBlockHeader(hash)
		}

		return nil, err
	}

	return result, nil
}

func (r *rpcClientWithReconnect) GetTxOut(hash *chainhash.Hash, index uint32, mempool bool) (*btcjson.GetTxOutResult, error) {
	r.mx.RLock()
	result, err := r.rpcClient.GetTxOut(hash, index, mempool)
	r.mx.RUnlock()

	if err != nil {
		if r.isTimeout(err) {
			r.reconnect()
			return r.rpcClient.GetTxOut(hash, index, mempool)
		}

		return nil, err
	}

	return result, nil
}

func (r *rpcClientWithReconnect) GetBlock(hash *chainhash.Hash) (*wire.MsgBlock, error) {
	r.mx.RLock()
	block, err := r.rpcClient.GetBlock(hash)
	r.mx.RUnlock()

	if err != nil {
		if r.isTimeout(err) {
			r.reconnect()
			return r.rpcClient.GetBlock(hash)
		}

		return nil, err
	}

	return block, nil
}

func (r *rpcClientWithReconnect) reconnect() {
	client, err := rpcclient.New(r.cfg, nil)
	if err != nil {
		// we don't need to modify r.rpcClient here
		// otherwise it can trigger nil pointer dereference in other methods
		return
	}

	r.mx.Lock()
	r.rpcClient = client
	r.mx.Unlock()
}

func (r *rpcClientWithReconnect) isTimeout(err error) bool {
	if err == nil {
		return false
	}

	// unfortunately rpcclient returns root cause like string, errors.Is or errors.As won't help
	return strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers")
}
