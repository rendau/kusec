package service

import (
	"errors"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = bcrypt.DefaultCost

// dummyPasswordHash — заранее посчитанный хэш для «холостого» сравнения,
// когда пользователя с таким логином нет: bcrypt выполняется в обоих
// случаях, и по времени ответа не отличить существующий логин от
// несуществующего. Считается один раз на процесс (см. New).
var dummyPasswordHash = sync.OnceValue(func() string {
	hash, err := bcrypt.GenerateFromPassword([]byte("kusec-no-such-user"), bcryptCost)
	if err != nil {
		return ""
	}
	return string(hash)
})

func hashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func comparePassword(hash, password string) (bool, error) {
	password = strings.TrimSpace(password)

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true, nil
	}

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if errors.Is(err, bcrypt.ErrHashTooShort) {
		return false, nil
	}

	return false, err
}
