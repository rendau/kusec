package dto

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	"github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// DecodeTimestamp — nil-safe *timestamppb.Timestamp → *time.Time.
func DecodeTimestamp(v *timestamppb.Timestamp) *time.Time {
	if v == nil {
		return nil
	}
	return new(v.AsTime())
}

// EncodeTimestamp — nil-safe *time.Time → *timestamppb.Timestamp.
func EncodeTimestamp(v *time.Time) *timestamppb.Timestamp {
	if v == nil {
		return nil
	}
	return timestamppb.New(*v)
}

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
