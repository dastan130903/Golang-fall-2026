package main

import "fmt"

func main() {
	service := NewTaskService(TextAnalyzer{})
	pool, err := NewWorkerPool(service, 3, 5)
	if err != nil {
		fmt.Println("Pool error:", err)
		return
	}

	var ids []int
	for _, input := range []string{"Hello Go world", "Go is fun", "Привет мир", "Channels connect workers", "Go text analyzer"} {
		task, createErr := service.Create(input)
		if createErr != nil {
			fmt.Println("Create error:", createErr)
			continue
		}
		ids = append(ids, task.ID)
		if submitErr := pool.Submit(task.ID); submitErr != nil {
			fmt.Printf("Submit task %d: %v\n", task.ID, submitErr)
		} else {
			fmt.Printf("Submitted task %d: %s\n", task.ID, task.Input)
		}
	}

	pool.Shutdown() // Process every accepted task before printing results.
	fmt.Println("\nResults after worker shutdown:")
	for _, id := range ids {
		task, getErr := service.Get(id)
		if getErr != nil {
			fmt.Println("Get error:", getErr)
			continue
		}
		if task.Result != nil {
			fmt.Printf("Task %d: %s | words=%d, characters=%d\n", id, task.Status, task.Result.Words, task.Result.Characters)
		} else {
			fmt.Printf("Task %d: %s | error=%s\n", id, task.Status, task.Error)
		}
	}
	fmt.Println("All workers stopped")
}
