package admin

import (
	"context"
	"encoding/json"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	runtimeskills "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/skills"
)

type SkillService struct {
	*Service[models.Skill]
	repo    *repositories.SkillRepository
	scanner *runtimeskills.Scanner
}

func NewSkillService(repo *repositories.SkillRepository) *SkillService {
	return &SkillService{Service: NewService[models.Skill](repo, nil), repo: repo}
}

func (s *SkillService) SetScanner(scanner *runtimeskills.Scanner) {
	s.scanner = scanner
}

func (s *SkillService) Scan(ctx context.Context) (runtimeskills.ScanResult, error) {
	if s.scanner == nil {
		return runtimeskills.ScanResult{}, nil
	}
	return s.scanner.Scan(ctx)
}

func (s *SkillService) Reload(ctx context.Context, id string) (runtimeskills.Skill, error) {
	if s.scanner == nil {
		return runtimeskills.Skill{}, runtimeskills.ErrSkillNotFound
	}
	return s.scanner.Reload(ctx, id)
}

func (s *SkillService) Documentation(ctx context.Context, id string) (string, error) {
	if s.scanner == nil {
		return "", runtimeskills.ErrSkillNotFound
	}
	return s.scanner.Documentation(ctx, id)
}

func (s *SkillService) UpsertSkill(ctx context.Context, skill runtimeskills.Skill) error {
	manifest, _ := json.Marshal(skill.Manifest)
	return s.repo.Upsert(ctx, models.Skill{
		BaseModel:     models.BaseModel{ID: skill.ID},
		Name:          skill.Manifest.Name,
		Description:   skill.Manifest.Description,
		Source:        "local",
		Path:          skill.BasePath,
		Version:       skill.Manifest.Metadata["version"],
		ContentHash:   skill.ContentHash,
		ManifestJSON:  string(manifest),
		Documentation: skill.Instructions,
		Enabled:       true,
	})
}

func (s *SkillService) MarkSkillMissing(ctx context.Context, id string) error {
	return s.repo.MarkMissing(ctx, id)
}
