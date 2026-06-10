package dto

import (
	commonModel "github.com/mechta-market/kusec/internal/domain/common/model"
	"github.com/mechta-market/kusec/pkg/proto/common"
)

func DecodeListParams(v *common.ListParamsSt) commonModel.ListParams {
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
