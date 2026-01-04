package mocarpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
)

type MocaJsonRPCCtx struct {
	Methods map[string]MocaRPCMethod

	SyncMap *ttlcache.Cache[string, *SyncMocaRPCType]

	ReadMessageChan chan *ReadMessageChanStruct
	WriteMessage    func(string, int, []byte) error

	GlobalContext       context.Context
	GlobalContextCancel context.CancelFunc

	// settings
	// IgnoreInvalidRequest bool
	UseJsonRPC2 bool
}

type ReadMessageChanStruct struct {
	ID      string
	Message []byte
}

func (corectx *MocaJsonRPCCtx) ReadMessage(message []byte) string {
	id := uuid.NewString()
	corectx.ReadMessageChan <- &ReadMessageChanStruct{
		Message: message,
		ID:      id,
	}

	return id
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
		ReadMessageChan: make(chan *ReadMessageChanStruct, 2000),

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
	for messageStruct := range corectx.ReadMessageChan {
		message := strings.TrimSpace(string(messageStruct.Message))
		// parse
		if len(message) <= 2 || !strings.HasPrefix(message, "{") || !strings.HasPrefix(message, "[") {
			slog.Debug("mocarpc", "id", messageStruct.ID, "original_message", message)

			res := corectx.NullIDErrorBuilder(messageStruct.ID, ParseError)
			if corectx.WriteMessage != nil {
				if err := corectx.WriteMessage(messageStruct.ID, ParseError, res); err != nil {
					slog.Error("mocarpc", "write error:", err)
				}
			}
		}

		// TODO prevent loop reading
		if IsJSONArrayFast(message) {
			parsedData := corectx.ParseBatch(messageStruct.Message)
			if len(parsedData) == 1 && parsedData[0].ErrorCode != 0 {
				slog.Debug("mocarpc", "id", messageStruct.ID, "original_message", message, "parsed_message", parsedData)

				res := corectx.NullIDErrorBuilder(messageStruct.ID, ParseError)
				if corectx.WriteMessage != nil {
					if err := corectx.WriteMessage(messageStruct.ID, ParseError, res); err != nil {
						slog.Error("mocarpc", "write error:", err)
					}
				}
				continue
			} else if len(parsedData) == 0 {
				slog.Debug("mocarpc", "id", messageStruct.ID, "original_message", message, "parsed_message", parsedData)

				res := corectx.NullIDErrorBuilder(messageStruct.ID, InvalidRequest)
				if corectx.WriteMessage != nil {
					if err := corectx.WriteMessage(messageStruct.ID, InvalidRequest, res); err != nil {
						slog.Error("mocarpc", "write error:", err)
					}
				}
				continue
			}

			if parsedData[0].RequestType == MocaRPCMessageTypeRequest {
				go func() {
					res := []*MocaJsonRPCBase{}

					for _, reqStruct := range parsedData {
						if reqStruct.ErrorCode != 0 {
							slog.Error("mocarpc", "error:", reqStruct.Error)
							res = append(res, corectx.RsponseBuilder(reqStruct.Message.ID, &MocaJsonRPCError{
								Code:    reqStruct.ErrorCode,
								Message: reqStruct.Error.Error(),
							}))
							continue
						} else {
							r, _, err := corectx.MocaRPCMethodFunc(reqStruct.Message.MocaJsonRPCBase)

							if err != nil {
								slog.Error("mocarpc", "err:", err)
							}

							if r == nil {
								r = corectx.RsponseBuilder(reqStruct.Message.ID, &MocaJsonRPCError{
									Code:    InvalidRequest,
									Message: ErrorsMap[InvalidRequest],
								})
							}

							res = append(res, r)
						}
					}
					responseBytes, err := json.Marshal(res)
					if err != nil {
						slog.Error("mocarpc", "marshal error:", err)
						return
					}
					if corectx.WriteMessage != nil {
						if err := corectx.WriteMessage(messageStruct.ID, 0, responseBytes); err != nil {
							slog.Error("mocarpc", "write error:", err)
						}
					}
				}()
			} else {
				if syncCall := corectx.SyncMap.Get(string(parsedData[0].Message.ID)); syncCall != nil {
					batchRes := []*MocaJsonRPCResponse{}
					for _, pd := range parsedData {
						batchRes = append(batchRes, pd.Message)
					}

					syncCall.Value().CallbackBatchChan <- batchRes
				}
			}
		} else {
			parsedData := corectx.Parse(messageStruct.Message)

			if parsedData.RequestType == MocaRPCMessageTypeRequest {
				if parsedData.Error != nil {
					slog.Error("mocarpc", "error:", parsedData.Error)
					return
				}

				go func() {
					r, code, err := corectx.MocaRPCMethodFunc(parsedData.Message.MocaJsonRPCBase)

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
						if err := corectx.WriteMessage(messageStruct.ID, code, responseBytes); err != nil {
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
