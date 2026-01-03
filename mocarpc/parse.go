package mocarpc

import (
	"encoding/json"
	"errors"
)

const (
	MocaRPCMessageTypeInvalid  int8 = -1
	MocaRPCMessageTypeRequest  int8 = 0
	MocaRPCMessageTypeResponse int8 = 1
)

func (ctx *MocaJsonRPCCtx) ParseParams(params json.RawMessage, target any) (int, error) {
	if err := json.Unmarshal(params, target); err != nil {
		return InvalidParams, err
	}
	return 0, nil
}

type ParseMessageStruct struct {
	Message     *MocaJsonRPCResponse
	ErrorCode   int
	Error       error
	RequestType int8
}

func (ctx *MocaJsonRPCCtx) Parse(in []byte) *ParseMessageStruct {
	if len(in) == 0 || string(in[0]) != "{" {
		return &ParseMessageStruct{nil, ParseError, errors.New("empty input"), MocaRPCMessageTypeInvalid}
	}

	var req = new(MocaJsonRPCResponse)
	if err := json.Unmarshal(in, req); err != nil {
		return &ParseMessageStruct{nil, ParseError, err, MocaRPCMessageTypeInvalid}
	}

	c, messageType, err := ctx.StaticCheck(req)

	return &ParseMessageStruct{req, c, err, messageType}
}

// TODO batch rpc calling
func (ctx *MocaJsonRPCCtx) ParseBatch(in []byte) []*ParseMessageStruct {
	var res = []*ParseMessageStruct{}
	if len(in) == 0 || string(in[0]) != "[" {
		res = append(res, &ParseMessageStruct{nil, ParseError, errors.New("empty input"), MocaRPCMessageTypeInvalid})
		return res
	}

	// TODO multiple errors
	var req []*MocaJsonRPCResponse
	if err := json.Unmarshal(in, &req); err != nil {
		res = append(res, &ParseMessageStruct{nil, ParseError, err, MocaRPCMessageTypeInvalid})
		return res
	}

	reqLen := len(req)

	if reqLen == 0 {
		res = append(res, &ParseMessageStruct{nil, ParseError, errors.New("empty request"), MocaRPCMessageTypeInvalid})
		return res
	}

	for i := 0; i < reqLen; i++ {
		c, messageType, err := ctx.StaticCheck(req[i])
		res = append(res, &ParseMessageStruct{req[i], c, err, messageType})
	}

	return res
}

func (ctx *MocaJsonRPCCtx) StaticCheck(in *MocaJsonRPCResponse) (int, int8, error) {
	messageType := MocaRPCMessageTypeInvalid
	if in == nil {
		return ParseError, messageType, errors.New("nil input")
	}

	// if in.JsonRPC != "2.0" {
	// 	return ParseError, errors.New("jsonrpc version not supported")
	// }

	if in.Method != "" {
		// request
		if in.Result != nil || in.Error != nil {
			return InvalidRequest, messageType, errors.New("request must not have result or error")
		}

		if _, ok := ctx.Methods[in.Method]; !ok {
			return MethodNotFound, messageType, errors.New("method not found")
		}

		messageType = MocaRPCMessageTypeRequest
	} else {
		// response
		if in.ID == nil {
			return InvalidRequest, messageType, errors.New("response must have id")
		}
		if (in.Result == nil) == (in.Error == nil) {
			return InvalidRequest, messageType, errors.New("response must have exactly one of result or error")
		}

		messageType = MocaRPCMessageTypeResponse
	}

	return 0, messageType, nil
}
