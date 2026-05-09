package hooks

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/policy"
)

func TestDispatcherRunsHooksInRegistrationOrder(t *testing.T) {
	var calls []string
	dispatcher := NewDispatcher(
		testHook{name: "first", action: ActionContinue, calls: &calls},
		testHook{name: "second", action: ActionContinue, calls: &calls},
	)

	result, err := dispatcher.Dispatch(context.Background(), Event{Type: EventBeforeRun})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Action != ActionContinue {
		t.Fatalf("action = %s, want continue", result.Action)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestDispatcherModifyPatchesEventForLaterHooks(t *testing.T) {
	var seen string
	dispatcher := NewDispatcher(
		testHook{
			name:   "modify",
			action: ActionModify,
			patches: map[string]any{
				"name": "updated_tool",
				"data": map[string]any{"command": "go test ./..."},
			},
		},
		TypedHook{
			HookName: "observer",
			Func: func(ctx context.Context, event Event) (Result, error) {
				seen = event.Name + ":" + event.Data["command"].(string)
				return Continue(), nil
			},
		},
	)

	result, err := dispatcher.Dispatch(context.Background(), Event{Type: EventBeforeToolCall, Name: "old"})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Action != ActionModify {
		t.Fatalf("action = %s, want modify", result.Action)
	}
	if seen != "updated_tool:go test ./..." {
		t.Fatalf("seen = %q", seen)
	}
	if result.Event.Name != "updated_tool" || result.Event.Data["command"] != "go test ./..." {
		t.Fatalf("patched event = %+v", result.Event)
	}
}

func TestDispatcherStopsOnBlockAndRequestApproval(t *testing.T) {
	tests := []Action{ActionBlock, ActionRequestApproval}
	for _, action := range tests {
		t.Run(string(action), func(t *testing.T) {
			var calls []string
			dispatcher := NewDispatcher(
				testHook{name: "stop", action: action, reason: "needs attention", calls: &calls},
				testHook{name: "later", action: ActionContinue, calls: &calls},
			)

			result, err := dispatcher.Dispatch(context.Background(), Event{Type: EventBeforeRun})
			if err != nil {
				t.Fatalf("Dispatch() error = %v", err)
			}
			if result.Action != action || result.Reason != "needs attention" {
				t.Fatalf("result = %+v", result)
			}
			if !reflect.DeepEqual(calls, []string{"stop"}) {
				t.Fatalf("calls = %+v", calls)
			}
		})
	}
}

func TestDispatcherHookErrorBlocks(t *testing.T) {
	dispatcher := NewDispatcher(TypedHook{
		HookName: "broken",
		Func: func(ctx context.Context, event Event) (Result, error) {
			return Result{}, errors.New("boom")
		},
	})

	result, err := dispatcher.Dispatch(context.Background(), Event{Type: EventBeforeFileWrite})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Action != ActionBlock {
		t.Fatalf("action = %s, want block", result.Action)
	}
	if result.Reason == "" {
		t.Fatalf("missing block reason: %+v", result)
	}
}

func TestSecurityHookUsesPolicyForShellAndFileWrite(t *testing.T) {
	dispatcher := NewDispatcher(NewSecurityHook(policy.New(policy.PermissionModeWorkspaceWrite)))
	result, err := dispatcher.Dispatch(context.Background(), Event{
		Type: EventBeforeShellCommand,
		Data: map[string]any{"command": "git reset --hard"},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Action != ActionRequestApproval {
		t.Fatalf("shell result = %+v, want request_approval", result)
	}

	root := t.TempDir()
	result, err = dispatcher.Dispatch(context.Background(), Event{
		Type: EventBeforeFileWrite,
		CWD:  root,
		Data: map[string]any{"path": filepath.Join(root, "file.txt")},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Action != ActionContinue {
		t.Fatalf("file write result = %+v, want continue", result)
	}
}

func TestPolicyGuardRechecksModifiedEvent(t *testing.T) {
	dispatcher := WithPolicyGuard(
		policy.New(policy.PermissionModeWorkspaceWrite),
		testHook{
			name:   "rewrite-command",
			action: ActionModify,
			patches: map[string]any{
				"data": map[string]any{"command": "rm -rf build"},
			},
		},
		testHook{name: "must-not-run", action: ActionContinue},
	)

	result, err := dispatcher.Dispatch(context.Background(), Event{
		Type: EventBeforeShellCommand,
		Data: map[string]any{"command": "go test ./..."},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result.Action != ActionRequestApproval {
		t.Fatalf("result = %+v, want request_approval after modified command", result)
	}
	if got := len(result.Results); got != 3 {
		t.Fatalf("hook runs = %d, want 3 (security, rewrite, guard)", got)
	}
	if result.Results[len(result.Results)-1].Name != "policy-guard" {
		t.Fatalf("last hook = %q, want policy-guard", result.Results[len(result.Results)-1].Name)
	}
}

type testHook struct {
	name    string
	action  Action
	reason  string
	patches map[string]any
	calls   *[]string
}

func (h testHook) Name() string {
	return h.name
}

func (h testHook) Match(event Event) bool {
	return true
}

func (h testHook) Execute(ctx context.Context, event Event) (Result, error) {
	if h.calls != nil {
		*h.calls = append(*h.calls, h.name)
	}
	return Result{Action: h.action, Reason: h.reason, Patches: h.patches}, nil
}
