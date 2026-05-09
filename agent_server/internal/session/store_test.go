package session

import (
	"context"
	"errors"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/agent"
)

func TestFileStoreCreateGetUpdateList(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(t.TempDir())

	created, err := store.Create(ctx, agent.Session{CWD: "E:/repo", Model: "gpt-4.1"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() did not assign session id")
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.CWD != "E:/repo" || got.Model != "gpt-4.1" {
		t.Fatalf("Get() = %#v", got)
	}

	got.Model = "gpt-4.1-mini"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get(updated) error = %v", err)
	}
	if updated.Model != "gpt-4.1-mini" {
		t.Fatalf("updated model = %q", updated.Model)
	}

	sessions, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("List() len = %d", len(sessions))
	}
}

func TestFileStoreGetMissing(t *testing.T) {
	_, err := NewFileStore(t.TempDir()).Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestFileStoreRejectsUnsafeID(t *testing.T) {
	store := NewFileStore(t.TempDir())
	err := store.Update(context.Background(), agent.Session{ID: "../escape"})
	if err == nil {
		t.Fatal("Update() error = nil, want invalid id error")
	}
}
