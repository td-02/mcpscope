package intercept

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type ParsedMessage struct {
	ID     json.RawMessage
	Method string
	Params json.RawMessage
	Result json.RawMessage
	Error  json.RawMessage
}

type Event struct {
	TraceID         string          `json:"trace_id"`
	Transport       string          `json:"transport"`
	Direction       string          `json:"direction"`
	ReceivedAtUnixN int64           `json:"received_at_unix_nano"`
	SentAtUnixN     int64           `json:"sent_at_unix_nano"`
	LatencyMs       int64           `json:"latency_ms"`
	ID              json.RawMessage `json:"id,omitempty"`
	Method          string          `json:"method,omitempty"`
	Params          json.RawMessage `json:"params,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           json.RawMessage `json:"error,omitempty"`
	ParamsHash      string          `json:"params_hash,omitempty"`
	ResponseHash    string          `json:"response_hash,omitempty"`
	IsError         bool            `json:"is_error"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	ParseError      string          `json:"parse_error,omitempty"`
}

type rpcEnvelope struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type rpcErrorEnvelope struct {
	Message string `json:"message"`
}

func ParseMessage(payload []byte) (ParsedMessage, error) {
	var envelope rpcEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ParsedMessage{}, fmt.Errorf("decode json-rpc payload: %w", err)
	}

	return ParsedMessage{
		ID:     cloneRaw(envelope.ID),
		Method: envelope.Method,
		Params: cloneRaw(envelope.Params),
		Result: cloneRaw(envelope.Result),
		Error:  cloneRaw(envelope.Error),
	}, nil
}

func Capture(transport, direction string, receivedAt, sentAt time.Time, payload []byte) Event {
	event := Event{
		TraceID:         NewUUID(),
		Transport:       transport,
		Direction:       direction,
		ReceivedAtUnixN: receivedAt.UnixNano(),
		SentAtUnixN:     sentAt.UnixNano(),
		LatencyMs:       maxLatencyMs(receivedAt, sentAt),
	}

	parsed, err := ParseMessage(payload)
	if err != nil {
		event.IsError = true
		event.ErrorMessage = err.Error()
		event.ParseError = err.Error()
		return event
	}

	event.ID = parsed.ID
	event.Method = parsed.Method
	event.Params = parsed.Params
	event.Result = parsed.Result
	event.Error = parsed.Error
	event.ParamsHash = hashRaw(parsed.Params)
	event.ResponseHash = hashRaw(selectResponsePayload(parsed))

	if len(parsed.Error) > 0 {
		event.IsError = true
		event.ErrorMessage = extractErrorMessage(parsed.Error)
	}

	return event
}

func EmitLog(w io.Writer, event Event) error {
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal interceptor log: %w", err)
	}

	if _, err := w.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write interceptor log: %w", err)
	}

	return nil
}

func NewUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		now := time.Now().UnixNano()
		return fmt.Sprintf("fallback-%d", now)
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	hexBuf := make([]byte, 36)
	hex.Encode(hexBuf[0:8], buf[0:4])
	hexBuf[8] = '-'
	hex.Encode(hexBuf[9:13], buf[4:6])
	hexBuf[13] = '-'
	hex.Encode(hexBuf[14:18], buf[6:8])
	hexBuf[18] = '-'
	hex.Encode(hexBuf[19:23], buf[8:10])
	hexBuf[23] = '-'
	hex.Encode(hexBuf[24:36], buf[10:16])

	return string(hexBuf)
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	cloned := make([]byte, len(raw))
	copy(cloned, raw)
	return cloned
}

func hashRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func selectResponsePayload(parsed ParsedMessage) json.RawMessage {
	if len(parsed.Result) > 0 {
		return parsed.Result
	}
	return parsed.Error
}

func extractErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var envelope rpcErrorEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return string(raw)
	}
	if envelope.Message == "" {
		return string(raw)
	}
	return envelope.Message
}

func maxLatencyMs(receivedAt, sentAt time.Time) int64 {
	if sentAt.Before(receivedAt) {
		return 0
	}
	return sentAt.Sub(receivedAt).Milliseconds()
}
