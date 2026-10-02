package errs

type Err string

func (e Err) Error() string {
	return string(e)
}

// common errors
const (
	ServiceNA         = Err("service_not_available")
	NotImplemented    = Err("not_implemented")
	InvalidConfig     = Err("invalid_config")
	NoPermission      = Err("no_permission")
	ObjectNotFound    = Err("object_not_found")
	NoRows            = Err("err_no_rows")
	NotAuthorized     = Err("not_authorized")
	InvalidRequest    = Err("invalid_request")
	IncorrectPageSize = Err("incorrect_page_size")
	IdRequired        = Err("id_required")

	NameRequired     = Err("name_required")
	UsernameRequired = Err("username_required")
	PasswordRequired = Err("password_required")

	PasswordTooShort        = Err("password_too_short")
	PasswordTooLong         = Err("password_too_long")
	PasswordRequiresUpper   = Err("password_requires_upper")
	PasswordRequiresSpecial = Err("password_requires_special")

	// нарушения уникальности (уникальные индексы БД)
	UsernameExists      = Err("username_exists")
	AppSlugExists       = Err("app_slug_exists")
	SecretSlugExists    = Err("secret_slug_exists")
	ItemKeyExists       = Err("item_key_exists")
	ConfigMapSlugExists = Err("configmap_slug_exists")
	ConfigItemKeyExists = Err("config_item_key_exists")

	TotpInvalid    = Err("totp_invalid")
	TotpAlreadyOn  = Err("totp_already_enabled")
	TotpNotEnabled = Err("totp_not_enabled")

	NotInCluster   = Err("not_in_cluster")
	SyncInProgress = Err("sync_in_progress")
)

type ErrFull struct {
	Err    error
	Desc   string
	Fields map[string]string
}

func (e ErrFull) Error() string {
	return e.Err.Error() + ", desc: " + e.Desc
}
