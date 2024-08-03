package asynctask_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	asynctask "github.com/simonwu-os/async-tasks-in-memory"
)

var _ = Describe("AsyncTask,focus", func() {

	It("PostTask", func() {
		result := 0
		callback := func() {
			result += 100
		}
		task := asynctask.PostTask(callback)
		asynctask.Sleep(10 * time.Millisecond)
		expected := 100
		Expect(result).To(Equal(expected))
		{
			result := true
			expected := task.Finished()
			Expect(result).To(Equal(expected))
		}
	})

	It("PostInterval", func() {
		result := 0
		callback := func() {
			result += 10
		}
		task := asynctask.PostTask(callback, asynctask.IntervalTask(20*time.Millisecond))
		asynctask.Sleep(110 * time.Millisecond)
		task.StopAndWait(10 * time.Millisecond)
		expected := 50
		Expect(result).To(Equal(expected))
		asynctask.Sleep(100 * time.Millisecond)
		Expect(result).To(Equal(expected))
	})

	It("PostWithDelay", func() {
		result := 0
		now := time.Now()
		callback := func() {
			result += 10
			///fmt.Println("Callback at ", time.Since(now))
		}
		task := asynctask.PostTask(callback,
			asynctask.DelayTask(5*time.Millisecond),
		)
		task.WaitForFinished(1000 * time.Millisecond)

		expected := 10
		Expect(result).To(Equal(expected))
		{
			result := time.Since(now)
			expected := 12 * time.Millisecond
			Expect(result).To(BeNumerically("<", expected))
		}
	})

	It("PostIntervalWithDelay", func() {
		result := 0
		///now := time.Now()
		callback := func() {
			result += 10
			///fmt.Println("Callback at ", time.Since(now))
		}
		task := asynctask.PostTask(callback,
			asynctask.IntervalWithDelayTask(5*time.Millisecond, 20*time.Millisecond),
		)
		asynctask.Sleep(110 * time.Millisecond)
		task.StopAndWait(10 * time.Millisecond)
		expected := 60
		Expect(result).To(Equal(expected))
		asynctask.Sleep(100 * time.Millisecond)
		Expect(result).To(Equal(expected))
	})

	It("with group", func() {
		group_ctx, _ := asynctask.GetAsyncTask().GroupContext(context.Background())
		data := 0
		group_ctx.Submit(func() error {
			data += 10
			return nil
		})
		group_ctx.Submit(func() error {
			data += 1700
			return nil
		})
		group_ctx.Wait()

		result := data
		expected := 1710
		Expect(result).To(Equal(expected))
	})

	It("group with cancel", func() {
		ctx, cancel_func := context.WithTimeout(context.Background(), 10*time.Millisecond)
		group_ctx, _ := asynctask.GetAsyncTask().GroupContext(ctx)
		data := 0
		group_ctx.Submit(func() error {
			data += 10
			return nil
		})
		group_ctx.Submit(func() error {
			data += 1700
			return nil
		})
		cancel_func()
		asynctask.Sleep(2 * time.Millisecond)
		err := group_ctx.Wait()
		result := data
		expected := 0
		Expect(result).To(Equal(expected))

		{
			result := err.Error()
			expected := "context canceled"
			Expect(result).To(MatchRegexp(expected))
		}
	})

	It("Interval with Delay Time", func() {
		result := 0
		task := asynctask.PostTask(func() {
			result += 10
		}, asynctask.IntervalWithDelayTask(6*time.Millisecond, 15*time.Millisecond))

		task.WaitForFinished(130 * time.Millisecond)
		expected := 10
		Expect(result).To(Equal(expected))

		task.WaitForFinishedCount(100, 120*time.Millisecond)
		expected = 90
		Expect(result).To(Equal(expected))
		asynctask.Sleep(10 * time.Millisecond)
		err := task.StopAndWait(20 * time.Millisecond)
		Expect(err).To(BeNil())
		Expect(result).To(BeNumerically(">=", expected))
	})

	It("cancel async task", func() {
		result := 0
		task := asynctask.PostTask(func() {
			result += 10
		})
		err := task.StopAndWait(5 * time.Millisecond)
		Expect(err).To(BeNil())

		expected := 0
		Expect(result).To(Equal(expected))
		asynctask.Sleep(10 * time.Millisecond)
		Expect(result).To(Equal(expected))

		result = int(task.CalledTimes())
		Expect(result).To(Equal(expected))

	})
})
