package admin

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
	"gorm.io/gorm"
)

func TestScheduleTaskServiceCreateComputesNextRunAndWakes(t *testing.T) {
	service := newScheduleTaskServiceForTest(t)
	wakes := 0
	service.SetOnChange(func() { wakes++ })

	created, err := service.Create(context.Background(), models.ScheduleTask{
		Name:    "every minute",
		Type:    "every",
		Spec:    "1m",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.NextRunAt == nil {
		t.Fatal("NextRunAt = nil, want computed value")
	}
	if wakes != 1 {
		t.Fatalf("wakes = %d, want 1", wakes)
	}
}

func TestScheduleTaskServicePayloadAndDeleteAfterRunMapping(t *testing.T) {
	now := time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC)
	ms := now.UnixMilli()
	task := scheduleModelToRuntime(models.ScheduleTask{
		BaseModel:      models.BaseModel{ID: "task-1"},
		Name:           "payload",
		Type:           "at",
		AgentID:        "agent-from-field",
		Content:        "field prompt",
		PayloadJSON:    `{"agentId":"agent-from-json","prompt":"json prompt","data":{"priority":"high"}}`,
		NextRunAt:      &ms,
		Enabled:        true,
		DeleteAfterRun: true,
	})

	if task.Payload.AgentID != "agent-from-json" {
		t.Fatalf("AgentID = %q", task.Payload.AgentID)
	}
	if task.Payload.Prompt != "json prompt" {
		t.Fatalf("Prompt = %q", task.Payload.Prompt)
	}
	if task.Payload.Data["priority"] != "high" {
		t.Fatalf("Data = %#v", task.Payload.Data)
	}
	if !task.DeleteAfterRun {
		t.Fatal("DeleteAfterRun = false, want true")
	}
	if task.Schedule.At == nil || !task.Schedule.At.Equal(now) {
		t.Fatalf("Schedule.At = %v, want %v", task.Schedule.At, now)
	}
}

func TestScheduleTaskRepositoryCompleteDeletesOneTimeTask(t *testing.T) {
	db := newAdminTestDB(t)
	repo := repositories.NewScheduleTaskRepository(db)
	task, err := repo.Create(context.Background(), models.ScheduleTask{
		Name:           "one shot",
		Type:           "at",
		Enabled:        true,
		DeleteAfterRun: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = repo.CompleteTask(context.Background(), completionForDelete(task.ID))
	if err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if _, err := repo.GetByID(context.Background(), task.ID); err == nil {
		t.Fatal("GetByID() error = nil, want deleted task")
	}
}

func newScheduleTaskServiceForTest(t *testing.T) *ScheduleTaskService {
	t.Helper()
	return NewScheduleTaskService(repositories.NewScheduleTaskRepository(newAdminTestDB(t)))
}

func newAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ScheduleTask{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func completionForDelete(taskID string) scheduler.Completion {
	now := time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC)
	return scheduler.Completion{
		TaskID:     taskID,
		StartedAt:  now,
		FinishedAt: now.Add(time.Second),
		Status:     "ok",
		Delete:     true,
	}
}
