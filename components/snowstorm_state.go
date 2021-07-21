/*
	Custom VM interface for snowstorm
	https://github.com/ava-labs/avalanchego/blob/master/vms/components/core/snowman_state.go
*/
package components

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/avalanche"
	"github.com/ava-labs/avalanchego/vms/components/state"
)

var errWrongType = errors.New("got unexpected type from database")

// state.Get(Db, IDTypeID, lastAcceptedID) == ID of last accepted block
var lastAcceptedID = ids.ID{'l', 'a', 's', 't'}

// SnowstormState is a wrapper around state.State
// In additions to the methods exposed by state.State,
// SnowstormState exposes a few methods needed for managing
// state in a snowman vm
type SnowstormState interface {
	state.State
	GetBlock(database.Database, ids.ID) (avalanche.Vertex, error)
	PutBlock(database.Database, avalanche.Vertex) error
	GetLastAccepted(database.Database) (ids.ID, error)
	PutLastAccepted(database.Database, ids.ID) error
}

// implements SnowstormState
type snowstormState struct {
	state.State
}

// GetBlock gets the block with ID [ID] from [db]
func (s *snowstormState) GetBlock(db database.Database, id ids.ID) (avalanche.Vertex, error) {
	blockInterface, err := s.Get(db, state.BlockTypeID, id)
	if err != nil {
		return nil, err
	}

	if block, ok := blockInterface.(avalanche.Vertex); ok {
		return block, nil
	}
	return nil, errWrongType
}

// PutBlock puts [block] in [db]
func (s *snowstormState) PutBlock(db database.Database, block avalanche.Vertex) error {
	return s.Put(db, state.BlockTypeID, block.ID(), block)
}

// GetLastAccepted returns the ID of the last accepted block in [db]
func (s *snowstormState) GetLastAccepted(db database.Database) (ids.ID, error) {
	lastAccepted, err := s.GetID(db, lastAcceptedID)
	if err != nil {
		return ids.ID{}, err
	}
	return lastAccepted, nil
}

// PutLastAccepted sets the ID of the last accepted block in [db] to [lastAccepted]
func (s *snowstormState) PutLastAccepted(db database.Database, lastAccepted ids.ID) error {
	return s.PutID(db, lastAcceptedID, lastAccepted)
}

// NewSnowStormState returns a new SnowstormState
func NewSnowStormState(unmarshalBlockFunc func([]byte) (avalanche.Vertex, error)) (SnowstormState, error) {
	rawState, err := state.NewState()
	if err != nil {
		return nil, fmt.Errorf("error creating new state: %w", err)
	}
	snowstormState := &snowstormState{State: rawState}
	return snowstormState, rawState.RegisterType(
		state.BlockTypeID,
		func(b interface{}) ([]byte, error) {
			if block, ok := b.(avalanche.Vertex); ok {
				return block.Bytes(), nil
			}
			return nil, errors.New("expected snowman.Block but got unexpected type")
		},
		func(bytes []byte) (interface{}, error) {
			return unmarshalBlockFunc(bytes)
		},
	)
}
