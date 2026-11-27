package kube

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Ошибка проверки k8s может повторять содержимое поля (у dockerconfigjson —
// символ, на котором споткнулся разбор JSON): в тексте sync остаются только
// пути полей.
func TestKubeErrText(t *testing.T) {
	t.Parallel()

	gk := schema.GroupKind{Kind: "Secret"}
	invalid := k8serrors.NewInvalid(gk, "kusec-app-db", field.ErrorList{
		field.Invalid(field.NewPath("data").Key(".dockerconfigjson"), "<secret contents redacted>", "invalid character 'S' looking for beginning of value"),
		field.Invalid(field.NewPath("data").Key(".dockerconfigjson"), "x", "second"),
	})

	text := kubeErrText(fmt.Errorf("wrap: %w", invalid))
	assert.Equal(t, "invalid object: rejected by k8s validation, fields: data[.dockerconfigjson]", text)
	assert.NotContains(t, text, "'S'")

	assert.Equal(t, "invalid object: rejected by k8s validation", kubeErrText(k8serrors.NewInvalid(gk, "x", nil)))

	conflict := k8serrors.NewConflict(schema.GroupResource{Resource: "secrets"}, "kusec-app-db", errors.New("changed"))
	assert.Equal(t, conflict.Error(), kubeErrText(conflict))
}
