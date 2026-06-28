package service

import (
	"context"

	"github.com/example/go-project/internal/dto"
)

type StatsService struct {
	stats StatsRepository
}

func NewStatsService(stats StatsRepository) *StatsService {
	return &StatsService{stats: stats}
}

func (s *StatsService) LastWeek(ctx context.Context) ([]dto.TeamStatsResponse, error) {
	return s.stats.TeamStatsLastWeek(ctx)
}

func (s *StatsService) TopCreators(ctx context.Context, sinceDays, limit int) ([]dto.TopCreatorEntry, error) {
	if sinceDays <= 0 {
		sinceDays = 30
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return s.stats.TopCreatorsByTeam(ctx, sinceDays, limit)
}

func (s *StatsService) Orphans(ctx context.Context) ([]dto.OrphanTaskResponse, error) {
	return s.stats.OrphanTasks(ctx)
}
