package store

import (
	"context"
	"time"
)

type Trace struct {
	ID           string
	TraceID      string
	ServerName   string
	Method       string
	ParamsHash   string
	ResponseHash string
	LatencyMs    int64
	IsError      bool
	ErrorMessage string
	CreatedAt    time.Time
}

type QueryFilter struct {
	TraceID    string
	ServerName string
	Method     string
	IsError    *bool
	Limit      int
}

type ListOptions struct {
	Limit  int
	Offset int
}

type TraceStore interface {
	Insert(ctx context.Context, trace Trace) error
	Query(ctx context.Context, filter QueryFilter) ([]Trace, error)
	List(ctx context.Context, opts ListOptions) ([]Trace, error)
}
