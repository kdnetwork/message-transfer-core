package mocarpc

import "errors"

type MocaRPCMethod func(in *MocaJsonRPCBase) (*MocaJsonRPCBase, error)

func (ctx *MocaJsonRPCCtx) MocaRPCMethodFunc(in *MocaJsonRPCBase) (*MocaJsonRPCBase, error) {
	if handler, exists := ctx.Methods[in.Method]; exists {
		res, err := handler(in)

		if err != nil {
			// TODO params parse error
			return &MocaJsonRPCBase{
				// JsonRPC: "2.0",
				ID: in.ID,
				Error: &MocaJsonRPCError{
					Code:    InternalError,
					Message: "failed", // TODO more information...
				},
			}, err
		}

		return res, nil
	}

	return &MocaJsonRPCBase{
		// JsonRPC: "2.0",
		ID: in.ID,
		Error: &MocaJsonRPCError{
			Code:    -32601,
			Message: "method not found",
		},
	}, errors.New("no method")
}

func (ctx *MocaJsonRPCCtx) RegisterMethod(method string, handler MocaRPCMethod) {
	ctx.Methods[method] = handler
}
