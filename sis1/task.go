package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Status string

const (
	Pending    Status = "pending"
	Processing Status = "processing"
	Completed  Status = "completed"
)

var (
	ErrInvalidInput = errors.New("text must not be empty")
	ErrNotFound     = errors.New("task not found")
	ErrInvalidState = errors.New("task must be pending to process")
)

type Result struct {
	Words      int
	Characters int
}

type Task struct {
	ID     int
	Input  string
	Status Status
	Result *Result
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
	s.nextID++
	task := &Task{ID: s.nextID, Input: input, Status: Pending}
	s.tasks[task.ID] = task
	return *task, nil
}

func (s *TaskService) Get(id int) (Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	snapshot := *task
	if task.Result != nil {
		result := *task.Result
		snapshot.Result = &result
	}
	return snapshot, nil
}

func (s *TaskService) Process(id int) (Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if task.Status != Pending {
		return Task{}, fmt.Errorf("%w: task %d is %s", ErrInvalidState, id, task.Status)
	}
	task.Status = Processing
	result, err := s.processor.Process(task.Input)
	if err != nil {

		task.Status = Pending
		return Task{}, fmt.Errorf("processing task %d: %w", id, err)
	}
	task.Result = &result
	task.Status = Completed
	return s.Get(id)
}
