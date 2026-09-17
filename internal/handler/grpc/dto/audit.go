package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeAuditChange(v auditModel.Change, _ int) *proto.AuditChangeSt {
	return &proto.AuditChangeSt{
		Field:     v.Field,
		Old:       v.Old,
		New:       v.New,
		OldHash:   v.OldHash,
		NewHash:   v.NewHash,
		OldSize:   v.OldSize,
		NewSize:   v.NewSize,
		Truncated: v.Truncated,
	}
}

func EncodeAuditMain(v *auditModel.Main, _ int) *proto.AuditMain {
	if v == nil {
		return nil
	}

	result := &proto.AuditMain{
		Id:            v.Id,
		CreatedAt:     timestamppb.New(v.CreatedAt),
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     v.ActorName,
		Source:        v.Source,
		RequestId:     v.RequestId,
		EntityType:    v.EntityType,
		EntityId:      v.EntityId,
		AppId:         v.AppId,
		Namespace:     v.Namespace,
		AppSlug:       v.AppSlug,
		KubeKind:      v.KubeKind,
		KubeName:      v.KubeName,
		Key:           v.Key,
		Action:        v.Action,
		BatchId:       v.BatchId,
	}
	for _, change := range v.Changes {
		result.Changes = append(result.Changes, EncodeAuditChange(change, 0))
	}

	return result
}

// proto → domain

func DecodeAuditListReq(v *proto.AuditListReq) *auditModel.ListReq {
	if v == nil {
		return nil
	}
	return &auditModel.ListReq{
		ListParams:   DecodeListParams(v.ListParams),
		AppId:        v.AppId,
		Namespace:    v.Namespace,
		AppSlug:      v.AppSlug,
		KubeName:     v.KubeName,
		EntityType:   v.EntityType,
		EntityId:     v.EntityId,
		Action:       v.Action,
		ActorUsrId:   v.ActorUsrId,
		Key:          v.Key,
		BatchId:      v.BatchId,
		CreatedAtGte: DecodeTimestamp(v.CreatedAtGte),
		CreatedAtLt:  DecodeTimestamp(v.CreatedAtLt),
	}
}
