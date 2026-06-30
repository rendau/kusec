package dto

import (
	kubeService "github.com/rendau/kusec/internal/service/kube"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// service → proto

func EncodeKubeClusterSecret(v *kubeService.ClusterSecret, _ int) *proto.KubeClusterSecretSt {
	if v == nil {
		return nil
	}
	return &proto.KubeClusterSecretSt{
		Namespace: v.Namespace,
		Name:      v.Name,
		Type:      v.Type,
		Keys:      v.Keys,
		Managed:   v.Managed,
	}
}

func EncodeKubeClusterResource(v *kubeService.ClusterResource, inCluster, found bool) *proto.KubeClusterResourceRep {
	rep := &proto.KubeClusterResourceRep{
		InCluster: inCluster,
		Found:     found,
	}
	if v == nil {
		return rep
	}

	rep.Namespace = v.Namespace
	rep.Name = v.Name
	rep.Type = v.Type
	rep.Managed = v.Managed
	rep.Items = make([]*proto.KubeClusterResourceItemSt, 0, len(v.Items))
	for _, item := range v.Items {
		rep.Items = append(rep.Items, &proto.KubeClusterResourceItemSt{
			Key:      item.Key,
			Value:    item.Value,
			Encoding: item.Encoding,
		})
	}

	return rep
}

func EncodeKubeImportResult(v *kubeService.ImportResult) *proto.KubeImportSecretRep {
	if v == nil {
		return nil
	}
	return &proto.KubeImportSecretRep{
		SecretId:      v.SecretId,
		SecretSlug:    v.SecretSlug,
		SecretCreated: v.SecretCreated,
		CreatedItems:  v.CreatedItems,
		UpdatedItems:  v.UpdatedItems,
	}
}

func EncodeKubeSyncSecretsRep(v *kubeService.SyncResult) *proto.KubeSyncSecretsRep {
	if v == nil {
		return nil
	}
	return &proto.KubeSyncSecretsRep{
		Created:   v.Created,
		Updated:   v.Updated,
		Deleted:   v.Deleted,
		Unchanged: v.Unchanged,
		Errors:    v.Errors,
	}
}

func EncodeKubeSyncConfigMapsRep(v *kubeService.SyncResult) *proto.KubeSyncConfigMapsRep {
	if v == nil {
		return nil
	}
	return &proto.KubeSyncConfigMapsRep{
		Created:   v.Created,
		Updated:   v.Updated,
		Deleted:   v.Deleted,
		Unchanged: v.Unchanged,
		Errors:    v.Errors,
	}
}
