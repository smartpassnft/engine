/*
	- Timestampvm - https://github.com/ava-labs/timestampvm/blob/main/timestampvm/vm.go
*/
package tvm

import (
	"errors"
	"github.com/smartpassnft/tvm/components"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/version"
)

const (
	dataLen      = 32
	codecVersion = 0
	name         = "ticketvm"
)

var (
	errNoPendingBlocks               = errors.New("no block to propose")
	errBadGenesisBytes               = errors.New("genesis data should be bytes (max length 32)")
	Version                          = version.NewDefaultVersion(1, 0, 0)
	_, block.ChainVM = &VM{}
)

type VM struct {
	components.SnowstormVM
	codec codec.Manager
	mempool [][dataLen]byte
}