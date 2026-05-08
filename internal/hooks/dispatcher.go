package hooks

import (
	"context"
	"fmt"
	"time"
)

type DefaultDispatcher struct {
	hooks []Hook
}

func NewDispatcher(hooks ...Hook) *DefaultDispatcher {
	d := &DefaultDispatcher{}
	for _, hook := range hooks {
		d.Register(hook)
	}
	return d
}

func (d *DefaultDispatcher) Register(hook Hook) {
	if hook == nil {
		return
	}
	d.hooks = append(d.hooks, hook)
}

func (d *DefaultDispatcher) Dispatch(ctx context.Context, event Event) (DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	current := CloneEvent(event)
	dispatch := DispatchResult{
		Action: ActionContinue,
		Event:  current,
	}

	for _, hook := range d.hooks {
		if !hook.Match(current) {
			continue
		}
		result, err := hook.Execute(ctx, CloneEvent(current))
		if err != nil {
			block := Block(fmt.Sprintf("hook %q failed: %v", hook.Name(), err))
			dispatch.Action = ActionBlock
			dispatch.Reason = block.Reason
			dispatch.Results = append(dispatch.Results, HookRun{Name: hook.Name(), Result: block})
			dispatch.Event = current
			return dispatch, nil
		}
		if result.Action == "" {
			result.Action = ActionContinue
		}
		dispatch.Results = append(dispatch.Results, HookRun{Name: hook.Name(), Result: result})

		switch result.Action {
		case ActionContinue:
		case ActionModify:
			current = applyPatches(current, result.Patches)
			dispatch.Action = ActionModify
			dispatch.Event = current
			if result.Reason != "" {
				dispatch.Reason = result.Reason
			}
		case ActionBlock, ActionRequestApproval:
			dispatch.Action = result.Action
			dispatch.Reason = result.Reason
			dispatch.Data = result.Data
			dispatch.Event = current
			return dispatch, nil
		default:
			block := Block(fmt.Sprintf("hook %q returned unknown action %q", hook.Name(), result.Action))
			dispatch.Action = ActionBlock
			dispatch.Reason = block.Reason
			dispatch.Results = append(dispatch.Results, HookRun{Name: hook.Name(), Result: block})
			dispatch.Event = current
			return dispatch, nil
		}

		if err := ctx.Err(); err != nil {
			return DispatchResult{}, err
		}
	}

	dispatch.Event = current
	return dispatch, nil
}

func applyPatches(event Event, patches map[string]any) Event {
	if len(patches) == 0 {
		return event
	}
	out := CloneEvent(event)
	for key, value := range patches {
		switch key {
		case "session_id":
			if str, ok := value.(string); ok {
				out.SessionID = str
			}
		case "name":
			if str, ok := value.(string); ok {
				out.Name = str
			}
		case "cwd":
			if str, ok := value.(string); ok {
				out.CWD = str
			}
		case "error":
			if str, ok := value.(string); ok {
				out.Error = str
			}
		case "data":
			if patchData, ok := value.(map[string]any); ok {
				if out.Data == nil {
					out.Data = map[string]any{}
				}
				for dataKey, dataValue := range patchData {
					out.Data[dataKey] = cloneValue(dataValue)
				}
			}
		default:
			if out.Data == nil {
				out.Data = map[string]any{}
			}
			out.Data[key] = cloneValue(value)
		}
	}
	return out
}

type TypedHook struct {
	HookName string
	Events   []EventType
	Func     func(context.Context, Event) (Result, error)
}

func (h TypedHook) Name() string {
	return h.HookName
}

func (h TypedHook) Match(event Event) bool {
	if len(h.Events) == 0 {
		return true
	}
	for _, typ := range h.Events {
		if event.Type == typ {
			return true
		}
	}
	return false
}

func (h TypedHook) Execute(ctx context.Context, event Event) (Result, error) {
	if h.Func == nil {
		return Continue(), nil
	}
	return h.Func(ctx, event)
}
