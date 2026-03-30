package proxy

import (
	"testing"
	"time"

	"mcpscope/internal/store"
)

func TestEvaluateAlertRuleFiringOnErrorRate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	evaluation, err := evaluateAlertRule(store.AlertRule{
		ID:            "rule-1",
		Name:          "Error budget",
		RuleType:      "error_rate",
		Threshold:     25,
		WindowMinutes: 15,
	}, now, []store.Trace{
		{IsError: true},
		{IsError: false},
		{IsError: true},
	})
	if err != nil {
		t.Fatalf("evaluateAlertRule returned error: %v", err)
	}
	if evaluation.Status != "firing" {
		t.Fatalf("status = %q", evaluation.Status)
	}
	if evaluation.CurrentValue <= 25 {
		t.Fatalf("current_value = %f", evaluation.CurrentValue)
	}
}

func TestEvaluateAlertRuleNoData(t *testing.T) {
	t.Parallel()

	evaluation, err := evaluateAlertRule(store.AlertRule{
		ID:            "rule-2",
		Name:          "Latency",
		RuleType:      "latency_p95",
		Threshold:     200,
		WindowMinutes: 15,
	}, time.Now(), nil)
	if err != nil {
		t.Fatalf("evaluateAlertRule returned error: %v", err)
	}
	if evaluation.Status != "no_data" {
		t.Fatalf("status = %q", evaluation.Status)
	}
}
