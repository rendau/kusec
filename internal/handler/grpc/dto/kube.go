package dto

import (
	kubeService "github.com/mechta-market/kusec/internal/service/kube"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
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

func EncodeKubeImportResult(v *kubeService.ImportResult) *proto.KubeImportSecretsRep {
	if v == nil {
		return nil
	}
	return &proto.KubeImportSecretsRep{
		Imported:       v.Imported,
		Skipped:        v.Skipped,
		Errors:         v.Errors,
		CreatedSecrets: v.CreatedSecrets,
		CreatedItems:   v.CreatedItems,
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

// proto → service

func DecodeKubeImportRef(v *proto.KubeImportSecretRefSt, _ int) kubeService.ImportRef {
	if v == nil {
		return kubeService.ImportRef{}
	}
	return kubeService.ImportRef{
		Namespace: v.Namespace,
		Name:      v.Name,
	}
}
