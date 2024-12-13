package btcclient

type BtcInterface interface {
	GetBlockCount() (int64, error)
	GetBlockTimestamp(height uint64) (int64, error)
}
