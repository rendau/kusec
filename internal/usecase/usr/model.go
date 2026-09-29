package usr

type UpdateProfileReq struct {
	Name     *string
	Username *string
	Password *string
}

// LoginResult — итог попытки логина. Возможны два исхода:
//   - выдана пара токенов (Jwt+RefreshToken);
//   - пароль верный, но включена 2FA и не передан код (TotpRequired).
type LoginResult struct {
	Jwt          string
	RefreshToken string

	TotpRequired bool
}
