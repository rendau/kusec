package util

import (
	"context"

	"github.com/google/uuid"
)

type requestIdCtxKeyT int8

const requestIdCtxKey = requestIdCtxKeyT(1)

// NewRequestId генерирует новый request id (uuid v4).
func NewRequestId() string {
	return uuid.NewString()
}

// RequestIdToContext кладёт request id в контекст (gRPC-интерсептор, MCP).
func RequestIdToContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIdCtxKey, id)
}

// RequestIdFromContext извлекает request id из контекста; пустая строка,
// если id не проставлен.
func RequestIdFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIdCtxKey).(string); ok {
		return id
	}
	return ""
}
