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
	ID                string
	CallbackChan      chan *MocaJsonRPCResponse
	CallbackBatchChan chan []*MocaJsonRPCResponse

	// ctx
	Ctx       context.Context
	CtxCancel context.CancelFunc
}

// TODO not yet support batch
func (corectx *MocaJsonRPCCtx) Call(ctx context.Context, message *MocaJsonRPCBase) (*MocaJsonRPCResponse, error) {
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
		CallbackChan: make(chan *MocaJsonRPCResponse, 1),
	}
	// defer close(syncRPCStruct.CallbackChan)

	corectx.SyncMap.Set(syncRPCStruct.ID, syncRPCStruct, ttlcache.DefaultTTL)
	defer corectx.SyncMap.Delete(syncRPCStruct.ID)

	if err = corectx.WriteMessage(syncRPCStruct.ID, 0, messageBytes); err != nil {
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

func (corectx *MocaJsonRPCCtx) CallBatch(ctx context.Context, message []*MocaJsonRPCBase) ([]*MocaJsonRPCResponse, error) {
	if len(message) == 0 {
		return nil, errors.New("mockrpc: empty message")
	}

	if corectx.WriteMessage == nil {
		return nil, errors.New("mockrpc: WriteMessage is nil")
	}

	if message[0].ID == nil {
		return nil, errors.New("mockrpc: message ID is nil")
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)

	syncRPCStruct := &SyncMocaRPCType{
		ID:                string(message[0].ID),
		Ctx:               ctx,
		CtxCancel:         cancel,
		CallbackBatchChan: make(chan []*MocaJsonRPCResponse, 1),
	}
	// defer close(syncRPCStruct.CallbackBatchChan)

	corectx.SyncMap.Set(syncRPCStruct.ID, syncRPCStruct, ttlcache.DefaultTTL)
	defer corectx.SyncMap.Delete(syncRPCStruct.ID)

	if err = corectx.WriteMessage(syncRPCStruct.ID, 0, messageBytes); err != nil {
		return nil, err
	}

	select {
	case response := <-syncRPCStruct.CallbackBatchChan:
		cancel()
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
