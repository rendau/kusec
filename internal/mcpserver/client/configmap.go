package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

func (c *Client) ConfigMapList(ctx context.Context, req model.ConfigMapListReq) (model.ConfigMapListRep, error) {
	q := listQuery(req.ListParams)
	setStr(q, "app_id", req.AppID)
	setBool(q, "active", req.Active)
	setStr(q, "search", req.Search)

	rep := model.ConfigMapListRep{}
	if err := c.sendRequest(ctx, http.MethodGet, "/configmap", q, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("configmap list: %w", err)
	}

	return rep, nil
}

func (c *Client) ConfigMapGet(ctx context.Context, id string) (model.ConfigMap, error) {
	rep := model.ConfigMap{}
	if err := c.sendRequest(ctx, http.MethodGet, "/configmap/"+url.PathEscape(id), nil, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("configmap get: %w", err)
	}

	return rep, nil
}

func (c *Client) ConfigMapCreate(ctx context.Context, req model.ConfigMapCreateReq) (string, error) {
	rep := model.ConfigMapCreateRep{}
	if err := c.sendRequest(ctx, http.MethodPost, "/configmap", nil, req, &rep, true); err != nil {
		return "", fmt.Errorf("configmap create: %w", err)
	}

	return rep.ID, nil
}

func (c *Client) ConfigMapUpdate(ctx context.Context, id string, req model.ConfigMapUpdateReq) error {
	if err := c.sendRequest(ctx, http.MethodPut, "/configmap/"+url.PathEscape(id), nil, req, nil, true); err != nil {
		return fmt.Errorf("configmap update: %w", err)
	}

	return nil
}
