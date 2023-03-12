# async-tasks-in-memory

  We construct the async tasks in memory in a process.
  Just try to PostTask easily. Dont support task serialization.

## Backgroud

  For async task project there are three modes.
  One is  worker pool mode, another is schedule mode, and the last is time wheel mode.

  We searched and compared many projects.
  At finally, we choose the two projects.

* [alitto/pond](https://github.com/alitto/pond)

  Minimalistic and High-performance goroutine worker pool written in Go

* [go-timewheel](https://github.com/rfyiamcool/go-timewheel)

  golang timewheel lib, similar to golang std timer


We thank the authors of these projects for their sharing and contributions.

## TimeWheel stragety

  We use three TimeWheel to construct our async delay and interval tasks.

```golang

var cron_timetick_config = []struct {
	duration time.Duration
	count    int
}{
	{time.Millisecond * 5, 200},
	{time.Second, 120},
	{time.Second * 120, 60 * 12},
}

```

So we use 5 millisecond for the minimum interval.

## Usage

### for async task in worker pool

```golang
package main

import (
	"fmt"
	"time"

	asynctask "github.com/simonwu-os/async-tasks-in-memory"
)

func TestAyncTask() *asynctask.AsyncTask {
	now := time.Now()
	callback := func() {
		fmt.Println("task", time.Since(now))
	}
	task := asynctask.PostTask(callback)
	return task
}

func main() {
   defer asynctask.WaitAndExit()
   TestAyncTask()
}

```

## delay task with time wheel

```golang
package main

import (
	"fmt"
	"time"

	asynctask "github.com/simonwu-os/async-tasks-in-memory"
)

func TestDelayTask() *asynctask.AsyncTask {
	now := time.Now()
	callback := func() {
		fmt.Println("task", time.Since(now))
	}
	task := asynctask.PostTask(callback, asynctask.DelayTask(10*time.Millisecond))
	return task
}

func main() {
	defer asynctask.WaitAndExit()
	task := TestDelayTask()
	task.WaitForFinished(100*time.Millisecond)
}
```

## Contribution & Support

Feel free to send a pull request if you consider there's something which can be improved. Also, please open up an issue if you run into a problem when using this library or just have a question.
