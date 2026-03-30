package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"mcpscope/internal/intercept"
	"mcpscope/internal/store"
)

type alertEvaluation struct {
	RuleID          string    `json:"rule_id"`
	Name            string    `json:"name"`
	RuleType        string    `json:"rule_type"`
	Status          string    `json:"status"`
	Threshold       float64   `json:"threshold"`
	CurrentValue    float64   `json:"current_value"`
	WindowMinutes   int       `json:"window_minutes"`
	ServerName      string    `json:"server_name,omitempty"`
	Method          string    `json:"method,omitempty"`
	SampleCount     int       `json:"sample_count"`
	LastEvaluatedAt time.Time `json:"last_evaluated_at"`
}

func evaluateAlertRules(ctx context.Context, traceStore store.TraceStore, now time.Time, rules []store.AlertRule) ([]alertEvaluation, error) {
	evaluations := make([]alertEvaluation, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			evaluations = append(evaluations, alertEvaluation{
				RuleID:          rule.ID,
				Name:            rule.Name,
				RuleType:        rule.RuleType,
				Status:          "disabled",
				Threshold:       rule.Threshold,
				WindowMinutes:   rule.WindowMinutes,
				ServerName:      rule.ServerName,
				Method:          rule.Method,
				LastEvaluatedAt: now.UTC(),
			})
			continue
		}

		start := now.Add(-time.Duration(rule.WindowMinutes) * time.Minute)
		evaluation, err := evaluateAlertRuleSQL(ctx, traceStore, rule, now, start)
		if err != nil {
			return nil, err
		}
		evaluations = append(evaluations, evaluation)
	}

	sort.Slice(evaluations, func(i, j int) bool {
		left := alertSeverity(evaluations[i].Status)
		right := alertSeverity(evaluations[j].Status)
		if left == right {
			return evaluations[i].Name < evaluations[j].Name
		}
		return left > right
	})

	return evaluations, nil
}

func evaluateAlertRuleSQL(ctx context.Context, traceStore store.TraceStore, rule store.AlertRule, now, start time.Time) (alertEvaluation, error) {
	filter := store.QueryFilter{
		Environment:  rule.Environment,
		ServerName:   rule.ServerName,
		Method:       rule.Method,
		CreatedAfter: &start,
	}

	evaluation := alertEvaluation{
		RuleID:          rule.ID,
		Name:            rule.Name,
		RuleType:        rule.RuleType,
		Threshold:       rule.Threshold,
		WindowMinutes:   rule.WindowMinutes,
		ServerName:      rule.ServerName,
		Method:          rule.Method,
		LastEvaluatedAt: now.UTC(),
	}

	switch rule.RuleType {
	case "error_rate":
		stats, err := traceStore.QueryErrorStats(ctx, filter)
		if err != nil {
			return alertEvaluation{}, err
		}
		if len(stats) == 0 {
			evaluation.Status = "no_data"
			return evaluation, nil
		}
		evaluation.SampleCount = stats[0].Count
		evaluation.CurrentValue = stats[0].ErrorRatePct
	case "latency_p95":
		stats, err := traceStore.QueryLatencyStats(ctx, filter)
		if err != nil {
			return alertEvaluation{}, err
		}
		if len(stats) == 0 {
			evaluation.Status = "no_data"
			return evaluation, nil
		}
		evaluation.SampleCount = stats[0].Count
		evaluation.CurrentValue = float64(stats[0].P95Ms)
	default:
		return alertEvaluation{}, fmt.Errorf("unsupported alert rule type %q", rule.RuleType)
	}

	if evaluation.CurrentValue >= evaluation.Threshold {
		evaluation.Status = "firing"
	} else {
		evaluation.Status = "ok"
	}

	return evaluation, nil
}

func processAlertEvaluations(ctx context.Context, cfg Config) error {
	if cfg.Store == nil {
		return nil
	}

	rules, err := cfg.Store.ListAlertRules(ctx)
	if err != nil {
		return err
	}

	filtered := make([]store.AlertRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Environment == cfg.Environment {
			filtered = append(filtered, rule)
		}
	}

	evaluations, err := evaluateAlertRules(ctx, cfg.Store, time.Now().UTC(), filtered)
	if err != nil {
		return err
	}

	for _, evaluation := range evaluations {
		previous, err := cfg.Store.LatestAlertEvent(ctx, cfg.Environment, evaluation.RuleID)
		if err != nil {
			return err
		}

		previousStatus := ""
		if previous != nil {
			previousStatus = previous.Status
		}
		if previousStatus == evaluation.Status {
			continue
		}

		event := store.AlertEvent{
			ID:             intercept.NewUUID(),
			RuleID:         evaluation.RuleID,
			Environment:    cfg.Environment,
			RuleName:       evaluation.Name,
			Status:         evaluation.Status,
			PreviousStatus: previousStatus,
			CurrentValue:   evaluation.CurrentValue,
			Threshold:      evaluation.Threshold,
			SampleCount:    evaluation.SampleCount,
			CreatedAt:      evaluation.LastEvaluatedAt,
		}

		event.Notification, event.DeliveryStatus, event.DeliveryError = deliverAlertNotifications(ctx, cfg.NotifyWebhooks, evaluation)
		if err := cfg.Store.InsertAlertEvent(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

func evaluateAlertRule(rule store.AlertRule, now time.Time, traces []store.Trace) (alertEvaluation, error) {
	evaluation := alertEvaluation{
		RuleID:          rule.ID,
		Name:            rule.Name,
		RuleType:        rule.RuleType,
		Threshold:       rule.Threshold,
		WindowMinutes:   rule.WindowMinutes,
		ServerName:      rule.ServerName,
		Method:          rule.Method,
		SampleCount:     len(traces),
		LastEvaluatedAt: now.UTC(),
	}

	if len(traces) == 0 {
		evaluation.Status = "no_data"
		return evaluation, nil
	}

	switch rule.RuleType {
	case "error_rate":
		var errors int
		for _, trace := range traces {
			if trace.IsError {
				errors++
			}
		}
		evaluation.CurrentValue = float64(errors) * 100 / float64(len(traces))
	case "latency_p95":
		values := make([]int64, 0, len(traces))
		for _, trace := range traces {
			values = append(values, trace.LatencyMs)
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		evaluation.CurrentValue = float64(percentile(values, 0.95))
	default:
		return alertEvaluation{}, fmt.Errorf("unsupported alert rule type %q", rule.RuleType)
	}

	if evaluation.CurrentValue >= evaluation.Threshold {
		evaluation.Status = "firing"
	} else {
		evaluation.Status = "ok"
	}

	return evaluation, nil
}

func alertSeverity(status string) int {
	switch status {
	case "firing":
		return 3
	case "ok":
		return 2
	case "no_data":
		return 1
	default:
		return 0
	}
}

func deliverAlertNotifications(ctx context.Context, webhooks []string, evaluation alertEvaluation) (string, string, string) {
	if len(webhooks) == 0 {
		return "", "skipped", ""
	}

	payload, err := json.Marshal(evaluation)
	if err != nil {
		return "", "failed", err.Error()
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, webhook := range webhooks {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
		if err != nil {
			return webhook, "failed", err.Error()
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return webhook, "failed", err.Error()
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return webhook, "failed", fmt.Sprintf("http %d", resp.StatusCode)
		}
	}

	return webhooks[0], "sent", ""
}
