package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

func (c *Client) SecretList(ctx context.Context, req model.SecretListReq) (model.SecretListRep, error) {
	q := listQuery(req.ListParams)
	setStr(q, "app_id", req.AppID)
	setBool(q, "active", req.Active)
	setStr(q, "search", req.Search)

	rep := model.SecretListRep{}
	if err := c.sendRequest(ctx, http.MethodGet, "/secret", q, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("secret list: %w", err)
	}

	return rep, nil
}

func (c *Client) SecretGet(ctx context.Context, id string) (model.Secret, error) {
	rep := model.Secret{}
	if err := c.sendRequest(ctx, http.MethodGet, "/secret/"+url.PathEscape(id), nil, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("secret get: %w", err)
	}

	return rep, nil
}

func (c *Client) SecretCreate(ctx context.Context, req model.SecretCreateReq) (string, error) {
	rep := model.SecretCreateRep{}
	if err := c.sendRequest(ctx, http.MethodPost, "/secret", nil, req, &rep, true); err != nil {
		return "", fmt.Errorf("secret create: %w", err)
	}

	return rep.ID, nil
}

func (c *Client) SecretUpdate(ctx context.Context, id string, req model.SecretUpdateReq) error {
	if err := c.sendRequest(ctx, http.MethodPut, "/secret/"+url.PathEscape(id), nil, req, nil, true); err != nil {
		return fmt.Errorf("secret update: %w", err)
	}

	return nil
}
