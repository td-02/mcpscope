package store

import (
	"context"
	"time"
)

type Trace struct {
	ID              string
	TraceID         string
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

type TraceStore interface {
	Insert(ctx context.Context, trace Trace) error
	Query(ctx context.Context, filter QueryFilter) ([]Trace, error)
	List(ctx context.Context, opts ListOptions) ([]Trace, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) error
	TrimToCount(ctx context.Context, keep int) error
	UpsertAlertRule(ctx context.Context, rule AlertRule) (AlertRule, error)
	ListAlertRules(ctx context.Context) ([]AlertRule, error)
	DeleteAlertRule(ctx context.Context, id string) error
}
