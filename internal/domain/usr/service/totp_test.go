package service

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTotpSecret(t *testing.T) {
	secret, url, err := generateTotpSecret("alice")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, url, "otpauth://totp/")
	assert.Contains(t, url, "kusec")
}

func TestValidateTotpCode(t *testing.T) {
	svc := &Service{}

	secret, _, err := generateTotpSecret("alice")
	require.NoError(t, err)

	code, err := totp.GenerateCode(secret, time.Now().UTC())
	require.NoError(t, err)

	assert.True(t, svc.ValidateTotpCode(secret, code), "valid code must pass")
	assert.False(t, svc.ValidateTotpCode(secret, "000000"), "wrong code must fail")
	assert.False(t, svc.ValidateTotpCode(secret, ""), "empty code must fail")
	assert.False(t, svc.ValidateTotpCode("", code), "empty secret must fail")
}
