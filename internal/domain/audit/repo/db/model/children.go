package model

import (
	"encoding/json"

	"github.com/samber/lo"

	domainModel "github.com/rendau/kusec/internal/domain/audit/model"
)

// changeDto — repo-копия domainModel.Change для (де)сериализации jsonb-колонки
// changes (байтовый подход: колонка читается/пишется как []byte).
type changeDto struct {
	Field     string  `json:"field"`
	Old       *string `json:"old,omitempty"`
	New       *string `json:"new,omitempty"`
	OldHash   string  `json:"old_hash,omitempty"`
	NewHash   string  `json:"new_hash,omitempty"`
	OldSize   *int64  `json:"old_size,omitempty"`
	NewSize   *int64  `json:"new_size,omitempty"`
	Truncated bool    `json:"truncated,omitempty"`
}

func decodeChangeDto(v changeDto, _ int) domainModel.Change {
	return domainModel.Change{
		Field:     v.Field,
		Old:       v.Old,
		New:       v.New,
		OldHash:   v.OldHash,
		NewHash:   v.NewHash,
		OldSize:   v.OldSize,
		NewSize:   v.NewSize,
		Truncated: v.Truncated,
	}
}

func encodeChangeDto(v domainModel.Change, _ int) changeDto {
	return changeDto{
		Field:     v.Field,
		Old:       v.Old,
		New:       v.New,
		OldHash:   v.OldHash,
		NewHash:   v.NewHash,
		OldSize:   v.OldSize,
		NewSize:   v.NewSize,
		Truncated: v.Truncated,
	}
}

// decodeChanges разбирает jsonb-колонку; битый json невозможен (пишем сами),
// но на всякий случай отдаём пустой список, а не роняем выборку.
func decodeChanges(raw []byte) []domainModel.Change {
	if len(raw) == 0 {
		return []domainModel.Change{}
	}
	dtos := make([]changeDto, 0)
	if err := json.Unmarshal(raw, &dtos); err != nil {
		return []domainModel.Change{}
	}
	return lo.Map(dtos, decodeChangeDto)
}

// encodeChanges сериализует изменения для записи в jsonb-колонку.
func encodeChanges(changes []domainModel.Change) ([]byte, error) {
	if changes == nil {
		changes = []domainModel.Change{}
	}
	return json.Marshal(lo.Map(changes, encodeChangeDto))
}
