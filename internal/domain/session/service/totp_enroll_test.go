package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnrollTokenRoundTrip(t *testing.T) {
	svc := New("test-secret")

	token, err := svc.CreateEnrollToken(42)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	usrId, err := svc.ParseEnrollToken(token)
	require.NoError(t, err)
	assert.Equal(t, int64(42), usrId)
}

func TestEnrollTokenNotAcceptedAsAccess(t *testing.T) {
	svc := New("test-secret")

	token, err := svc.CreateEnrollToken(42)
	require.NoError(t, err)

	// enroll-токен не должен проходить как access-токен (нет доступа к обычным ручкам)
	_, err = svc.FromToken(token)
	assert.Error(t, err)
}

func TestAccessTokenNotAcceptedAsEnroll(t *testing.T) {
	svc := New("test-secret")

	token, err := svc.CreateToken(42, false, nil)
	require.NoError(t, err)

	_, err = svc.ParseEnrollToken(token)
	assert.Error(t, err)
}
