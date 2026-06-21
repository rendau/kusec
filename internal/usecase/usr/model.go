package usr

type UpdateProfileReq struct {
	Name     *string
	Username *string
	Password *string
}

// LoginResult — итог попытки логина. Возможны три исхода:
//   - выдана пара токенов (Jwt+RefreshToken);
//   - пароль верный, но включена 2FA и не передан код (TotpRequired);
//   - админ без настроенной 2FA — обязан её привязать (TotpSetupRequired +
//     краткоживущий SetupToken для эндпоинтов настройки).
type LoginResult struct {
	Jwt          string
	RefreshToken string

	TotpRequired      bool
	TotpSetupRequired bool
	SetupToken        string
}
