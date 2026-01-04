package mocarpc

import "errors"

type MocaRPCMethod func(in *MocaJsonRPCBase) (*MocaJsonRPCBase, int, error)

func (corectx *MocaJsonRPCCtx) MocaRPCMethodFunc(in *MocaJsonRPCBase) (*MocaJsonRPCBase, int, error) {
	if handler, exists := corectx.Methods[in.Method]; exists {
		res, errorCode, err := handler(in)

		if err != nil {
			// TODO params parse error
			return corectx.RsponseBuilder(in.ID, &MocaJsonRPCError{
				Code:    InternalError,
				Message: ErrorsMap[InternalError], // TODO more information...
			}), errorCode, err
		}

		return res, 0, nil
	}

	return corectx.RsponseBuilder(in.ID, &MocaJsonRPCError{
		Code:    MethodNotFound,
		Message: ErrorsMap[MethodNotFound], // TODO more information...
	}), MethodNotFound, errors.New("no method")
}

func (corectx *MocaJsonRPCCtx) RegisterMethod(method string, handler MocaRPCMethod) {
	corectx.Methods[method] = handler
}
