package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterResourceOwnedBy(t *testing.T) {
	t.Parallel()

	own := &ClusterResource{Managed: true, OwnerAppId: "app-1", OwnerRecordId: "sec-1"}
	assert.True(t, own.OwnedBy("app-1", "sec-1"))
	assert.True(t, own.OwnedBy("app-2", "sec-1"), "запись перенесли в другой app")
	assert.True(t, own.OwnedBy("app-1", "sec-9"), "свой app")

	// объект другого app с совпавшим именем
	assert.False(t, own.OwnedBy("app-2", "sec-2"))
	// объект вне kusec (helm-релиз, TLS-секрет)
	assert.False(t, (&ClusterResource{OwnerAppId: "app-1"}).OwnedBy("app-1", "sec-1"))
	assert.False(t, (*ClusterResource)(nil).OwnedBy("app-1", "sec-1"))
}
