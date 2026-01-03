package mocarpc

import (
	"encoding/json"

	"github.com/google/uuid"
)

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

type MocaJsonRPCBase struct {
	// JsonRPC string `json:"jsonrpc"`
	Method string          `json:"method,omitempty"`
	ID     json.RawMessage `json:"id,omitempty"`

	// Request
	Params json.RawMessage `json:"params,omitempty"`

	// Response
	Result any               `json:"result,omitempty"`
	Error  *MocaJsonRPCError `json:"error,omitempty"`
}

type MocaJsonRPCResponse struct {
	*MocaJsonRPCBase
	Result json.RawMessage `json:"result,omitempty"`
}

func (ctx *MocaJsonRPCBase) GetID() any {
	return ctx.ID
}

func (ctx *MocaJsonRPCBase) GetMethod() string {
	return ctx.Method
}

type MocaJsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (corectx *MocaJsonRPCCtx) RequestBuilder(method string, params ...any) *MocaJsonRPCBase {
	req := &MocaJsonRPCBase{
		// JsonRPC: "2.0",
		Method: method,
		ID:     json.RawMessage("\"" + uuid.NewString() + "\""),
	}
	if len(params) == 1 {
		// TODO err...
		rawJson, _ := json.Marshal(params[0])
		req.Params = rawJson
	} else if len(params) > 1 {
		rawJson, _ := json.Marshal(params)
		req.Params = rawJson
	}

	return req
}
