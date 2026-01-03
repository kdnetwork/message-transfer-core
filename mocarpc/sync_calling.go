// unable to fix this feature

package mocarpc

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type SyncMocaRPCType struct {
	ID           string
	CallbackChan chan *MocaJsonRPCResponse

	// ctx
	Ctx       context.Context
	CtxCancel context.CancelFunc
}

// TODO not yet support batch
func (corectx *MocaJsonRPCCtx) SendSyncRPCMessage(ctx context.Context, message *MocaJsonRPCBase) (*MocaJsonRPCResponse, error) {
	if message == nil {
		return nil, errors.New("mockrpc: message is nil")
	}

	if corectx.WriteMessage == nil {
		return nil, errors.New("mockrpc: WriteMessage is nil")
	}

	if message.ID == nil {
		return nil, errors.New("mockrpc: message ID is nil")
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)

	syncRPCStruct := &SyncMocaRPCType{
		ID:           string(message.ID),
		Ctx:          ctx,
		CtxCancel:    cancel,
		CallbackChan: make(chan *MocaJsonRPCResponse),
	}
	defer close(syncRPCStruct.CallbackChan)

	corectx.SyncMap.Set(syncRPCStruct.ID, syncRPCStruct, ttlcache.DefaultTTL)
	defer corectx.SyncMap.Delete(syncRPCStruct.ID)

	if err = corectx.WriteMessage(messageBytes); err != nil {
		return nil, err
	}

	select {
	case response := <-syncRPCStruct.CallbackChan:
		cancel()
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
