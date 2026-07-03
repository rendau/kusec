// Package client — HTTP-клиент к API kusec (через grpc-gateway) для MCP-сервера.
//
// Значения секретов проходят через этот пакет транзитом (Item.Value); наружу,
// в ответы MCP-инструментов, их выпускает только value-механика mcpserver.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mechta-market/kusec/internal/errs"
	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

const (
	defaultPageSize = 100

	// tokenExpirySkew — за сколько до истечения access-токена считаем его протухшим.
	tokenExpirySkew = 30 * time.Second

	// maxResponseBytes — предохранитель на размер тела ответа.
	maxResponseBytes = 32 << 20
)

// APIError — семантическая ошибка API kusec (common.ErrorRep + HTTP-статус).
// Тело неразобранных ошибок наружу не выносится.
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	Fields     map[string]string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("kusec api [%d/%s]", e.HTTPStatus, e.Code)
	if e.Message != "" && e.Message != e.Code {
		msg += ": " + e.Message
	}
	for k, v := range e.Fields {
		msg += fmt.Sprintf("; %s: %s", k, v)
	}
	return msg
}

type Client struct {
	apiURL     string
	apiKey     string
	username   string
	password   string
	httpClient *http.Client

	mu           sync.Mutex
	accessToken  string
	accessExp    time.Time
	refreshToken string
}

func New(apiURL, apiKey, username, password, refreshToken string, insecureSkipVerify bool) *Client {
	return &Client{
		apiURL:       strings.TrimRight(apiURL, "/"),
		apiKey:       apiKey,
		username:     username,
		password:     password,
		refreshToken: refreshToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext:         (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
				TLSHandshakeTimeout: 3 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: insecureSkipVerify},
				MaxIdleConnsPerHost: 5,
			},
		},
	}
}

// ── Транспорт ───────────────────────────────────────────

// sendRequest — единственная точка отправки запросов. При протухшем
// access-токене (not_authorized) один раз переавторизуется и повторяет запрос.
func (c *Client) sendRequest(ctx context.Context, method, path string, query url.Values, reqObj, repObj any, withAuth bool) error {
	err := c.doRequest(ctx, method, path, query, reqObj, repObj, withAuth)
	if err != nil && withAuth && c.apiKey == "" {
		if apiErr, ok := errors.AsType[*APIError](err); ok && apiErr.Code == errs.NotAuthorized.Error() {
			c.invalidateToken()
			err = c.doRequest(ctx, method, path, query, reqObj, repObj, withAuth)
		}
	}
	return err
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, reqObj, repObj any, withAuth bool) error {
	uri := c.apiURL + path
	if len(query) > 0 {
		uri += "?" + query.Encode()
	}

	var reqBody io.Reader
	if reqObj != nil {
		raw, err := json.Marshal(reqObj)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, reqBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if withAuth {
		token, err := c.token(ctx)
		if err != nil {
			return fmt.Errorf("auth: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseError(resp.StatusCode, respBody)
	}

	if repObj != nil {
		if err = json.Unmarshal(respBody, repObj); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// parseError разбирает тело ошибки gateway. Неразобранное тело в ошибку не
// попадает — только HTTP-статус.
func parseError(status int, body []byte) *APIError {
	errRep := model.ErrorRep{}
	if err := json.Unmarshal(body, &errRep); err != nil || errRep.Code == "" {
		return &APIError{
			HTTPStatus: status,
			Code:       "http_" + strconv.Itoa(status),
			Message:    http.StatusText(status),
		}
	}

	return &APIError{
		HTTPStatus: status,
		Code:       errRep.Code,
		Message:    errRep.Message,
		Fields:     errRep.Fields,
	}
}

// ── Авторизация ─────────────────────────────────────────

func (c *Client) token(ctx context.Context) (string, error) {
	// API-ключ статичен: без обновлений и локального состояния
	if c.apiKey != "" {
		return c.apiKey, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.accessExp.Add(-tokenExpirySkew)) {
		return c.accessToken, nil
	}

	return c.authenticate(ctx)
}

// authenticate вызывается под c.mu. Сначала пробует refresh-токен, затем логин.
func (c *Client) authenticate(ctx context.Context) (string, error) {
	var refreshErr error

	if c.refreshToken != "" {
		rep := model.LoginRep{}
		refreshErr = c.doRequest(ctx, http.MethodPost, "/usr/token/refresh", nil,
			model.RefreshTokenReq{RefreshToken: c.refreshToken}, &rep, false)
		if refreshErr == nil && rep.Jwt != "" {
			c.storeTokens(rep)
			return c.accessToken, nil
		}
	}

	if c.username != "" {
		rep := model.LoginRep{}
		err := c.doRequest(ctx, http.MethodPost, "/usr/login", nil,
			model.LoginReq{Username: c.username, Password: c.password}, &rep, false)
		if err != nil {
			return "", fmt.Errorf("login: %w", err)
		}
		if rep.TotpRequired || rep.TotpSetupRequired {
			return "", errors.New("login: аккаунту требуется 2FA — задайте KUSEC_MCP_REFRESH_TOKEN или используйте сервисный аккаунт без 2FA")
		}
		if rep.Jwt == "" {
			return "", errors.New("login: пустой jwt в ответе")
		}
		c.storeTokens(rep)
		return c.accessToken, nil
	}

	if refreshErr != nil {
		return "", fmt.Errorf("refresh token: %w", refreshErr)
	}

	return "", errors.New("нет учётных данных: задайте KUSEC_MCP_REFRESH_TOKEN либо KUSEC_MCP_USERNAME/KUSEC_MCP_PASSWORD")
}

// storeTokens вызывается под c.mu.
func (c *Client) storeTokens(rep model.LoginRep) {
	c.accessToken = rep.Jwt
	c.accessExp = jwtExpiry(rep.Jwt)
	if rep.RefreshToken != "" {
		c.refreshToken = rep.RefreshToken
	}
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
}

// jwtExpiry достаёт exp из claims без проверки подписи (нужен только момент
// протухания). Если распарсить не удалось — короткий локальный TTL.
func jwtExpiry(token string) time.Time {
	fallback := time.Now().Add(5 * time.Minute)

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fallback
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fallback
	}

	claims := struct {
		Exp int64 `json:"exp"`
	}{}
	if err = json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return fallback
	}

	return time.Unix(claims.Exp, 0)
}

// ── Query-хелперы ───────────────────────────────────────

func listQuery(p model.ListParams) url.Values {
	if p.PageSize <= 0 {
		p.PageSize = defaultPageSize
	}

	q := url.Values{}
	q.Set("list_params.page", strconv.FormatInt(p.Page, 10))
	q.Set("list_params.page_size", strconv.FormatInt(p.PageSize, 10))
	if p.WithTotalCount {
		q.Set("list_params.with_total_count", "true")
	}

	return q
}

func setStr(q url.Values, key string, v *string) {
	if v != nil {
		q.Set(key, *v)
	}
}

func setBool(q url.Values, key string, v *bool) {
	if v != nil {
		q.Set(key, strconv.FormatBool(*v))
	}
}

func setStrSlice(q url.Values, key string, vv []string) {
	for _, v := range vv {
		q.Add(key, v)
	}
}
