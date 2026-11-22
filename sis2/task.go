package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
)

// The domain and the synchronous Process method come from Checkpoint 1.
type Status string

const (
	Pending    Status = "pending"
	Queued     Status = "queued"
	Processing Status = "processing"
	Completed  Status = "completed"
	Failed     Status = "failed"
)

var (
	ErrInvalidInput = errors.New("text must not be empty")
	ErrNotFound     = errors.New("task not found")
	ErrInvalidState = errors.New("task must be pending to process")
)

type Result struct {
	Words      int
	Characters int // Unicode code points, including spaces
}

type Task struct {
	ID     int
	Input  string
	Status Status
	Result *Result
	Error  string // Only used for failed asynchronous processing.
}

type Processor interface {
	Process(input string) (Result, error)
}

type TextAnalyzer struct{}

func (TextAnalyzer) Process(input string) (Result, error) {
	if strings.TrimSpace(input) == "" {
		return Result{}, ErrInvalidInput
	}
	return Result{
		Words:      len(strings.Fields(input)),
		Characters: utf8.RuneCountInString(input),
	}, nil
}

type TaskService struct {
	mu        sync.RWMutex // protects the map, nextID, and every stored Task field
	tasks     map[int]*Task
	nextID    int
	processor Processor
}

func NewTaskService(processor Processor) *TaskService {
	return &TaskService{tasks: make(map[int]*Task), processor: processor}
}

func (s *TaskService) Create(input string) (Task, error) {
	if strings.TrimSpace(input) == "" {
		return Task{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	task := &Task{ID: s.nextID, Input: input, Status: Pending}
	s.tasks[task.ID] = task
	return copyTask(task), nil
}

func copyTask(task *Task) Task {
	snapshot := *task
	if task.Result != nil {
		result := *task.Result
		snapshot.Result = &result
	}
	return snapshot
}

func (s *TaskService) Get(id int) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	return copyTask(task), nil
}

// Process preserves Checkpoint 1's synchronous API and retry-on-error behavior.
// The slow processor call runs WITHOUT holding the shared-data lock.
func (s *TaskService) Process(id int) (Task, error) {
	s.mu.Lock()
	task, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return Task{}, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if task.Status != Pending {
		s.mu.Unlock()
		return Task{}, fmt.Errorf("%w: task %d is %s", ErrInvalidState, id, task.Status)
	}
	task.Status = Processing
	input := task.Input
	s.mu.Unlock()

	result, err := s.processor.Process(input)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		task.Status = Pending // existing CP1 behavior
		return Task{}, fmt.Errorf("processing task %d: %w", id, err)
	}
	task.Result = &result
	task.Status = Completed
	return copyTask(task), nil
}

// reserveQueued marks a pending task as queued before the pool submits its ID.
// The pool's own mutex serializes reserveQueued, enqueue, and shutdown.
func (s *TaskService) reserveQueued(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if task.Status != Pending {
		return fmt.Errorf("%w: task %d is %s", ErrInvalidState, id, task.Status)
	}
	task.Status = Queued
	return nil
}

func (s *TaskService) releaseQueued(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[id]; ok && task.Status == Queued {
		task.Status = Pending
	}
}

// processQueued is called only by workers after receiving an accepted ID.
func (s *TaskService) processQueued(id int) {
	s.mu.Lock()
	task, exists := s.tasks[id]
	if !exists || task.Status != Queued {
		s.mu.Unlock()
		return
	}
	task.Status = Processing
	input := task.Input
	s.mu.Unlock()

	result, err := s.processor.Process(input)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		task.Status = Failed
		task.Error = err.Error()
		return
	}
	task.Result = &result
	task.Error = ""
	task.Status = Completed
}
