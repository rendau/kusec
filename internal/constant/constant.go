package constant

const (
	ServiceName = "kusec"

	MaxPageSize = 1000

	// ApiKeyPrefix — префикс API-ключей (kusec service key); по нему
	// session-интерсептор отличает ключ от JWT.
	ApiKeyPrefix = "ksk_"
)

// Скоупы API-ключей (api_key.scope).
const (
	// ApiKeyScopeFull — полный доступ (наследует права владельца).
	ApiKeyScopeFull = "full"
	// ApiKeyScopeReadOnly — только методы чтения из белого списка,
	// значения item/config_item в ответах маскируются.
	ApiKeyScopeReadOnly = "read_only"
	// ApiKeyScopeMcpOnly — ключ принимается только встроенным MCP-эндпоинтом.
	ApiKeyScopeMcpOnly = "mcp_only"
)

// Источники запросов (session.Source, audit.source).
const (
	SourceUi     = "ui"
	SourceApi    = "api"
	SourceMcp    = "mcp"
	SourceSystem = "system"
)
