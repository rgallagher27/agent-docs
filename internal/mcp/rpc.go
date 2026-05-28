package mcp

import "encoding/json"

const jsonRPCVersion = "2.0"

// rpcMessage is the on-the-wire JSON-RPC 2.0 envelope. The presence
// of Method makes it a request or notification; ID + (Result|Error)
// makes it a response. A nil ID indicates a notification (no response
// expected).
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC 2.0 standard error codes.
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

func errorResponse(id json.RawMessage, code int, msg string) *rpcMessage {
	return &rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	}
}

// resultResponse marshals result and wraps it in an rpcMessage.
// Returns an internal-error rpcMessage if marshal fails — callers
// should never see an unwrapped error from this path.
func resultResponse(id json.RawMessage, result any) *rpcMessage {
	raw, err := json.Marshal(result)
	if err != nil {
		return errorResponse(id, errInternal, "marshal result: "+err.Error())
	}
	return &rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  raw,
	}
}
