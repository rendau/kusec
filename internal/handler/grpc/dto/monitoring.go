package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	monitoringUsc "github.com/rendau/kusec/internal/usecase/monitoring"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// usecase-модели мониторинга → proto

func EncodeAppKey(v *monitoringUsc.AppKey, _ int) *proto.AppKeySt {
	if v == nil {
		return nil
	}
	return &proto.AppKeySt{
		Key:       v.Key,
		KubeKind:  v.KubeKind,
		KubeName:  v.KubeName,
		IsSecret:  v.IsSecret,
		Active:    v.Active,
		ValueSize: v.ValueSize,
		ValueHash: v.ValueHash,
		UpdatedAt: timestamppb.New(v.UpdatedAt),
		UpdatedBy: v.UpdatedBy,
		Synced:    v.Synced,
		ItemId:    v.ItemId,
		ParentId:  v.ParentId,
	}
}

func EncodeAppDriftObject(v *monitoringUsc.DriftObject, _ int) *proto.AppDriftObjectSt {
	if v == nil {
		return nil
	}
	return &proto.AppDriftObjectSt{
		KubeKind:         v.KubeKind,
		KubeName:         v.KubeName,
		Namespace:        v.Namespace,
		ObjectId:         v.ObjectId,
		ExistsInCluster:  v.ExistsInCluster,
		Managed:          v.Managed,
		MissingInCluster: v.MissingInCluster,
		ExtraInCluster:   v.ExtraInCluster,
		ValueDiffers:     v.ValueDiffers,
		NotSyncedSince:   EncodeTimestamp(v.NotSyncedSince),
	}
}

func EncodeAppResolveRep(v *monitoringUsc.ResolveResult) *proto.AppResolveRep {
	if v == nil {
		return &proto.AppResolveRep{Found: false}
	}
	return &proto.AppResolveRep{
		Found:      true,
		App:        EncodeAppMain(v.App, 0),
		KubeKind:   v.KubeKind,
		ObjectId:   v.ObjectId,
		ObjectSlug: v.ObjectSlug,
	}
}
