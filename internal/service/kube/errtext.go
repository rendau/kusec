package kube

import (
	"errors"
	"strings"

	"github.com/samber/lo"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// kubeErrText — текст ошибки k8s API для результата и журнала sync.
//
// Тексты sync уходят и MCP-агентам, которым значения секретов не видны,
// поэтому в них не должно быть ничего, выведенного из значения. Ошибка
// проверки объекта (Invalid) может повторять содержимое поля: у секрета
// kubernetes.io/dockerconfigjson в ней символ, на котором споткнулся разбор
// JSON. От неё остаются только пути полей, без сообщений. Остальные ошибки
// (конфликт, нет прав, таймаут) содержат лишь имена объектов.
func kubeErrText(err error) string {
	statusErr, ok := errors.AsType[*k8serrors.StatusError](err)
	if !ok || statusErr.ErrStatus.Reason != metav1.StatusReasonInvalid {
		return err.Error()
	}

	var fields []string
	if details := statusErr.ErrStatus.Details; details != nil {
		fields = lo.Uniq(lo.FilterMap(details.Causes, func(c metav1.StatusCause, _ int) (string, bool) {
			return clipName(c.Field), c.Field != ""
		}))
	}
	if len(fields) == 0 {
		return "invalid object: rejected by k8s validation"
	}

	return "invalid object: rejected by k8s validation, fields: " + strings.Join(fields, ", ")
}
