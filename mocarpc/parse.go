package mocarpc

import (
	"encoding/json"
	"errors"
	"strings"
)

const (
	MocaRPCMessageTypeInvalid  int8 = -1
	MocaRPCMessageTypeRequest  int8 = 0
	MocaRPCMessageTypeResponse int8 = 1
)

type ParseMessageStruct struct {
	Message     *MocaJsonRPCResponse
	ErrorCode   int
	Error       error
	RequestType int8
}

func (corectx *MocaJsonRPCCtx) Parse(in []byte) *ParseMessageStruct {
	if len(in) == 0 || string(in[0]) != "{" {
		return &ParseMessageStruct{nil, ParseError, errors.New("empty input"), MocaRPCMessageTypeInvalid}
	}

	var unknownReq = json.RawMessage{}
	if err := json.Unmarshal(in, &unknownReq); err != nil {
		return &ParseMessageStruct{nil, ParseError, err, MocaRPCMessageTypeInvalid}
	}

	req, c, messageType, err := corectx.StaticCheck(unknownReq)

	return &ParseMessageStruct{req, c, err, messageType}
}

// TODO batch rpc calling
func (corectx *MocaJsonRPCCtx) ParseBatch(in []byte) []*ParseMessageStruct {
	var res = []*ParseMessageStruct{}
	if len(in) == 0 || string(in[0]) != "[" {
		res = append(res, &ParseMessageStruct{nil, ParseError, errors.New("empty input"), MocaRPCMessageTypeInvalid})
		return res
	}

	// TODO multiple errors
	var req []json.RawMessage
	if err := json.Unmarshal(in, &req); err != nil {
		res = append(res, &ParseMessageStruct{nil, ParseError, err, MocaRPCMessageTypeInvalid})
		return res
	}

	reqLen := len(req)
	if reqLen == 0 {
		res = append(res, &ParseMessageStruct{nil, ParseError, errors.New("empty request"), MocaRPCMessageTypeInvalid})
		return res
	}

	for i := range reqLen {
		validReq, c, messageType, err := corectx.StaticCheck(req[i])
		res = append(res, &ParseMessageStruct{validReq, c, err, messageType})
	}

	return res
}

func (corectx *MocaJsonRPCCtx) StaticCheck(unknownIn json.RawMessage) (*MocaJsonRPCResponse, int, int8, error) {
	var in = new(MocaJsonRPCResponse)
	err := json.Unmarshal(unknownIn, in)

	if err != nil {
		return nil, InvalidRequest, MocaRPCMessageTypeInvalid, errors.New("invalid message format")
	}

	messageType := MocaRPCMessageTypeInvalid

	// if in.JsonRPC != "2.0" {
	// 	return ParseError, errors.New("jsonrpc version not supported")
	// }

	if in.Method != "" {
		// request
		if in.Result != nil || in.Error != nil {
			return nil, InvalidRequest, messageType, errors.New("request must not have result or error")
		}

		if _, ok := corectx.Methods[in.Method]; !ok {
			return nil, MethodNotFound, messageType, errors.New("method not found")
		}

		messageType = MocaRPCMessageTypeRequest
	} else {
		// response
		if in.ID == nil {
			return nil, InvalidRequest, messageType, errors.New("response must have id")
		}
		if (in.Result == nil) == (in.Error == nil) {
			return nil, InvalidRequest, messageType, errors.New("response must have exactly one of result or error")
		}

		messageType = MocaRPCMessageTypeResponse
	}

	return in, 0, messageType, nil
}

// by chatgpt
func IsJSONArrayFast(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return false
	}
	return json.Valid([]byte(s))
}
