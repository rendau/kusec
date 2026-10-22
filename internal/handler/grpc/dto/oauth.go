package dto

import (
	"net/url"

	"github.com/rendau/kusec/internal/domain/oauth/model"
	usecaseOAuth "github.com/rendau/kusec/internal/usecase/oauth"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

func DecodeOAuthAuthorizeReq(v *proto.OAuthAuthorizeReq) *usecaseOAuth.AuthorizeReq {
	return &usecaseOAuth.AuthorizeReq{
		ResponseType:        v.ResponseType,
		ClientId:            v.ClientId,
		RedirectUri:         v.RedirectUri,
		State:               v.State,
		CodeChallenge:       v.CodeChallenge,
		CodeChallengeMethod: v.CodeChallengeMethod,
		Scope:               v.Scope,
		Resource:            v.Resource,
	}
}

func EncodeOAuthAuthorizeInfo(v *usecaseOAuth.AuthorizeInfo) *proto.OAuthAuthorizeInfoRep {
	return &proto.OAuthAuthorizeInfoRep{
		ClientName:   v.ClientName,
		RedirectUri:  v.RedirectUri,
		RedirectHost: v.RedirectHost,
	}
}

func EncodeOAuthGrantMain(v *model.Grant, _ int) *proto.OAuthGrantMain {
	result := &proto.OAuthGrantMain{
		Id:            v.Id,
		ClientName:    v.ClientName,
		CreatedAtUnix: v.CreatedAt.Unix(),
	}
	if parsed, err := url.Parse(v.RedirectUri); err == nil {
		result.RedirectHost = parsed.Host
	}
	if v.ConnectedAt != nil {
		result.ConnectedAtUnix = v.ConnectedAt.Unix()
	}
	if v.LastUsedAt != nil {
		result.LastUsedAtUnix = v.LastUsedAt.Unix()
	}
	return result
}
