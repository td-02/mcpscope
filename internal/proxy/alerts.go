package proxy

import (
	"context"
	"fmt"
	"sort"
	"time"

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
		traces, err := traceStore.Query(ctx, store.QueryFilter{
			ServerName:   rule.ServerName,
			Method:       rule.Method,
			CreatedAfter: &start,
		})
		if err != nil {
			return nil, err
		}

		evaluation, err := evaluateAlertRule(rule, now, traces)
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
