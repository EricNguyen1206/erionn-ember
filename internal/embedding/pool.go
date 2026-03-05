package embedding

import (
	"context"
	"runtime"
)

// EmbedResult wraps an embedding result or error.
type EmbedResult struct {
	Vec []float32
	Err error
}

type embedJob struct {
	ctx      context.Context
	prompt   string
	respChan chan<- EmbedResult
}

// Pool manages N goroutine workers for non-blocking embedding.
// The fast path (xxhash hit) never blocks on this pool.
type Pool struct {
	jobChan chan embedJob
	done    chan struct{}
}

// NewPool creates N workers using the provided embedder.
// workers <= 0 defaults to runtime.NumCPU().
func NewPool(workers int, embedder Embedder) *Pool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	p := &Pool{
		jobChan: make(chan embedJob, 512),
		done:    make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		go p.worker(embedder)
	}
	return p
}

// Embed submits a job and blocks until the vector is ready or ctx is done.
func (p *Pool) Embed(ctx context.Context, prompt string) ([]float32, error) {
	ch := make(chan EmbedResult, 1)
	select {
	case p.jobChan <- embedJob{ctx: ctx, prompt: prompt, respChan: ch}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.done:
		return nil, context.Canceled
	}
	select {
	case r := <-ch:
		return r.Vec, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close shuts down all workers.
func (p *Pool) Close() { close(p.done) }

func (p *Pool) worker(embedder Embedder) {
	for {
		select {
		case job := <-p.jobChan:
			vec, err := embedder.Embed(job.ctx, job.prompt)
			select {
			case job.respChan <- EmbedResult{Vec: vec, Err: err}:
			case <-p.done:
				return
			}
		case <-p.done:
			return
		}
	}
}
