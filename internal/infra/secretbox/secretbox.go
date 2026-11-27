// Package secretbox шифрует значения отдельных колонок перед записью в базу,
// чтобы резервная копия или дамп базы не раскрывали секреты.
//
// Алгоритм — AES-256-GCM. Формат хранения:
//
//	enc:v1:<base64(nonce ‖ ciphertext ‖ tag)>
//
// nonce — 12 случайных байт на каждое значение. В дополнительные данные GCM
// (AAD) уходит scope — строка-контекст значения, обычно «таблица.колонка»
// ("item.value"): шифртекст, перенесённый в другую колонку, не расшифруется.
//
// Режимы:
//   - ключ задан — значения шифруются при записи и расшифровываются при
//     чтении; строки без префикса (ещё не переведённые) читаются как есть;
//   - ключ пуст — шифрование выключено, значения хранятся открытым текстом;
//     встретив зашифрованную строку, Open отдаёт ErrKeyRequired, а не шифртекст.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Prefix — признак зашифрованного значения. v1 — версия формата: смена
// алгоритма или ключа получит свой префикс, старые строки останутся читаемыми.
const Prefix = "enc:v1:"

// KeySize — длина ключа AES-256 в байтах (до кодирования в base64).
const KeySize = 32

var (
	// ErrInvalidKey — ключ задан, но это не base64 (std) от 32 байт.
	ErrInvalidKey = errors.New("secretbox: DATA_ENCRYPTION_KEY must be base64 (std) of 32 random bytes")
	// ErrKeyRequired — в хранилище зашифрованное значение, а ключ не задан.
	ErrKeyRequired = errors.New("secretbox: stored values are encrypted but DATA_ENCRYPTION_KEY is not set")
	// ErrDecrypt — значение не расшифровалось: другой ключ, другой scope или
	// данные повреждены. Текст ошибки не содержит ни значения, ни ключа.
	ErrDecrypt = errors.New("secretbox: cannot decrypt value (wrong key, wrong scope or corrupted data)")
	// ErrReservedPrefix — при выключенном шифровании пытаются сохранить
	// открытый текст, начинающийся с Prefix: такую строку потом не отличить
	// от зашифрованной.
	ErrReservedPrefix = errors.New(`secretbox: value must not start with reserved prefix "` + Prefix + `"`)
	// ErrScopeRequired — пустой scope (ошибка вызывающего кода).
	ErrScopeRequired = errors.New("secretbox: scope is required")
)

// Box шифрует и расшифровывает значения одним ключом. Нулевой указатель и
// Box без ключа ведут себя одинаково — шифрование выключено.
type Box struct {
	aead cipher.AEAD
}

// New собирает Box из ключа: base64 (std) от 32 случайных байт. Пустой ключ —
// шифрование выключено (ошибки нет); неверный формат или длина — ErrInvalidKey.
func New(key string) (*Box, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return &Box{}, nil
	}

	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		// текст ошибки base64 не пробрасываем: в нём позиция и байт ключа
		return nil, fmt.Errorf("%w: not a valid base64", ErrInvalidKey)
	}
	if len(raw) != KeySize {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKey, len(raw))
	}

	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}

	return &Box{aead: aead}, nil
}

// Enabled сообщает, включено ли шифрование (задан ли ключ).
func (b *Box) Enabled() bool {
	return b != nil && b.aead != nil
}

// Seal готовит значение к записи в базу.
//
// Шифрование включено: непустое значение шифруется (каждый раз с новым nonce,
// поэтому одинаковые значения дают разные строки); пустое остаётся пустым —
// проверки на пустоту в SQL продолжают работать.
//
// Шифрование выключено: значение возвращается как есть. Исключение — значение,
// начинающееся с Prefix: его отклоняем (ErrReservedPrefix), иначе оно выглядело
// бы зашифрованным и сервис не поднялся бы при следующем запуске.
func (b *Box) Seal(scope, plain string) (string, error) {
	if scope == "" {
		return "", ErrScopeRequired
	}
	if !b.Enabled() {
		if IsSealed(plain) {
			return "", ErrReservedPrefix
		}
		return plain, nil
	}
	if plain == "" {
		return "", nil
	}

	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	// nonce — первым в буфере: получается nonce ‖ ciphertext ‖ tag
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), []byte(scope))

	return Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Open возвращает открытое значение по строке из базы.
//
// Строка без Prefix возвращается как есть: это пустое значение либо старая,
// ещё не зашифрованная строка. Строка с Prefix расшифровывается; scope должен
// совпадать с тем, что был при Seal. Без ключа — ErrKeyRequired.
func (b *Box) Open(scope, stored string) (string, error) {
	if scope == "" {
		return "", ErrScopeRequired
	}
	if !IsSealed(stored) {
		return stored, nil
	}
	if !b.Enabled() {
		return "", ErrKeyRequired
	}

	raw, err := base64.StdEncoding.DecodeString(stored[len(Prefix):])
	if err != nil {
		return "", ErrDecrypt
	}
	nonceSize := b.aead.NonceSize()
	if len(raw) < nonceSize+b.aead.Overhead() {
		return "", ErrDecrypt
	}

	plain, err := b.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], []byte(scope))
	if err != nil {
		return "", ErrDecrypt
	}

	return string(plain), nil
}

// IsSealed сообщает, зашифрована ли строка из базы (начинается ли с Prefix).
// Ключ для проверки не нужен.
func IsSealed(stored string) bool {
	return strings.HasPrefix(stored, Prefix)
}
