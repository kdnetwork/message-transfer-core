package mocarpc

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

var ErrorsMap = map[int]string{
	ParseError:     "Parse error",
	InvalidRequest: "Invalid Request",
	MethodNotFound: "Method not found",
	InvalidParams:  "Invalid params",
	InternalError:  "Internal error",
}
