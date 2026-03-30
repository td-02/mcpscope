package store

import (
	"context"
	"time"
)

type Trace struct {
	ID              string
	TraceID         string
	Environment     string
	ServerName      string
	Method          string
	ParamsHash      string
	ParamsPayload   string
	ResponseHash    string
	ResponsePayload string
	LatencyMs       int64
	IsError         bool
	ErrorMessage    string
	CreatedAt       time.Time
}

type QueryFilter struct {
	TraceID      string
	Environment  string
	ServerName   string
	Method       string
	IsError      *bool
	CreatedAfter *time.Time
	Offset       int
	Limit        int
}

type ListOptions struct {
	Limit  int
	Offset int
}

type AlertRule struct {
	ID            string
	Environment   string
	Name          string
	RuleType      string
	Threshold     float64
	WindowMinutes int
	ServerName    string
	Method        string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AlertEvent struct {
	ID             string
	RuleID         string
	Environment    string
	RuleName       string
	Status         string
	PreviousStatus string
	CurrentValue   float64
	Threshold      float64
	SampleCount    int
	Notification   string
	DeliveryStatus string
	DeliveryError  string
	CreatedAt      time.Time
}

type LatencyStat struct {
	Environment string
	ServerName  string
	Method      string
	Count       int
	P50Ms       int64
	P95Ms       int64
	P99Ms       int64
}

type ErrorStat struct {
	Environment        string
	Method             string
	Count              int
	ErrorCount         int
	ErrorRatePct       float64
	RecentErrorMessage string
	RecentErrorAt      *time.Time
}

type TraceStore interface {
	Insert(ctx context.Context, trace Trace) error
	Query(ctx context.Context, filter QueryFilter) ([]Trace, error)
	List(ctx context.Context, opts ListOptions) ([]Trace, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) error
	TrimToCount(ctx context.Context, keep int) error
	UpsertAlertRule(ctx context.Context, rule AlertRule) (AlertRule, error)
	ListAlertRules(ctx context.Context) ([]AlertRule, error)
	DeleteAlertRule(ctx context.Context, id string) error
	InsertAlertEvent(ctx context.Context, event AlertEvent) error
	ListAlertEvents(ctx context.Context, environment string, limit int) ([]AlertEvent, error)
	LatestAlertEvent(ctx context.Context, environment, ruleID string) (*AlertEvent, error)
	QueryLatencyStats(ctx context.Context, filter QueryFilter) ([]LatencyStat, error)
	QueryErrorStats(ctx context.Context, filter QueryFilter) ([]ErrorStat, error)
}
