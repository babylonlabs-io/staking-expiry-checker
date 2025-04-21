package btcclient

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/davecgh/go-spew/spew"
)

func TestBtcClient(t *testing.T) {
	t.Skip()
	client, err := NewBtcClient(cfg)
	require.NoError(t, err)

	spew.Dump(client.GetBlockCount())
}
