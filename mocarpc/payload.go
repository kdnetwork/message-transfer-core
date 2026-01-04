package mocarpc

import (
	"encoding/json"
)

type MocaJsonRPCBase struct {
	JsonRPC string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`

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

func (corectx *MocaJsonRPCCtx) RequestBuilder(id, method string, params ...any) *MocaJsonRPCBase {
	req := &MocaJsonRPCBase{
		Method: method,
	}

	if id != "" {
		req.ID = json.RawMessage("\"" + id + "\"")
	}

	if corectx.UseJsonRPC2 {
		req.JsonRPC = "2.0"
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

func (corectx *MocaJsonRPCCtx) RsponseBuilder(id json.RawMessage, _error *MocaJsonRPCError, results ...any) *MocaJsonRPCBase {
	res := &MocaJsonRPCBase{
		ID:    id,
		Error: _error,
	}

	if corectx.UseJsonRPC2 {
		res.JsonRPC = "2.0"
	}

	if res.Error != nil {
		return res
	}
	if len(results) == 1 {
		// TODO err...
		rawJson, _ := json.Marshal(results[0])
		res.Result = rawJson
	} else if len(results) > 1 {
		rawJson, _ := json.Marshal(results)
		res.Result = rawJson
	}

	return res
}

func (corectx *MocaJsonRPCCtx) NullIDErrorBuilder(id string, errorCode int) []byte {
	errorMessage := ErrorsMap[errorCode]
	if errorMessage == "" {
		errorMessage = "unknown error"
	}

	res, _ := json.Marshal(corectx.RsponseBuilder(json.RawMessage("null"), &MocaJsonRPCError{
		Code:    errorCode,
		Message: errorMessage,
	}))
	return res
}

func (ctx *MocaJsonRPCBase) ParseParams(params json.RawMessage, target any) (int, error) {
	if err := json.Unmarshal(params, target); err != nil {
		return InvalidParams, err
	}
	return 0, nil
}

func (ctx *MocaJsonRPCResponse) ToMap() (map[string]any, int, error) {
	var target map[string]any
	code, err := ctx.ParseParams(ctx.Result, &target)

	return target, code, err
}

func (ctx *MocaJsonRPCResponse) ToArray() ([]any, int, error) {
	var target []any
	code, err := ctx.ParseParams(ctx.Result, &target)

	return target, code, err
}
