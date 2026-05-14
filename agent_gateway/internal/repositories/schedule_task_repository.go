package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
	"gorm.io/gorm"
)

type ScheduleTaskRepository struct {
	*Repository[models.ScheduleTask]
}

func NewScheduleTaskRepository(db *gorm.DB) *ScheduleTaskRepository {
	return &ScheduleTaskRepository{Repository: NewRepository[models.ScheduleTask](db, WithDefaultSort[models.ScheduleTask]("created_at desc"))}
}

func (r *ScheduleTaskRepository) ListEnabledTasks(ctx context.Context) ([]models.ScheduleTask, error) {
	var items []models.ScheduleTask
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&items).Error
	return items, err
}

func (r *ScheduleTaskRepository) DueTasks(ctx context.Context, now time.Time) ([]models.ScheduleTask, error) {
	nowMS := now.UnixMilli()
	var items []models.ScheduleTask
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, nowMS).
		Find(&items).Error
	return items, err
}

func (r *ScheduleTaskRepository) TryLockTask(ctx context.Context, id string, now time.Time) (models.ScheduleTask, bool, error) {
	nowMS := now.UnixMilli()
	tx := r.db.WithContext(ctx).Model(&models.ScheduleTask{}).
		Where("id = ? AND enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", id, true, nowMS).
		Update("next_run_at", nil)
	if tx.Error != nil {
		return models.ScheduleTask{}, false, tx.Error
	}
	if tx.RowsAffected == 0 {
		return models.ScheduleTask{}, false, nil
	}
	var item models.ScheduleTask
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return models.ScheduleTask{}, false, mapScheduleTaskError(err)
	}
	return item, true, nil
}

func (r *ScheduleTaskRepository) CompleteTask(ctx context.Context, completion scheduler.Completion) error {
	updates := map[string]any{
		"last_run_at": completion.StartedAt.UnixMilli(),
		"last_status": completion.Status,
		"last_error":  completion.Error,
		"next_run_at": timePtrToMillis(completion.NextRunAt),
		"updated_at":  completion.FinishedAt,
	}
	if completion.Disable {
		updates["enabled"] = false
		updates["next_run_at"] = nil
	}
	if completion.Delete {
		return r.db.WithContext(ctx).Where("id = ?", completion.TaskID).Delete(&models.ScheduleTask{}).Error
	}
	return r.db.WithContext(ctx).Model(&models.ScheduleTask{}).
		Where("id = ?", completion.TaskID).
		Updates(updates).Error
}

func (r *ScheduleTaskRepository) UpdateNextRun(ctx context.Context, id string, nextRunAt *time.Time) error {
	return r.db.WithContext(ctx).Model(&models.ScheduleTask{}).
		Where("id = ?", id).
		Update("next_run_at", timePtrToMillis(nextRunAt)).Error
}

func timePtrToMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	ms := value.UnixMilli()
	return &ms
}

func mapScheduleTaskError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
