package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

func (c *Client) ItemList(ctx context.Context, req model.ItemListReq) (model.ItemListRep, error) {
	q := listQuery(req.ListParams)
	setStr(q, "secret_id", req.SecretID)
	setStrSlice(q, "secret_ids", req.SecretIDs)
	setBool(q, "active", req.Active)
	setStr(q, "search", req.Search)

	rep := model.ItemListRep{}
	if err := c.sendRequest(ctx, http.MethodGet, "/item", q, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("item list: %w", err)
	}

	return rep, nil
}

func (c *Client) ItemGet(ctx context.Context, id string) (model.Item, error) {
	rep := model.Item{}
	if err := c.sendRequest(ctx, http.MethodGet, "/item/"+url.PathEscape(id), nil, nil, &rep, true); err != nil {
		return rep, fmt.Errorf("item get: %w", err)
	}

	return rep, nil
}

func (c *Client) ItemCreate(ctx context.Context, req model.ItemCreateReq) (string, error) {
	rep := model.ItemCreateRep{}
	if err := c.sendRequest(ctx, http.MethodPost, "/item", nil, req, &rep, true); err != nil {
		return "", fmt.Errorf("item create: %w", err)
	}

	return rep.ID, nil
}

func (c *Client) ItemUpdate(ctx context.Context, id string, req model.ItemUpdateReq) error {
	if err := c.sendRequest(ctx, http.MethodPut, "/item/"+url.PathEscape(id), nil, req, nil, true); err != nil {
		return fmt.Errorf("item update: %w", err)
	}

	return nil
}
