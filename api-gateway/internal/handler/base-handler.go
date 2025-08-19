package handler

import (
	"context"
	"sync"
	"time"
)

type ReqBatcherInterface[K any, V any] interface {
	GetBatch(context context.Context, params QueryParams) (*V, error)
	flush()
}

type ReqBatcher[K any, V any] struct {
	client      K
	batchWindow time.Duration
	mu          sync.Mutex
	pending     []*BatchRequest[V]
	timer       *time.Timer
}

type BatchRequest[V any] struct {
	ctx    context.Context
	params QueryParams
	resp   chan *V
	err    chan error
}

func NewReqBatcher[K any, V any](client K, batchWindow time.Duration) *ReqBatcher[K, V] {
	return &ReqBatcher[K, V]{
		client:      client,
		batchWindow: batchWindow,
		pending:     []*BatchRequest[V]{},
	}
}
