package mcp

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLimits_BodySize(t *testing.T) {
	t.Parallel()

	var got []byte
	srv := httptest.NewServer(withRequestLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		got = body
		w.WriteHeader(http.StatusOK)
	}), 16, time.Second))
	defer srv.Close()

	// в пределах — доходит целиком
	rep, err := http.Post(srv.URL, "application/json", strings.NewReader("0123456789"))
	require.NoError(t, err)
	_ = rep.Body.Close()
	assert.Equal(t, http.StatusOK, rep.StatusCode)
	assert.Equal(t, "0123456789", string(got))

	// заявленная длина больше предела — 413 до чтения
	rep, err = http.Post(srv.URL, "application/json", strings.NewReader(strings.Repeat("x", 17)))
	require.NoError(t, err)
	_ = rep.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, rep.StatusCode)

	// длина не заявлена (chunked) — обрывается на пределе при чтении
	rep, err = http.Post(srv.URL, "application/json", io.MultiReader(strings.NewReader(strings.Repeat("x", 40))))
	require.NoError(t, err)
	_ = rep.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, rep.StatusCode)
}

func TestRequestLimits_SlowBodyCut(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(withRequestLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusOK)
	}), 1024, 200*time.Millisecond))
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	require.NoError(t, err)
	defer conn.Close()

	// заявлено 100 байт, прислано 10 — и тишина
	_, err = conn.Write([]byte("POST / HTTP/1.1\r\nHost: x\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n0123456789"))
	require.NoError(t, err)

	started := time.Now()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	rep, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err, "сервер должен ответить, а не ждать тело вечно")
	_ = rep.Body.Close()
	assert.Equal(t, http.StatusRequestTimeout, rep.StatusCode)
	assert.Less(t, time.Since(started), 2*time.Second)
}

// После дочитанного тела срок снят: долгий (потоковый) ответ не обрывается и
// контекст запроса не отменяется.
func TestRequestLimits_LongResponseAfterBody(t *testing.T) {
	t.Parallel()

	const readTimeout = 150 * time.Millisecond

	srv := httptest.NewServer(withRequestLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		for range 4 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(readTimeout):
			}
			_, _ = w.Write([]byte("tick\n"))
			http.NewResponseController(w).Flush()
		}
	}), 1024, readTimeout))
	defer srv.Close()

	rep, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"a":1}`)))
	require.NoError(t, err)
	defer rep.Body.Close()

	body, err := io.ReadAll(rep.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rep.StatusCode)
	assert.Equal(t, strings.Repeat("tick\n", 4), string(body))
}
