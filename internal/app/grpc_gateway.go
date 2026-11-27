package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"sync/atomic"

	"github.com/rendau/kusec/internal/errs"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/rendau/kusec/internal/config"
)

// httpMaxBodyBytes — предел тела запроса на основном порту (раньше предела не
// было: любой без входа мог слать гигантские тела на /api/usr/login и занять
// всю память пода). Откуда число: самое большое осмысленное тело — значение
// item-а, а объект Kubernetes (Secret, ConfigMap) целиком ограничен 1 МиБ;
// остальное — запас на пакетные вставки и импорт.
const httpMaxBodyBytes = 8 << 20

func GrpcGatewayCreateHandler(muxHook func(*runtime.ServeMux) error) (http.Handler, error) {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		runtime.WithErrorHandler(func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
			// тело оборвано на пределе при чтении (длина не была заявлена):
			// шлюз видит это как ошибку разбора JSON — отвечаем тем же 413
			if bodyLimitExceeded(r) {
				writeBodyTooLarge(w, httpMaxBodyBytes)
				return
			}

			var repBody []byte

			if st, ok := status.FromError(err); ok {
				if st.Code() == codes.NotFound {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`service path not found`))
					return
				} else if st.Code() == codes.InvalidArgument && len(st.Details()) > 0 {
					var marshalErr error
					repBody, marshalErr = marshaler.Marshal(st.Details()[0])
					if marshalErr != nil {
						slog.Error("GRPC_GW: ErrorHandler: Failed to marshal", "error", marshalErr)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}
			}

			if len(repBody) == 0 {
				// runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
				obj := map[string]string{
					"code":    errs.ServiceNA.Error(),
					"message": err.Error(),
				}
				repBody, err = json.Marshal(obj)
				if err != nil {
					slog.Error("GRPC_GW: ErrorHandler: Failed to marshal", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			// slog.Error("GRPC_GW: ErrorHandler", "error", err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err = io.Copy(w, bytes.NewReader(repBody))
			if err != nil {
				slog.Error("GRPC_GW: ErrorHandler: Failed to write response", "error", err)
			}
			// _, _ = w.Write(repBody)
		}),
	)

	if muxHook != nil {
		err := muxHook(mux)
		if err != nil {
			return nil, fmt.Errorf("grpc-gateway: muxHook: %w", err)
		}
	}

	// ограничение тела запроса — под cors, чтобы отказ 413 тоже шёл с
	// CORS-заголовками и браузер мог показать его текст
	handler := withBodyLimit(mux, httpMaxBodyBytes)

	// add cors middleware
	if config.Conf.HttpCors {
		corsOptions := cors.Options{
			AllowedMethods: []string{
				http.MethodGet,
				http.MethodPut,
				http.MethodPost,
				http.MethodDelete,
			},
			AllowedHeaders: []string{
				"Accept",
				"Content-Type",
				"X-Requested-With",
				"Authorization",
			},
			AllowCredentials: true,
			MaxAge:           604800,
		}

		origins := config.Conf.HttpCorsAllowedOrigins
		if len(origins) == 0 || slices.Contains(origins, "*") {
			// Белый список не задан — разрешаем любой Origin (старое
			// поведение). Небезопасно при AllowCredentials: задайте
			// HTTP_CORS_ALLOWED_ORIGINS, чтобы ограничить источники.
			corsOptions.AllowOriginFunc = func(string) bool { return true }
			slog.Warn("CORS: all origins allowed; set HTTP_CORS_ALLOWED_ORIGINS to restrict")
		} else {
			corsOptions.AllowedOrigins = origins
		}

		handler = cors.New(corsOptions).Handler(handler)
	}

	// add recover middleware
	handler = func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				// use always new err instance in deferring
				if err := recover(); err != nil {
					slog.Error(
						"Recovered from panic",
						slog.Any("error", err),
						slog.Any("recovery_stacktrace", string(debug.Stack())),
					)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			h.ServeHTTP(w, r)
		})
	}(handler)

	return handler, nil
}

// ── Ограничение тела запроса ────────────────────────────

type bodyLimitCtxKeyT int8

const bodyLimitCtxKey = bodyLimitCtxKeyT(1)

// bodyLimitState — отметка «тело запроса оборвано на пределе». Её ставит
// limitedBody и читает обработчик ошибок шлюза.
type bodyLimitState struct {
	exceeded atomic.Bool
}

// withBodyLimit не даёт прочитать из тела запроса больше maxBytes. Заявленная
// длина больше предела — отказ 413 сразу, до чтения; длина не заявлена
// (chunked) — чтение обрывается на пределе, и 413 отдаёт обработчик ошибок
// шлюза. В обоих случаях net/http закрывает соединение, не дочитывая остаток.
func withBodyLimit(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBytes {
			writeBodyTooLarge(w, maxBytes)
			return
		}

		if r.Body != nil && r.Body != http.NoBody {
			state := &bodyLimitState{}
			r.Body = &limitedBody{
				ReadCloser: http.MaxBytesReader(w, r.Body, maxBytes),
				state:      state,
			}
			r = r.WithContext(context.WithValue(r.Context(), bodyLimitCtxKey, state))
		}

		next.ServeHTTP(w, r)
	})
}

// limitedBody запоминает, что чтение тела упёрлось в предел.
type limitedBody struct {
	io.ReadCloser
	state *bodyLimitState
}

func (b *limitedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		b.state.exceeded.Store(true)
	}
	return n, err
}

func bodyLimitExceeded(r *http.Request) bool {
	state, _ := r.Context().Value(bodyLimitCtxKey).(*bodyLimitState)
	return state != nil && state.exceeded.Load()
}

// writeBodyTooLarge — отказ 413 в формате ошибок шлюза ({code, message}).
func writeBodyTooLarge(w http.ResponseWriter, maxBytes int64) {
	repBody, err := json.Marshal(map[string]string{
		"code":    errs.InvalidRequest.Error(),
		"message": fmt.Sprintf("request body is too large: the limit is %d MiB", maxBytes>>20),
	})
	if err != nil {
		slog.Error("GRPC_GW: body limit: Failed to marshal", "error", err)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	if _, err = w.Write(repBody); err != nil {
		slog.Error("GRPC_GW: body limit: Failed to write response", "error", err)
	}
}
