package constant

const (
	ServiceName = "kusec"

	MaxPageSize = 1000

	// ApiKeyPrefix — префикс API-ключей (kusec service key); по нему
	// session-интерсептор отличает ключ от JWT.
	ApiKeyPrefix = "ksk_"
)
