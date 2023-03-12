package asynctask

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond"
	"github.com/rfyiamcool/go-timewheel"
)

var (
	async_task_config AsyncTaskInMemoryConfig
)

type Callback = func()

type AsyncTaskInMemory interface {
	PostDelayTask(callback Callback, delay time.Duration) *AsyncTask

	PostIntervalTask(callback Callback, interval time.Duration) *AsyncTask

	PostAsyncTask(callback Callback) *AsyncTask

	GroupContext(ctx context.Context) (*pond.TaskGroupWithContext, context.Context)

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
	sync_data    sync.Mutex
}

func (self *AsyncTask) CanCancel() bool {
	return self.time_wheel != nil
}

func (self *AsyncTask) Finished() bool {
	return atomic.LoadInt32(&self.called_count) > 0
}

func (self *AsyncTask) replace_new_task(new_async_task *AsyncTask) {
	self.sync_data.Lock()
	defer self.sync_data.Unlock()
	self.time_wheel = new_async_task.time_wheel
	self.wheel_task = new_async_task.wheel_task
	self.called_count = new_async_task.called_count
}

func (self *AsyncTask) WaitForFinished(timeout time.Duration) {
	if self.Finished() {
		return
	}
	workersDone := make(chan struct{})
	ctx, cancel_func := context.WithCancel(context.Background())
	defer cancel_func()
	go func(ctx context.Context) {
	exit_for:
		for {
			if self.Finished() {
				workersDone <- struct{}{}
			}
			select {
			case <-ctx.Done():
				break exit_for
			default:
				Sleep(2 * time.Millisecond)
				break
			}
		}
	}(ctx)
	// Wait until either all workers have exited or the deadline is reached
	select {
	case <-workersDone:
		return
	case <-time.After(timeout):
		return
	}
}

func (self *AsyncTask) StopAndWait(deadline time.Duration) {
	if self.CanCancel() {
		self.sync_data.Lock()
		defer self.sync_data.Unlock()
		workersDone := make(chan struct{})
		go func() {
			self.time_wheel.Remove(self.wheel_task)
			workersDone <- struct{}{}
		}()
		// Wait until either all workers have exited or the deadline is reached
		select {
		case <-workersDone:
			return
		case <-time.After(deadline):
			return
		}
	}
}

func newAsyncTaskWithTimeWheel(
	time_wheel *timewheel.TimeWheel,
	wheel_task *timewheel.Task,
) *AsyncTask {
	data := &AsyncTask{
		time_wheel: time_wheel,
		wheel_task: wheel_task,
	}
	return data
}

func newAsyncTask() *AsyncTask {
	data := &AsyncTask{}
	return data
}

type innerAsyncTaskInMemory struct {
	cron_wheels []*timewheel.TimeWheel
	work_pool   *pond.WorkerPool
}

func (self *innerAsyncTaskInMemory) Init() {

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

func (self *innerAsyncTaskInMemory) PostDelayTask(
	callback Callback,
	delay time.Duration) *AsyncTask {

	wheel := self.findTimeWheel(delay)
	task := wheel.Add(delay, callback)
	return newAsyncTaskWithTimeWheel(wheel, task)
}

func (self *innerAsyncTaskInMemory) PostIntervalTask(
	callback Callback,
	interval time.Duration) *AsyncTask {
	wheel := self.findTimeWheel(interval)
	task := wheel.AddCron(interval, callback)
	return newAsyncTaskWithTimeWheel(wheel, task)
}

func (self *innerAsyncTaskInMemory) PostAsyncTask(callback Callback) *AsyncTask {
	self.work_pool.Submit(callback)
	return newAsyncTask()
}

func (self *innerAsyncTaskInMemory) GroupContext(ctx context.Context) (*pond.TaskGroupWithContext, context.Context) {
	return self.work_pool.GroupContext(ctx)
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
	interval        time.Duration
	start_delay     time.Duration
	normal_interval bool
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
		o.normal_interval = true
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

	if task_option.normal_interval {
		return GetAsyncTask().PostIntervalTask(callback, task_option.interval)
	}

	var async_task *AsyncTask = nil
	interval := task_option.interval

	task_callback := func() {
		callback()
		atomic.StoreInt32(&async_task.called_count, 1)

		if interval > 0 {
			new_task := GetAsyncTask().PostIntervalTask(callback, interval)
			async_task.replace_new_task(new_task)
		}
	}

	if task_option.start_delay == 0 {
		async_task = GetAsyncTask().PostAsyncTask(task_callback)
	} else {
		async_task = GetAsyncTask().PostDelayTask(task_callback, task_option.start_delay)
	}
	return async_task
}
