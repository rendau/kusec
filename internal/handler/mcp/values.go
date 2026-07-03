package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// ── Генерация значений ──────────────────────────────────

const (
	defaultGenLength = 32
	maxGenLength     = 4096

	alnumChars  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	digitChars  = "0123456789"
	asciiExtras = "!#$%&()*+,-./:;<=>?@[]^_{|}~"
)

// generateValue генерирует случайное значение указанного формата через
// crypto/rand. length — длина результата в символах (для uuid игнорируется).
func generateValue(format string, length int) (string, error) {
	if length <= 0 {
		length = defaultGenLength
	}
	if length > maxGenLength {
		return "", fmt.Errorf("length %d превышает максимум %d", length, maxGenLength)
	}

	switch format {
	case "", "alnum":
		return randString(alnumChars, length)
	case "ascii":
		return randString(alnumChars+asciiExtras, length)
	case "digits":
		return randString(digitChars, length)
	case "hex":
		raw := make([]byte, (length+1)/2)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("rand read: %w", err)
		}
		return hex.EncodeToString(raw)[:length], nil
	case "base64url":
		raw := make([]byte, (length*3+3)/4+1)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("rand read: %w", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)[:length], nil
	case "uuid":
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("rand read: %w", err)
		}
		raw[6] = (raw[6] & 0x0f) | 0x40 // версия 4
		raw[8] = (raw[8] & 0x3f) | 0x80 // вариант RFC 4122
		return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
	default:
		return "", fmt.Errorf("неизвестный формат генерации %q (доступны: alnum, ascii, digits, hex, base64url, uuid)", format)
	}
}

// randString — равномерная выборка из charset без modulo-bias (rejection sampling).
func randString(charset string, length int) (string, error) {
	limit := 256 - 256%len(charset)

	result := make([]byte, 0, length)
	buf := make([]byte, length*2)

	for len(result) < length {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("rand read: %w", err)
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			result = append(result, charset[int(b)%len(charset)])
			if len(result) == length {
				break
			}
		}
	}

	return string(result), nil
}

// ── Маскирование ────────────────────────────────────────

const valueHashLen = 12

type maskedValue struct {
	Chars  int
	Bytes  int
	Sha256 string
}

// maskValue сводит значение к безопасным метаданным: длина в символах и
// байтах + усечённый sha256 (для сравнения значений между собой).
func maskValue(value string) maskedValue {
	sum := sha256.Sum256([]byte(value))

	return maskedValue{
		Chars:  utf8.RuneCountInString(value),
		Bytes:  len(value),
		Sha256: hex.EncodeToString(sum[:])[:valueHashLen],
	}
}

// ── Реестр значений и скраб ─────────────────────────────

// vault хранит значения секретов, прошедшие через MCP-сервер, только в памяти
// процесса: именованные — для переиспользования (reuse), все увиденные — для
// вычищения из текстов ошибок (scrub).
type vault struct {
	mu    sync.Mutex
	named map[string]string
	seen  map[string]struct{}
}

func newVault() *vault {
	return &vault{
		named: map[string]string{},
		seen:  map[string]struct{}{},
	}
}

func (v *vault) remember(name, value string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.named[name] = value
	v.seen[value] = struct{}{}
}

func (v *vault) lookup(name string) (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.named[name]
	return value, ok
}

// names возвращает имена зарегистрированных значений (сами значения не раскрывает).
func (v *vault) names() []string {
	v.mu.Lock()
	defer v.mu.Unlock()

	result := make([]string, 0, len(v.named))
	for name := range v.named {
		result = append(result, name)
	}
	sort.Strings(result)

	return result
}

func (v *vault) markSeen(value string) {
	if value == "" {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.seen[value] = struct{}{}
}

// scrub вычищает из текста все известные значения секретов.
func (v *vault) scrub(s string) string {
	v.mu.Lock()
	defer v.mu.Unlock()

	for value := range v.seen {
		if strings.Contains(s, value) {
			s = strings.ReplaceAll(s, value, "[REDACTED]")
		}
	}

	return s
}
