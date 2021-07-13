package tvm

import (
  "fmt"
  "time"
  "errors"

  "github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowstorm"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	cjson "github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/components/core"
)

/* 
DAG Reference 
- https://github.com/ava-labs/avalanchego/blob/v1.4.10/snow/engine/avalanche/vertex/vm.go
*/
type TVM struct {
  common.VM
  PendingTxs() []snowstorm.Tx
  ParseTx(tx []byte) (snowstorm.Tx, error)
  GetTx(ids.ID) (snowstorm.Tx, error)
}
