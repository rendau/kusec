package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

func (c *Client) ConfigItemList(ctx context.Context, req model.ConfigItemListReq) (model.ConfigItemListRep, error) {
	q := listQuery(req.ListParams)
	setStr(q, "configmap_id", req.ConfigmapID)
	setStrSlice(q, "configmap_ids", req.ConfigmapIDs)
	setBool(q, "active", req.Active)
	setStr(q, "search", req.Search)

	rep := model.ConfigItemListRep{}
	if err := c.sendRequest(ctx, http.MethodGet, "/config-item", q, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("config-item list: %w", err)
	}

	return rep, nil
}

func (c *Client) ConfigItemGet(ctx context.Context, id string) (model.ConfigItem, error) {
	rep := model.ConfigItem{}
	if err := c.sendRequest(ctx, http.MethodGet, "/config-item/"+url.PathEscape(id), nil, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("config-item get: %w", err)
	}

	return rep, nil
}

func (c *Client) ConfigItemCreate(ctx context.Context, req model.ConfigItemCreateReq) (string, error) {
	rep := model.ConfigItemCreateRep{}
	if err := c.sendRequest(ctx, http.MethodPost, "/config-item", nil, req, &rep, true); err != nil {
		return "", fmt.Errorf("config-item create: %w", err)
	}

	return rep.ID, nil
}

func (c *Client) ConfigItemUpdate(ctx context.Context, id string, req model.ConfigItemUpdateReq) error {
	if err := c.sendRequest(ctx, http.MethodPut, "/config-item/"+url.PathEscape(id), nil, req, nil, true); err != nil {
		return fmt.Errorf("config-item update: %w", err)
	}

	return nil
}
