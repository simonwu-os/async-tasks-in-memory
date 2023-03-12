# async-tasks-in-memory

  We construct the async tasks in memory in a process.
  Just try to PostTask easily. Dont support task serialization.

## Backgroud

  For async task project there are threw modes.
  One is  work pool mode, another is schedule mode, the last is time wheel mode.

  We searched and compared many projects.
  At finally, we choose the two projects.

  [alitto/pond](https://github.com/alitto/pond)
  Minimalistic and High-performance goroutine worker pool written in Go

  [golang timewheel similar to glang std timer](https://github.com/rfyiamcool/go-timewheel)
  golang timewheel lib, similar to golang std timer

  We thank the authors of these projects for their sharing and contributions.

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

## Contribution & Support

Feel free to send a pull request if you consider there's something which can be improved. Also, please open up an issue if you run into a problem when using this library or just have a question.
