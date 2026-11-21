package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/abeta-dev/kmp/bff/pkg/sdui/actions"
	"github.com/abeta-dev/sdui-bff/internal/domain"
)

// Compile-time check ensuring DefaultActionExecutor satisfies domain.ActionExecutor.
var _ domain.ActionExecutor = (*DefaultActionExecutor)(nil)

// DefaultActionExecutor dispatches generic POST /api/v1/actions/execute requests
// and guarantees string-only values inside all KMP analytics tracking property maps.
type DefaultActionExecutor struct{}

// NewActionExecutor creates a DefaultActionExecutor.
func NewActionExecutor() *DefaultActionExecutor {
	return &DefaultActionExecutor{}
}

// Execute processes a generic SDUI action request and returns an ActionResult
// with structured follow_up_actions compatible with KMP's RemoteApiCallExecutor.
func (e *DefaultActionExecutor) Execute(ctx context.Context, req domain.ActionExecuteRequest) (domain.ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActionResult{}, err
	}

	actionType := strings.ToLower(strings.TrimSpace(req.ActionType))
	if actionType == "" {
		return domain.ActionResult{}, fmt.Errorf("%w: action_type is required", domain.ErrInvalidInput)
	}

	actionID := strings.TrimSpace(req.ActionID)
	if actionID == "" {
		actionID = "act_" + actionType
	}

	switch actionType {
	case string(actions.ActionTrackEvent):
		return e.executeTrackEvent(actionID, req.Payload), nil
	case string(actions.ActionShowToast):
		return e.executeShowToast(actionID, req.Payload), nil
	case string(actions.ActionNavigate):
		return e.executeNavigate(actionID, req.Payload)
	case "submit_form":
		return e.executeSubmitForm(actionID, req.Payload), nil
	default:
		return e.executeDefault(actionID, actionType, req.Payload), nil
	}
}

func (e *DefaultActionExecutor) executeTrackEvent(actionID string, payload map[string]any) domain.ActionResult {
	eventName := payloadString(payload, "event_name")
	if eventName == "" {
		eventName = payloadString(payload, "event")
	}
	if eventName == "" {
		eventName = "custom_event"
	}

	props := stringifyProperties(payload["properties"])
	props["action_id"] = actionID
	props["event_name"] = eventName

	return domain.ActionResult{
		Status:  "executed",
		Message: fmt.Sprintf("Tracked event %q", eventName),
		Data: map[string]any{
			"action_id":   actionID,
			"action_type": string(actions.ActionTrackEvent),
			"event_name":  eventName,
			"properties":  props,
		},
	}
}

func (e *DefaultActionExecutor) executeShowToast(actionID string, payload map[string]any) domain.ActionResult {
	message := payloadString(payload, "message")
	if message == "" {
		message = "Action completed"
	}
	variant := payloadString(payload, "variant")
	if variant == "" {
		variant = "info"
	}
	durationMs := payloadInt(payload, "duration_ms", 2200)
	if durationMs <= 0 {
		durationMs = 2200
	}

	toast := actions.NewShowToastAction(actionID+"_toast", actions.ShowToastPayload{
		Message:    message,
		DurationMs: durationMs,
		Variant:    variant,
	})

	return domain.ActionResult{
		Status:          "executed",
		Message:         message,
		FollowUpActions: []actions.ActionTrigger{toast},
		Data: map[string]any{
			"action_id":   actionID,
			"action_type": string(actions.ActionShowToast),
			"message":     message,
			"variant":     variant,
		},
	}
}

func (e *DefaultActionExecutor) executeNavigate(actionID string, payload map[string]any) (domain.ActionResult, error) {
	route := payloadString(payload, "route")
	if route == "" {
		return domain.ActionResult{}, fmt.Errorf("%w: navigate payload requires non-empty route", domain.ErrInvalidInput)
	}

	params := payloadStringMap(payload["params"])
	clearBackStack, _ := payload["clear_back_stack"].(bool)
	launchMode := payloadString(payload, "launch_mode")

	nav := actions.NewNavigateAction(actionID+"_nav", actions.NavigatePayload{
		Route:          route,
		Params:         params,
		ClearBackStack: clearBackStack,
		LaunchMode:     launchMode,
	})

	return domain.ActionResult{
		Status:          "executed",
		Message:         fmt.Sprintf("Navigating to %s", route),
		FollowUpActions: []actions.ActionTrigger{nav},
		Data: map[string]any{
			"action_id":   actionID,
			"action_type": string(actions.ActionNavigate),
			"route":       route,
		},
	}, nil
}

func (e *DefaultActionExecutor) executeSubmitForm(actionID string, payload map[string]any) domain.ActionResult {
	formID := payloadString(payload, "form_id")
	if formID == "" {
		formID = "form"
	}

	feedback := actions.NewTriggerFeedbackAction(actionID+"_feedback", actions.TriggerFeedbackPayload{
		Effect:     actions.FeedbackCelebration,
		Title:      "Submission Received",
		Subtitle:   fmt.Sprintf("Form %s saved successfully", formID),
		DurationMs: 2400,
	})
	toast := actions.NewShowToastAction(actionID+"_toast", actions.ShowToastPayload{
		Message:    fmt.Sprintf("Form %s submitted", formID),
		DurationMs: 2200,
		Variant:    "success",
	})
	track := actions.NewTrackEventAction(actionID+"_track", actions.TrackEventPayload{
		EventName: "form_submitted",
		Properties: map[string]any{
			"action_id":   actionID,
			"form_id":     formID,
			"field_count": strconv.Itoa(len(payload)),
		},
	})

	return domain.ActionResult{
		Status:          "executed",
		Message:         fmt.Sprintf("Form %q submitted", formID),
		FollowUpActions: []actions.ActionTrigger{feedback, toast, track},
		Data: map[string]any{
			"action_id":   actionID,
			"action_type": "submit_form",
			"form_id":     formID,
			"field_count": len(payload),
		},
	}
}

func (e *DefaultActionExecutor) executeDefault(actionID, actionType string, payload map[string]any) domain.ActionResult {
	props := map[string]any{
		"action_id":   actionID,
		"action_type": actionType,
	}
	if screenID := payloadString(payload, "screen_id"); screenID != "" {
		props["screen_id"] = screenID
	}

	track := actions.NewTrackEventAction(actionID+"_track", actions.TrackEventPayload{
		EventName:  "action_executed",
		Properties: props,
	})

	return domain.ActionResult{
		Status:          "executed",
		Message:         fmt.Sprintf("Action %s executed", actionType),
		FollowUpActions: []actions.ActionTrigger{track},
		Data: map[string]any{
			"action_id":   actionID,
			"action_type": actionType,
		},
	}
}

// stringifyProperties coerces any incoming property map into map[string]any
// containing strictly string values, honoring the KMP Map<String, String> wire rule.
func stringifyProperties(raw any) map[string]any {
	out := make(map[string]any)
	switch m := raw.(type) {
	case map[string]string:
		for k, v := range m {
			out[k] = v
		}
	case map[string]any:
		for k, v := range m {
			out[k] = formatScalarString(v)
		}
	}
	return out
}

func payloadStringMap(raw any) map[string]string {
	switch m := raw.(type) {
	case map[string]string:
		if len(m) == 0 {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	case map[string]any:
		if len(m) == 0 {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = formatScalarString(v)
		}
		return out
	default:
		return nil
	}
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(formatScalarString(raw))
}

func payloadInt(payload map[string]any, key string, fallback int) int {
	if payload == nil {
		return fallback
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return fallback
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func formatScalarString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprint(val)
	}
}
