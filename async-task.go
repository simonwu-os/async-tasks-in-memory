package asynctask

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond"
	"github.com/simonwu-os/go-timewheel/v2"
)

var (
	async_task_config AsyncTaskInMemoryConfig
)

type Callback = func()

type AsyncTaskInMemory interface {
	PostDelayTask(callback Callback, delay time.Duration) *AsyncTask

	PostIntervalTask(callback Callback, interval time.Duration) *AsyncTask

	PostAsyncTask(callback Callback) *AsyncTask

	///返回group,后面可以将group的异步消息做为一个整体，一起等待完成.
	/// group.Submit(task) ...., group.Wait()

	GroupContext(ctx context.Context) (*pond.TaskGroupWithContext, context.Context)

	/// 退出整个异步环境. 一般无需调用.
	WaitAndExit(timeout time.Duration)
}

const WHELL_10_MILLSECOND = 0
const WHEEL_SECOND = 1
const WHEEL_MINUTE = 2

var cron_timetick_config = []struct {
	duration time.Duration
	count    int
}{
	{time.Millisecond * 5, 200},
	{time.Second, 120},
	{time.Second * 120, 60 * 12},
}
var wheel_cron_count = len(cron_timetick_config)

type pond_workpool_config struct {
	max_workers  int
	max_capacity int
	pond_options []pond.Option
}

type AsyncTaskInMemoryConfig struct {
	single_ton       AsyncTaskInMemory
	init_once        sync.Once
	pond_config      *pond_workpool_config
	init_pond_config sync.Once
	need_wait_exit   bool
}

func (self *AsyncTaskInMemoryConfig) GetPondConfig() *pond_workpool_config {
	self.init_pond_config.Do(func() {
		self.pond_config = &pond_workpool_config{
			max_workers:  10,
			max_capacity: 10000,
			pond_options: []pond.Option{pond.MinWorkers(2)},
		}
	})
	return self.pond_config
}

func (self *AsyncTaskInMemoryConfig) freePondConfig() {
	self.pond_config = nil
}

type AsyncTask struct {
	time_wheel   *timewheel.TimeWheel
	wheel_task   *timewheel.Task
	called_count int32
	running      int32
}

func (self *AsyncTask) try_cancel_task() bool {
	return atomic.CompareAndSwapInt32(&self.running, 0, 2)
}

func (self *AsyncTask) task_is_cancelled() bool {
	return atomic.LoadInt32(&self.running) == 2
}

func (self *AsyncTask) on_task_done() {
	atomic.AddInt32(&self.called_count, 1)
	atomic.CompareAndSwapInt32(&self.running, 1, 0)
}

func (self *AsyncTask) on_task_start() bool {
	return atomic.CompareAndSwapInt32(&self.running, 0, 1)
}

func (self *AsyncTask) IsRunning() bool {
	return atomic.LoadInt32(&self.running) == 1
}

func (self *AsyncTask) Finished() bool {
	return self.CalledTimes() > 0
}

func (self *AsyncTask) CalledTimes() int32 {
	return atomic.LoadInt32(&self.called_count)
}

func (self *AsyncTask) can_cancel() bool {
	return self.time_wheel != nil
}

func (self *AsyncTask) cancel_task() {
	self.time_wheel.Remove(self.wheel_task)
	self.wheel_task = nil
	self.time_wheel = nil
}

func error_task_cancelled() error {
	return fmt.Errorf("task is cancelled")
}

func (self *AsyncTask) WaitForFinished(timeout time.Duration) error {

	if self.task_is_cancelled() {
		return error_task_cancelled()
	}
	if self.Finished() {
		return nil
	}

	workersDone := make(chan struct{})
	ctx, cancel_func := context.WithCancel(context.Background())
	defer cancel_func()
	go func(ctx context.Context) {
	exit_for:
		for {
			if self.Finished() || self.task_is_cancelled() {
				workersDone <- struct{}{}
				break exit_for
			}
			select {
			case <-ctx.Done():
				break exit_for
			default:
				Sleep(5 * time.Millisecond)
				break
			}
		}
	}(ctx)
	// Wait until either all workers have exited or the deadline is reached
	select {
	case <-workersDone:
		if self.Finished() {
			return nil
		}
		return error_task_cancelled()
	case <-time.After(timeout):
		return fmt.Errorf("time is up")
	}
}

func (self *AsyncTask) StopAndWait(deadline time.Duration) error {
	if deadline < time.Millisecond*10 {
		deadline = time.Millisecond * 10
	}
	for {
		if self.try_cancel_task() {
			break
		}
		select {
		case <-time.After(deadline):
			return fmt.Errorf("time is up")
		default:
			Sleep(5 * time.Millisecond)
		}
	}
	if self.can_cancel() {
		self.cancel_task()
	}
	return nil
}

type innerAsyncTaskInMemory struct {
	cron_wheels []*timewheel.TimeWheel
	work_pool   *pond.WorkerPool
}

func (self *innerAsyncTaskInMemory) findTimeWheel(delay time.Duration) *timewheel.TimeWheel {
	ret := self.cron_wheels[0]
	for index := wheel_cron_count - 1; index >= 0; index -= 1 {
		config := cron_timetick_config[index]
		if delay >= config.duration {
			ret = self.cron_wheels[index]
			break
		}
	}
	return ret
}

func (self *innerAsyncTaskInMemory) wrap_task_callback(callback Callback) (*AsyncTask, Callback) {
	ret := &AsyncTask{}
	wrap_callback := func() {
		///已经进入cancel状态，停止运行.
		if !ret.on_task_start() {
			return
		}
		defer ret.on_task_done()
		callback()
	}
	return ret, wrap_callback
}

func (self *innerAsyncTaskInMemory) PostDelayTask(
	callback Callback,
	delay time.Duration) *AsyncTask {

	ret, callback_wrapped := self.wrap_task_callback(callback)
	wheel := self.findTimeWheel(delay)
	task := wheel.Add(delay, callback_wrapped)

	ret.time_wheel = wheel
	ret.wheel_task = task
	return ret
}

func (self *innerAsyncTaskInMemory) PostIntervalTask(
	callback Callback,
	interval time.Duration) *AsyncTask {
	ret, callback_wrapped := self.wrap_task_callback(callback)

	wheel := self.findTimeWheel(interval)
	task := wheel.AddCron(interval, callback_wrapped)

	ret.time_wheel = wheel
	ret.wheel_task = task
	return ret
}

func (self *innerAsyncTaskInMemory) PostAsyncTask(callback Callback) *AsyncTask {
	ret, callback_wrapped := self.wrap_task_callback(callback)
	self.work_pool.Submit(callback_wrapped)
	return ret
}

///返回group,后面可以将group的异步消息做为一个整体，一起等待.

func (self *innerAsyncTaskInMemory) GroupContext(ctx context.Context) (*pond.TaskGroupWithContext, context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	group, new_ctx := self.work_pool.GroupContext(ctx)
	return group, new_ctx
}

func (self *innerAsyncTaskInMemory) WaitAndExit(timeout time.Duration) {
	for index := 0; index < wheel_cron_count; index += 1 {
		wheel := self.cron_wheels[index]
		wheel.Stop()
	}
	self.work_pool.StopAndWaitFor(timeout)
}

func newAsyncTaskInMemory(pond_config *pond_workpool_config) *innerAsyncTaskInMemory {
	data := innerAsyncTaskInMemory{}
	data.cron_wheels = make([]*timewheel.TimeWheel, wheel_cron_count)
	for index := 0; index < wheel_cron_count; index += 1 {
		config := cron_timetick_config[index]
		wheel, _ := timewheel.NewTimeWheel(config.duration, config.count)
		data.cron_wheels[index] = wheel
		wheel.Start()
	}
	data.work_pool = pond.New(pond_config.max_workers, pond_config.max_capacity, pond_config.pond_options...)
	return &data
}

func init() {
}

func SetWorkPoolOptions(maxWorkers, maxCapacity int, options ...pond.Option) {
	pond_config := async_task_config.GetPondConfig()
	if pond_config == nil {
		panic("It has already initialised pond.WorkPool.")
	}

	if maxWorkers > 0 {
		pond_config.max_workers = maxCapacity
	}

	if maxCapacity > 0 {
		pond_config.max_capacity = maxCapacity
	}
	if len(options) > 0 {
		pond_config.pond_options = append(pond_config.pond_options, options...)
	}
}

func WaitAndExit() {
	if !async_task_config.need_wait_exit {
		return
	}
	GetAsyncTask().WaitAndExit(1 * time.Second)
}

func GetAsyncTask() AsyncTaskInMemory {
	async_task_config.init_once.Do(func() {
		pond_config := async_task_config.GetPondConfig()
		async_task_config.single_ton = newAsyncTaskInMemory(pond_config)
		async_task_config.freePondConfig()
		async_task_config.need_wait_exit = true
	})
	return async_task_config.single_ton
}

type TaskOption struct {
	interval    time.Duration
	start_delay time.Duration
}
type optionTask func(*TaskOption)

func IntervalWithDelayTask(start_delay time.Duration, interal time.Duration) optionTask {
	return func(o *TaskOption) {
		o.interval = interal
		o.start_delay = start_delay
	}
}

func IntervalTask(interal time.Duration) optionTask {
	return func(o *TaskOption) {
		o.interval = interal
	}
}

func DelayTask(delay time.Duration) optionTask {
	return func(o *TaskOption) {
		o.start_delay = delay
	}
}

func Sleep(delay time.Duration) {
	time.Sleep(delay)
}

func PostTask(callback Callback, options ...optionTask) *AsyncTask {
	task_option := &TaskOption{}
	for _, op := range options {
		op(task_option)
	}

	if task_option.start_delay > 0 {
		if task_option.interval == 0 {
			return GetAsyncTask().PostDelayTask(callback, task_option.start_delay)
		} else {
			var async_task *AsyncTask
			wrap_callback := func() {
				callback()
				inner_task := GetAsyncTask().PostIntervalTask(callback, task_option.interval)
				async_task.time_wheel = inner_task.time_wheel
				async_task.wheel_task = inner_task.wheel_task
			}
			async_task = GetAsyncTask().PostAsyncTask(wrap_callback)
			return async_task
		}
	} else if task_option.interval > 0 {
		return GetAsyncTask().PostIntervalTask(callback, task_option.interval)
	} else {
		return GetAsyncTask().PostAsyncTask(callback)
	}
}
