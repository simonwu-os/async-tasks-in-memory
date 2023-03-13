package main

import (
	"fmt"
	"time"

	asynctask "github.com/simonwu-os/async-tasks-in-memory"
)

func TestIntervalWithDelayTask() *asynctask.AsyncTask {
	now := time.Now()
	callback := func() {
		fmt.Println("task", time.Since(now))
	}
	task := asynctask.PostTask(callback,
		asynctask.IntervalWithDelayTask(4*time.Millisecond, 15*time.Millisecond))

	return task
}

func main() {
	defer asynctask.WaitAndExit()
	task := TestIntervalWithDelayTask()
	task.WaitForFinished(100 * time.Millisecond)
}
