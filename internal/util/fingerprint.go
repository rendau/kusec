package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// fingerprintLen — длина отпечатка в hex-символах.
const fingerprintLen = 16

// Fingerprint — HMAC-SHA256 отпечаток данных с серверным ключом
// (config AUDIT_HASH_KEY), усечённый до 16 hex-символов. Голый sha256 не
// годится: короткие значения перебираются по словарю. Отпечаток позволяет
// видеть «значение поменялось / вернулось к прежнему», не раскрывая его.
func Fingerprint(hashKey string, data []byte) string {
	mac := hmac.New(sha256.New, []byte(hashKey))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))[:fingerprintLen]
}

// ValueFingerprint — отпечаток строкового значения.
func ValueFingerprint(hashKey, value string) string {
	return Fingerprint(hashKey, []byte(value))
}
