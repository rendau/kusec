package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

func (c *Client) AppList(ctx context.Context, req model.AppListReq) (model.AppListRep, error) {
	q := listQuery(req.ListParams)
	setBool(q, "active", req.Active)
	setStr(q, "namespace", req.Namespace)
	setStr(q, "search", req.Search)

	rep := model.AppListRep{}
	if err := c.sendRequest(ctx, http.MethodGet, "/app", q, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("app list: %w", err)
	}

	return rep, nil
}

func (c *Client) AppGet(ctx context.Context, id string) (model.App, error) {
	rep := model.App{}
	if err := c.sendRequest(ctx, http.MethodGet, "/app/"+url.PathEscape(id), nil, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("app get: %w", err)
	}

	return rep, nil
}

func (c *Client) AppCreate(ctx context.Context, req model.AppCreateReq) (string, error) {
	rep := model.AppCreateRep{}
	if err := c.sendRequest(ctx, http.MethodPost, "/app", nil, req, &rep, true); err != nil {
		return "", fmt.Errorf("app create: %w", err)
	}

	return rep.ID, nil
}

func (c *Client) AppUpdate(ctx context.Context, id string, req model.AppUpdateReq) error {
	if err := c.sendRequest(ctx, http.MethodPut, "/app/"+url.PathEscape(id), nil, req, nil, true); err != nil {
		return fmt.Errorf("app update: %w", err)
	}

	return nil
}
