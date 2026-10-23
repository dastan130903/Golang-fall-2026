package main

import "fmt"

func main() {
	service := NewTaskService(TextAnalyzer{})
	for _, input := range []string{"Hello Go worldвыааааааав", "Go is fun", "Привет мир"} {
		task, err := service.Create(input)
		if err != nil {
			fmt.Println("Create error:", err)
			continue
		}
		fmt.Printf("Created task %d: %s\n", task.ID, task.Status)
		task, err = service.Process(task.ID)
		if err != nil {
			fmt.Println("Process error:", err)
			continue
		}
		fmt.Printf("Completed task %d: words=%d, characters=%d, status=%s\n",
			task.ID, task.Result.Words, task.Result.Characters, task.Status)
	}
}
