package main

import (
	"errors"
	"sync"
)

var (
	ErrQueueFull  = errors.New("task queue is full")
	ErrStopped    = errors.New("worker pool has been stopped")
	ErrPoolConfig = errors.New("workers must be at least 2 and queue capacity must be positive")
)

// WorkerPool owns and closes the jobs channel. Queue items are task IDs.
// gate synchronizes Submit with Shutdown so no send can race with close.
type WorkerPool struct {
	service *TaskService
	jobs    chan int
	gate    sync.Mutex
	stopped bool
	wg      sync.WaitGroup
	done    chan struct{}
}

func NewWorkerPool(service *TaskService, workers, queueCapacity int) (*WorkerPool, error) {
	if service == nil || workers < 2 || queueCapacity < 1 {
		return nil, ErrPoolConfig
	}
	pool := &WorkerPool{
		service: service,
		jobs:    make(chan int, queueCapacity),
		done:    make(chan struct{}),
	}
	pool.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go pool.worker()
	}
	return pool, nil
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for id := range p.jobs {
		p.service.processQueued(id)
	}
}

// Submit does not wait for processing. When the bounded queue is full, it
// returns ErrQueueFull and restores the task's Pending state.
func (p *WorkerPool) Submit(id int) error {
	p.gate.Lock()
	defer p.gate.Unlock()
	if p.stopped {
		return ErrStopped
	}
	if err := p.service.reserveQueued(id); err != nil {
		return err
	}
	select {
	case p.jobs <- id:
		return nil
	default:
		p.service.releaseQueued(id)
		return ErrQueueFull
	}
}

// Shutdown drains all accepted jobs, waits for all workers, then returns.
// It is safe to call more than once, including concurrently with Submit.
func (p *WorkerPool) Shutdown() {
	p.gate.Lock()
	if !p.stopped {
		p.stopped = true
		close(p.jobs)
		go func() {
			p.wg.Wait()
			close(p.done)
		}()
	}
	p.gate.Unlock()
	<-p.done
}
