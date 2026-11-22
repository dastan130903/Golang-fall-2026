# SIS Project - Checkpoint 2: Text Analyzer

## About the project

This is the second checkpoint of my SIS project in Go. I continued working on my Text Analyzer from Checkpoint 1 and added concurrent task processing.

The program takes text as input and counts the number of words and characters.

For example, for `Hello Go world`, the result is:
- Words: 3
- Characters: 14

In Checkpoint 1, tasks were processed one by one. In Checkpoint 2, I added a worker pool so multiple tasks can be processed at the same time.

## What I added in Checkpoint 2

- A worker pool with 3 workers.
- A buffered channel with a queue capacity of 5 tasks.
- A `Submit()` method to send tasks to the queue.
- `sync.RWMutex` to protect shared task data.
- Task status updates during processing.
- Error handling for a full queue and invalid tasks.
- A `Shutdown()` method to stop workers correctly.
- New tests for concurrent processing.

I also kept the original functions and tests from Checkpoint 1.

## How the program works

First, the program creates a task with some text and gives it a unique ID.

The new task starts with the `Pending` status.

When I call `Submit()`, the task is added to the queue. One of the available workers takes the task and processes it.

The worker counts the words and characters and saves the result.

The task statuses are:

`Pending -> Queued -> Processing -> Completed`

If processing fails, the task gets the `Failed` status.

The workers can process different tasks at the same time. The order in which tasks finish can be different each time.

## Worker Pool and Queue

I used 3 workers and a buffered channel with capacity 5.

The workers take tasks from the same queue. After finishing one task, a worker can take another task.

If the queue is full, the program returns `ErrQueueFull`. The rejected task stays in the `Pending` status, so it can be submitted later.

If a task is already queued, processing, or completed, the program does not allow submitting it again.

## Synchronization

Because multiple goroutines work with the same task storage, I used `sync.RWMutex`.

It protects the task map, task IDs, statuses, and results.

I used `RLock()` for reading data and `Lock()` for changing data.

The actual text processing happens outside the lock, so workers can process different tasks at the same time.

I also used a mutex in the worker pool to make task submission and shutdown safe.

## Shutdown

I added `Shutdown()` to stop the worker pool.

When shutdown starts, the program stops accepting new tasks. Workers finish the tasks that were already accepted, and then they stop.

The program waits until all workers finish.

After shutdown, calling `Submit()` returns `ErrStopped`.

## Project files

- `main.go` - runs the program and shows example results.
- `task.go` - contains tasks, statuses, text analysis, and task storage.
- `pool.go` - contains the worker pool, queue, and shutdown logic.
- `task_test.go` - original tests from Checkpoint 1.
- `pool_test.go` - new tests for Checkpoint 2.
- `go.mod` - Go module information.
- `DEFENCE-RU-EN.md` - notes for explaining the project.

## How to run

The project uses Go 1.27 and does not need any external libraries.

Open the project folder in a terminal and run:

```bash
go run .
```

To check the code and run the tests:

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

`gofmt -l .` should not show any files.

`go test ./...` runs the regular tests.

`go test -race ./...` checks for data races during testing.

## Testing

I kept the tests from Checkpoint 1 and added new tests for the worker pool.

The new tests check:

- Creating and starting workers.
- Submitting tasks to the queue.
- Processing multiple tasks concurrently.
- Correct results and task statuses.
- Submitting the same task more than once.
- Handling a full queue.
- Reading shared task data safely.
- Stopping workers and rejecting tasks after shutdown.
- Handling processing errors.

The tests also check that two tasks can be processed at the same time, not just one after another.

## Summary

In Checkpoint 1, I learned how to create tasks, store them in memory, and process text.

In Checkpoint 2, I learned how to use goroutines, channels, mutexes, and worker pools to process tasks concurrently.

The main goal was to make the program process multiple tasks safely while keeping the original text analyzer functionality.