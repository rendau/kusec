package usr

// LoginReq — попытка входа. ClientIp — адрес клиента, как его видит
// транспорт; нужен только для журнала входов.
type LoginReq struct {
	Username string
	Password string
	TotpCode string

	ClientIp string
}

type UpdateProfileReq struct {
	Name     *string
	Username *string
	Password *string

	// CurrentPassword — текущий пароль: обязателен, если меняется пароль
	// или логин.
	CurrentPassword *string
}

// LoginResult — итог попытки логина. Возможны два исхода:
//   - выдана пара токенов (Jwt+RefreshToken);
//   - пароль верный, но включена 2FA и не передан код (TotpRequired).
type LoginResult struct {
	Jwt          string
	RefreshToken string

	TotpRequired bool
}
