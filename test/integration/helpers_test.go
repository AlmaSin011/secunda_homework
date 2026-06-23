//go:build integration

package integration_test

import (
	"github.com/example/go-project/internal/dto"
)

// dto_TeamFilter — узкий фильтр для выборки задач конкретной команды в тестах.
func dto_TeamFilter(teamID uint64) dto.TaskFilter {
	return dto.TaskFilter{
		TeamID: teamID,
		Page:   1,
		Limit:  100,
	}
}

func ptrUint64(v uint64) *uint64 {
	return &v
}
