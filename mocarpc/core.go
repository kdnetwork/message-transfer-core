package mocarpc

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type MocaJsonRPCCtx struct {
	Methods map[string]MocaRPCMethod

	SyncMap *ttlcache.Cache[string, *SyncMocaRPCType]

	ReadMessageChan chan []byte
	WriteMessage    func([]byte) error

	GlobalContext       context.Context
	GlobalContextCancel context.CancelFunc
}

// TODO ttl
func InitMocaJsonRPCCtx(ctx context.Context) *MocaJsonRPCCtx {
	ctx, cancel := context.WithCancel(context.Background())
	corectx := &MocaJsonRPCCtx{
		Methods: make(map[string]MocaRPCMethod),
		SyncMap: ttlcache.New(
			ttlcache.WithDisableTouchOnHit[string, *SyncMocaRPCType](),
			ttlcache.WithTTL[string, *SyncMocaRPCType](time.Second*11),
		),
		ReadMessageChan: make(chan []byte, 2000),

		GlobalContext:       ctx,
		GlobalContextCancel: cancel,
	}

	go corectx.SyncMap.Start()
	go corectx.OnMessage()

	go func() {
		<-ctx.Done()
		corectx.SyncMap.Stop()
		close(corectx.ReadMessageChan)
	}()

	return corectx
}

func (corectx *MocaJsonRPCCtx) OnMessage() {
	for message := range corectx.ReadMessageChan {
		message = bytes.TrimSpace(message)
		// parse
		/// TODO check last code
		if len(message) == 0 || !slices.Contains([]string{"{", "["}, string(message[0])) {
			res, _ := json.Marshal(&MocaJsonRPCBase{
				// JsonRPC: "2.0",
				ID: json.RawMessage("null"),
				Error: &MocaJsonRPCError{
					Code:    -32601,
					Message: "method not found",
				},
			})
			slog.Error("mocarpc", "error:", string(res))
			if corectx.WriteMessage != nil {
				if err := corectx.WriteMessage(res); err != nil {
					slog.Error("mocarpc", "write error:", err)
				}
			}
		}

		firstCode := string(message[0])
		isBatch := firstCode == "["

		// batch mode not yet support nil response
		if isBatch {
			parsedData := corectx.ParseBatch(message)
			if len(parsedData) <= 1 && parsedData[0].Error != nil {
				slog.Error("mocarpc", "error:", parsedData[0].Error)
				return
			}

			go func() {
				res := make([]*MocaJsonRPCBase, len(parsedData))

				for i, reqStruct := range parsedData {
					r, err := corectx.Methods[reqStruct.Message.Method](reqStruct.Message.MocaJsonRPCBase)

					if err != nil {
						slog.Error("mocarpc", "err:", err)
					}

					res[i] = r
				}
				responseBytes, err := json.Marshal(res)
				if err != nil {
					slog.Error("mocarpc", "marshal error:", err)
					return
				}
				if corectx.WriteMessage != nil {
					if err := corectx.WriteMessage(responseBytes); err != nil {
						slog.Error("mocarpc", "write error:", err)
					}
				}
			}()
		} else {
			parsedData := corectx.Parse(message)

			if parsedData.RequestType == MocaRPCMessageTypeRequest {
				if parsedData.Error != nil {
					slog.Error("mocarpc", "error:", parsedData.Error)
					return
				}

				go func() {
					r, err := corectx.Methods[parsedData.Message.Method](parsedData.Message.MocaJsonRPCBase)

					if err != nil {
						slog.Error("mocarpc", "err:", err)
					}

					// restrn a nil will not response
					if r == nil {
						return
					}

					responseBytes, err := json.Marshal(r)
					if err != nil {
						slog.Error("mocarpc", "marshal error:", err)
						return
					}
					if corectx.WriteMessage != nil {
						if err := corectx.WriteMessage(responseBytes); err != nil {
							slog.Error("mocarpc", "write error:", err)
						}
					}
				}()
			} else {
				if syncCall := corectx.SyncMap.Get(string(parsedData.Message.ID)); syncCall != nil {
					syncCall.Value().CallbackChan <- parsedData.Message
				}
			}
		}
	}
}
