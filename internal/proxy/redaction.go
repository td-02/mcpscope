package proxy

import (
	"encoding/json"
	"strings"

	"mcpscope/internal/intercept"
)

const redactedValue = "[REDACTED]"

type payloadRedactor struct {
	keys map[string]struct{}
}

func newPayloadRedactor(keys []string) *payloadRedactor {
	if len(keys) == 0 {
		return nil
	}

	normalized := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		normalized[key] = struct{}{}
	}
	if len(normalized) == 0 {
		return nil
	}

	return &payloadRedactor{keys: normalized}
}

func (r *payloadRedactor) Raw(raw json.RawMessage) json.RawMessage {
	if r == nil || len(raw) == 0 {
		return cloneRawMessage(raw)
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return cloneRawMessage(raw)
	}

	redacted := r.value(value)
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return cloneRawMessage(raw)
	}

	return encoded
}

func (r *payloadRedactor) Event(event intercept.Event) intercept.Event {
	if r == nil {
		return event
	}

	event.Params = r.Raw(event.Params)
	event.Result = r.Raw(event.Result)
	event.Error = r.Raw(event.Error)
	event.ParamsHash = hashRawMessage(event.Params)
	event.ResponseHash = hashRawMessage(selectResponsePayload(event))
	return event
}

func (r *payloadRedactor) value(input any) any {
	switch typed := input.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			if _, ok := r.keys[strings.ToLower(key)]; ok {
				out[key] = redactedValue
				continue
			}
			out[key] = r.value(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = r.value(value)
		}
		return out
	default:
		return input
	}
}
