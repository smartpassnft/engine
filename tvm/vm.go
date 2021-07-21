/*
	- Timestampvm - https://github.com/ava-labs/timestampvm/blob/main/timestampvm/vm.go
*/
package tvm

import (
	"errors"
	"fmt"
	"net/rpc"
	"time"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/avalanche"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/components/core"
	"github.com/prometheus/common/log"
	"github.com/smartpassnft/tvm/components"
)

const (
	dataLen      = 32
	codecVersion = 0
	name         = "ticketvm"
)

var (
	errNoPendingBlocks = errors.New("no block to propose")
	errBadGenesisBytes = errors.New("genesis data should be bytes (max length 32)")
	Version            = version.NewDefaultVersion(1, 0, 0)
	// _                  block.ChainVM = &components.SnowstormVM{}
)

type VM struct {
	components.SnowstormVM
	codec   codec.Manager
	mempool [][dataLen]byte
}

func (vm *VM) Initialize(
	ctx *snow.Context,
	dbManager manager.Manager,
	genesisData []byte,
	upgradeData []byte,
	configData []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
) error {
	version, err := vm.Version()
	if err != nil {
		log.Error("error initializing ticketvm: %v", err)
		return err
	}
	log.Info("Initializing ticketvm %v", version)
	if err := vm.SnowstormVM.Initialize(ctx, dbManager.Current().Database, vm.ParseBlock, toEngine); err != nil {
		log.Error("error initializing ticketvm: %v", err)
	}
	c := linearcodec.NewDefault()
	manager := codec.NewDefaultManager()
	if err := manager.RegisterCodec(codecVersion, c); err != nil {
		return err
	}
	vm.codec = manager

	if vm.DBInitialized() {
		if len(genesisData) > dataLen {
			return errBadGenesisBytes
		}

		var genesisDataArr [dataLen]byte
		copy(genesisDataArr[:], genesisData)

		genesisBlock, err := vm.NewBlock(ids.Empty, 0, genesisDataArr, time.Unix(0, 0))
		if err != nil {
			log.Error("error while creating genesis block: %v", err)
			return err
		}

		if err := vm.SaveBlock(vm.DB, genesisBlock); err != nil {
			log.Error("error while saving genesis block: %v", err)
			return err
		}

		if err := genesisBlock.Accept(); err != nil {
			log.Error("error while accepting genesis block: %w", err)
			return err
		}

		if vm.SetDBInitialized(); err != nil {
			log.Error("error while setting db to initialize: %w", err)
			return err
		}

		if err := vm.DB.Commit(); err != nil {
			log.Error("error while commiting db: %v", err)
			return err
		}
	}
	return nil
}

// CreateHandlers returns a map where:
// Keys: The path extension for this VM's API (empty in this case)
// Values: The handler for the API
func (vm *VM) CreateHandlers() (map[string]*common.HTTPHandler, error) {
	handler, err := vm.NewHandler(Name, &Service{vm})
	return map[string]*common.HTTPHandler{
		"": handler,
	}, err
}

// CreateStaticHandlers returns a map where:
// Keys: The path extension for this VM's static API
// Values: The handler for that static API
// We return nil because this VM has no static API
// CreateStaticHandlers implements the common.StaticVM interface
func (vm *VM) CreateStaticHandlers() (map[string]*common.HTTPHandler, error) {
	newServer := rpc.NewServer()
	codec := cjson.NewCodec()
	newServer.RegisterCodec(codec, "application/json")
	newServer.RegisterCodec(codec, "application/json;charset=UTF-8")

	// name this service "timestamp"
	staticService := CreateStaticService()
	return map[string]*common.HTTPHandler{
		"": {LockOptions: common.WriteLock, Handler: newServer},
	}, newServer.RegisterService(staticService, Name)
}

// Health implements the common.VM interface
func (vm *VM) HealthCheck() (interface{}, error) { return nil, nil }

// BuildBlock returns a block that this vm wants to add to consensus
func (vm *VM) BuildBlock() (avalanche.Vertex, error) {
	if len(vm.mempool) == 0 { // There is no block to be built
		return nil, errNoPendingBlocks
	}

	// Get the value to put in the new block
	value := vm.mempool[0]
	vm.mempool = vm.mempool[1:]

	// Notify consensus engine that there are more pending data for blocks
	// (if that is the case) when done building this block
	if len(vm.mempool) > 0 {
		defer vm.NotifyBlockReady()
	}

	// Gets Preferred Block
	preferredIntf, err := vm.GetBlock(vm.Preferred())
	if err != nil {
		return nil, fmt.Errorf("couldn't get preferred block: %w", err)
	}
	preferredHeight := preferredIntf.(*Block).Height()

	// Build the block with preferred height
	block, err := vm.NewBlock(vm.Preferred(), preferredHeight+1, value, time.Now())
	if err != nil {
		return nil, fmt.Errorf("couldn't build block: %w", err)
	}

	// Verifies block
	if err := block.Verify(); err != nil {
		return nil, err
	}
	return block, nil
}

// proposeBlock appends [data] to [p.mempool].
// Then it notifies the consensus engine
// that a new block is ready to be added to consensus
// (namely, a block with data [data])
func (vm *VM) proposeBlock(data [dataLen]byte) {
	vm.mempool = append(vm.mempool, data)
	vm.NotifyBlockReady()
}

// ParseBlock parses [bytes] to a avalanche.Vertex
// This function is used by the vm's state to unmarshal blocks saved in state
// and by the consensus layer when it receives the byte representation of a block
// from another node
func (vm *VM) ParseBlock(bytes []byte) (avalanche.Vertex, error) {
	// A new empty block
	block := &Block{}

	// Unmarshal the byte repr. of the block into our empty block
	_, err := vm.codec.Unmarshal(bytes, block)
	if err != nil {
		return nil, err
	}

	// Initialize the block
	// (Block inherits Initialize from its embedded *core.Block)
	block.Initialize(bytes, &vm.SnowstormVM)

	// Return the block
	return block, nil
}

// NewBlock returns a new Block where:
// - the block's parent is [parentID]
// - the block's data is [data]
// - the block's timestamp is [timestamp]
// The block is persisted in storage
func (vm *VM) NewBlock(parentID ids.ID, height uint64, data [dataLen]byte, timestamp time.Time) (*Block, error) {
	// Create our new block
	block := &Block{
		Block:     core.NewBlock(parentID, height),
		Data:      data,
		Timestamp: timestamp.Unix(),
	}

	// Get the byte representation of the block
	blockBytes, err := vm.codec.Marshal(codecVersion, block)
	if err != nil {
		return nil, err
	}

	// Initialize the block by providing it with its byte representation
	// and a reference to SnowstormVM
	block.Initialize(blockBytes, &vm.SnowstormVM)
	return block, nil
}

// Returns this VM's version
func (vm *VM) Version() (string, error) {
	return Version.String(), nil
}

func (vm *VM) Connected(id ids.ShortID) error {
	return nil // noop
}

func (vm *VM) Disconnected(id ids.ShortID) error {
	return nil // noop
}
