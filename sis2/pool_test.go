package main

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// controlledProcessor makes concurrency tests deterministic without sleeps.
type controlledProcessor struct {
	started chan string
	release chan struct{}
}

func (p *controlledProcessor) Process(input string) (Result, error) {
	p.started <- input
	<-p.release
	return TextAnalyzer{}.Process(input)
}

func waitFor[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for concurrent event")
		var zero T
		return zero
	}
}

func TestPoolConfigurationAndStartup(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	for _, tc := range [][2]int{{1, 2}, {2, 0}, {0, 1}} {
		if _, err := NewWorkerPool(s, tc[0], tc[1]); !errors.Is(err, ErrPoolConfig) {
			t.Fatalf("config %v: expected ErrPoolConfig, got %v", tc, err)
		}
	}
	pool, err := NewWorkerPool(s, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	pool.Shutdown()
	pool.Shutdown()
}

func TestConcurrentProcessingAndResults(t *testing.T) {
	processor := &controlledProcessor{started: make(chan string, 3), release: make(chan struct{})}
	s := NewTaskService(processor)
	pool, err := NewWorkerPool(s, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := s.Create("Hello Go world")
	second, _ := s.Create("Привет мир")
	if err := pool.Submit(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Submit(second.ID); err != nil {
		t.Fatal(err)
	}
	// Both processor calls must have started BEFORE either is released.
	// This proves real overlap, not just sequential completion.
	_ = waitFor(t, processor.started)
	_ = waitFor(t, processor.started)
	for _, id := range []int{first.ID, second.ID} {
		got, err := s.Get(id)
		if err != nil || got.Status != Processing {
			t.Fatalf("task %d must be processing, got %+v, err=%v", id, got, err)
		}
	}
	if err := pool.Submit(first.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("duplicate processing must fail: %v", err)
	}
	close(processor.release)
	pool.Shutdown()
	for _, tc := range []struct{ id, words, chars int }{{first.ID, 3, 14}, {second.ID, 2, 10}} {
		got, err := s.Get(tc.id)
		if err != nil || got.Status != Completed || got.Result == nil || got.Result.Words != tc.words || got.Result.Characters != tc.chars {
			t.Fatalf("task %d: unexpected result %+v, err=%v", tc.id, got, err)
		}
	}
}

func TestFullQueueRestoresPending(t *testing.T) {
	processor := &controlledProcessor{started: make(chan string, 2), release: make(chan struct{})}
	s := NewTaskService(processor)
	pool, _ := NewWorkerPool(s, 2, 1)
	for i := 0; i < 2; i++ {
		task, _ := s.Create(fmt.Sprintf("busy %d", i))
		if err := pool.Submit(task.ID); err != nil {
			t.Fatal(err)
		}
		waitFor(t, processor.started) // this worker is occupied
	}
	queued, _ := s.Create("in the queue")
	if err := pool.Submit(queued.ID); err != nil {
		t.Fatal(err)
	}
	full, _ := s.Create("should be rejected")
	if err := pool.Submit(full.ID); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected queue full, got %v", err)
	}
	got, _ := s.Get(full.ID)
	if got.Status != Pending {
		t.Fatalf("rejected submission state = %s", got.Status)
	}
	got, _ = s.Get(queued.ID)
	if got.Status != Queued {
		t.Fatalf("accepted queued state = %s", got.Status)
	}
	close(processor.release)
	pool.Shutdown()
	got, _ = s.Get(queued.ID)
	if got.Status != Completed {
		t.Fatalf("shutdown lost queued task: %s", got.Status)
	}
}

func TestInvalidSubmissionsAndShutdown(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	pool, _ := NewWorkerPool(s, 2, 5)
	if err := pool.Submit(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing ID: %v", err)
	}
	first, _ := s.Create("one two")
	if err := pool.Submit(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Submit(first.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("duplicate submit: %v", err)
	}
	pool.Shutdown()
	got, _ := s.Get(first.ID)
	if got.Status != Completed {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if err := pool.Submit(first.ID); !errors.Is(err, ErrStopped) {
		t.Fatalf("submit after stop: %v", err)
	}
	pool.Shutdown() // idempotent
}

func TestManyTasksAndConcurrentReads(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	pool, _ := NewWorkerPool(s, 4, 500)
	const n = 300
	ids := make([]int, 0, n)
	for i := 0; i < n; i++ {
		task, err := s.Create(fmt.Sprintf("task number %d", i))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}
	var reads sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		reads.Add(1)
		go func() {
			defer reads.Done()
			for _, id := range ids {
				if _, err := s.Get(id); err != nil {
					t.Errorf("Get(%d): %v", id, err)
				}
			}
		}()
	}
	for _, id := range ids {
		if err := pool.Submit(id); err != nil {
			t.Fatalf("Submit(%d): %v", id, err)
		}
	}
	pool.Shutdown()
	reads.Wait()
	for _, id := range ids {
		got, _ := s.Get(id)
		if got.Status != Completed || got.Result == nil || got.Result.Words != 3 {
			t.Fatalf("incorrect result for %d: %+v", id, got)
		}
	}
}

func TestSubmitShutdownRace(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	pool, _ := NewWorkerPool(s, 3, 1000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task, err := s.Create(fmt.Sprintf("text %d", i))
			if err != nil {
				t.Errorf("Create: %v", err)
				return
			}
			err = pool.Submit(task.ID)
			if err != nil && !errors.Is(err, ErrStopped) {
				t.Errorf("Submit: %v", err)
			}
		}(i)
	}
	// Concurrent shutdown must never lead to send-on-closed-channel panic.
	pool.Shutdown()
	wg.Wait()
	pool.Shutdown()
}

func TestAsyncFailureIsNotCompleted(t *testing.T) {
	s := NewTaskService(failingProcessor{}) // original Checkpoint 1 fake
	pool, _ := NewWorkerPool(s, 2, 2)
	task, _ := s.Create("valid text")
	if err := pool.Submit(task.ID); err != nil {
		t.Fatal(err)
	}
	pool.Shutdown()
	got, _ := s.Get(task.ID)
	if got.Status != Failed || got.Result != nil || got.Error == "" {
		t.Fatalf("failure was lost: %+v", got)
	}
	if err := pool.Submit(task.ID); !errors.Is(err, ErrStopped) {
		t.Fatalf("expected stopped error: %v", err)
	}
}

func TestSnapshotDoesNotExposeSharedResult(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	pool, _ := NewWorkerPool(s, 2, 2)
	task, _ := s.Create("hello world")
	if err := pool.Submit(task.ID); err != nil {
		t.Fatal(err)
	}
	pool.Shutdown()
	snapshot, _ := s.Get(task.ID)
	snapshot.Result.Words = 999
	stored, _ := s.Get(task.ID)
	if stored.Result.Words != 2 {
		t.Fatalf("mutable snapshot leaked into task: %+v", stored)
	}
}
