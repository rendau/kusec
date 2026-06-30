package dto

import (
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	"github.com/rendau/kusec/pkg/proto/kusec_v1"
)

func DecodeListParams(v *kusec_v1.ListParamsSt) commonModel.ListParams {
	if v == nil {
		return commonModel.ListParams{}
	}
	return commonModel.ListParams{
		Page:           v.Page,
		PageSize:       v.PageSize,
		WithTotalCount: v.WithTotalCount,
		OnlyCount:      v.OnlyCount,
		SortName:       v.SortName,
		Sort:           v.Sort,
	}
}
