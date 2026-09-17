package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeSyncRunObject(v *syncrunModel.Object, _ int) *proto.SyncRunObjectSt {
	if v == nil {
		return nil
	}
	changedKeys := v.ChangedKeys
	if changedKeys == nil {
		changedKeys = []string{}
	}
	return &proto.SyncRunObjectSt{
		Namespace:   v.Namespace,
		KubeKind:    v.KubeKind,
		KubeName:    v.KubeName,
		Op:          v.Op,
		Error:       v.Error,
		ContentHash: v.ContentHash,
		ChangedKeys: changedKeys,
	}
}

func EncodeSyncRunMain(v *syncrunModel.Main, _ int) *proto.SyncRunMain {
	if v == nil {
		return nil
	}

	result := &proto.SyncRunMain{
		Id:            v.Id,
		StartedAt:     timestamppb.New(v.StartedAt),
		FinishedAt:    EncodeTimestamp(v.FinishedAt),
		Status:        v.Status,
		Error:         v.Error,
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     v.ActorName,
		Source:        v.Source,
		RequestId:     v.RequestId,
		AppId:         v.AppId,
		DurationMs:    v.DurationMs,
	}
	for _, object := range v.Objects {
		result.Objects = append(result.Objects, EncodeSyncRunObject(object, 0))
	}

	return result
}

// proto → domain

func DecodeSyncRunListReq(v *proto.SyncRunListReq) *syncrunModel.ListReq {
	if v == nil {
		return nil
	}
	return &syncrunModel.ListReq{
		ListParams:   DecodeListParams(v.ListParams),
		AppId:        v.AppId,
		Namespace:    v.Namespace,
		Status:       v.Status,
		StartedAtGte: DecodeTimestamp(v.StartedAtGte),
		StartedAtLt:  DecodeTimestamp(v.StartedAtLt),
	}
}
