package main

import (
	"errors"
	"testing"
)

func TestCreateAndRetrieve(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	task, err := s.Create("Hello world")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID != 1 || task.Status != Pending || task.Result != nil {
		t.Fatalf("unexpected new task: %+v", task)
	}
	got, err := s.Get(task.ID)
	if err != nil || got.Input != "Hello world" {
		t.Fatalf("Get: task=%+v, err=%v", got, err)
	}
}

func TestInvalidInput(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	for _, input := range []string{"", "  ", "\n\t"} {
		t.Run("input="+input, func(t *testing.T) {
			if _, err := s.Create(input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
	task, err := s.Create("Valid")
	if err != nil || task.ID != 1 {
		t.Fatalf("invalid input consumed an ID: %+v, %v", task, err)
	}
}

func TestMissingTask(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	if _, err := s.Get(404); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get expected ErrNotFound, got %v", err)
	}
	if _, err := s.Process(404); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Process expected ErrNotFound, got %v", err)
	}
}

func TestProcessingResults(t *testing.T) {
	cases := []struct {
		name, input  string
		words, chars int
	}{
		{"english", "Hello Go world", 3, 14},
		{"spaces", " two   words ", 2, 13},
		{"unicode", "Привет мир", 2, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewTaskService(TextAnalyzer{})
			task, err := s.Create(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			task, err = s.Process(task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if task.Status != Completed || task.Result == nil || task.Result.Words != tc.words || task.Result.Characters != tc.chars {
				t.Fatalf("unexpected processed task: %+v", task)
			}
			stored, err := s.Get(task.ID)
			if err != nil || stored.Result == nil || *stored.Result != *task.Result {
				t.Fatalf("result not stored: %+v, %v", stored, err)
			}
			if _, err := s.Process(task.ID); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("processing twice should fail: %v", err)
			}
		})
	}
}

func TestMultipleTasksAndSnapshot(t *testing.T) {
	s := NewTaskService(TextAnalyzer{})
	first, _ := s.Create("One")
	second, _ := s.Create("Two words")
	if first.ID == second.ID {
		t.Fatal("IDs are not unique")
	}
	completed, err := s.Process(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Get(second.ID)
	if err != nil || other.Status != Pending || other.Result != nil || other.Input != "Two words" {
		t.Fatalf("second task changed: %+v, %v", other, err)
	}
	completed.Result.Words = 999
	original, _ := s.Get(first.ID)
	if original.Result.Words != 1 {
		t.Fatal("stored result was mutated through snapshot")
	}
	_, err = s.Process(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	original, _ = s.Get(first.ID)
	if original.Result.Words != 1 {
		t.Fatal("first result was overwritten")
	}
}

type failingProcessor struct{}

func (failingProcessor) Process(string) (Result, error) {
	return Result{}, errors.New("temporary failure")
}

func TestProcessorFailureRestoresPending(t *testing.T) {
	s := NewTaskService(failingProcessor{})
	task, _ := s.Create("Valid text")
	if _, err := s.Process(task.ID); err == nil {
		t.Fatal("expected processing error")
	}
	stored, _ := s.Get(task.ID)
	if stored.Status != Pending || stored.Result != nil {
		t.Fatalf("unexpected state after error: %+v", stored)
	}
}
