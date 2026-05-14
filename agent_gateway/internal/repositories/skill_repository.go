package repositories

import (
	"context"
	"errors"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type SkillRepository struct {
	*Repository[models.Skill]
}

func NewSkillRepository(db *gorm.DB) *SkillRepository {
	return &SkillRepository{Repository: NewRepository[models.Skill](db)}
}

func (r *SkillRepository) Upsert(ctx context.Context, item models.Skill) error {
	var existing models.Skill
	err := r.db.WithContext(ctx).Where("id = ?", item.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(&item).Error
	}
	if err != nil {
		return err
	}
	item.BaseModel = existing.BaseModel
	return r.db.WithContext(ctx).Save(&item).Error
}

func (r *SkillRepository) MarkMissing(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&models.Skill{}).
		Where("id = ?", id).
		Updates(map[string]any{"enabled": false, "last_error": "skill missing"}).Error
}
