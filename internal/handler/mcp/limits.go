package mcp

import (
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// requestMaxBytes — предел тела запроса на MCP-порт. Самые большие вызовы —
	// import_*_batch (до 500 объектов, только имена ключей) и значения
	// config_item (k8s-объект и так не больше 1 МБ).
	requestMaxBytes = 4 << 20

	// requestReadTimeout — за сколько тело запроса должно дочитаться.
	// http.Server.ReadTimeout тут не подходит: он остаётся на соединении и
	// после чтения тела и отменяет контекст долгих потоковых (SSE) ответов.
	requestReadTimeout = time.Minute
)

// withRequestLimits ограничивает размер тела запроса и время его чтения.
// Срок чтения ставится на соединение до любых проверок (медленное тело без
// ключа тоже обрывается) и снимается, как только тело дочитано, — потоковый
// ответ после этого живёт сколько нужно.
func withRequestLimits(next http.Handler, maxBytes int64, readTimeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBytes {
			http.Error(w, "kusec mcp: тело запроса слишком большое", http.StatusRequestEntityTooLarge)
			return
		}

		if r.Body != nil && r.Body != http.NoBody {
			rc := http.NewResponseController(w)
			// ошибка — writer без управления сроками (тесты на ResponseRecorder);
			// размер ограничивается и тогда
			if rc.SetReadDeadline(time.Now().Add(readTimeout)) == nil {
				r.Body = &deadlineBody{ReadCloser: r.Body, rc: rc}
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}

		next.ServeHTTP(w, r)
	})
}

// deadlineBody снимает срок чтения с соединения, когда тело дочитано до EOF.
// При обрыве (истёк срок) срок не снимается: net/http перед ответом пытается
// дочитать остаток тела и без срока ждал бы его вечно.
type deadlineBody struct {
	io.ReadCloser
	rc   *http.ResponseController
	once sync.Once
}

func (b *deadlineBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if errors.Is(err, io.EOF) {
		b.once.Do(func() {
			_ = b.rc.SetReadDeadline(time.Time{})
		})
	}
	return n, err
}
