/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package loadtester

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
)

type TaskRunnerInterface interface {
	Add(task Task)
	GetTotalExecs() uint64
	Start(interval time.Duration, stopCh <-chan struct{})
	Timeout() time.Duration
}

type TaskRunner struct {
	logger       logr.Logger
	timeout      time.Duration
	todoTasks    *sync.Map
	runningTasks *sync.Map
	totalExecs   uint64
}

func NewTaskRunner(logger logr.Logger, timeout time.Duration) *TaskRunner {
	return &TaskRunner{
		logger:       logger.WithName("task-runner"),
		todoTasks:    new(sync.Map),
		runningTasks: new(sync.Map),
		timeout:      timeout,
	}
}

func (tr *TaskRunner) Add(task Task) {
	tr.todoTasks.Store(task.Hash(), task)
}

func (tr *TaskRunner) GetTotalExecs() uint64 {
	return atomic.LoadUint64(&tr.totalExecs)
}

func (tr *TaskRunner) runAll() {
	tr.todoTasks.Range(func(key interface{}, value interface{}) bool {
		task := value.(Task)
		go func(t Task) {
			// remove task from the to do list
			tr.todoTasks.Delete(t.Hash())

			// check if task is already running, if not run the task's command
			if _, exists := tr.runningTasks.Load(t.Hash()); !exists {
				// save the task in the running list
				tr.runningTasks.Store(t.Hash(), t)

				// create timeout context
				ctx, cancel := context.WithTimeout(context.Background(), tr.timeout)
				defer cancel()

				// increment the total exec counter
				atomic.AddUint64(&tr.totalExecs, 1)

				tr.logger.Info(t.Canary(), "Canary command skipped %s is already running", t)

				// run task with the timeout context
				t.Run(ctx)

				// remove task from the running list
				tr.runningTasks.Delete(t.Hash())
			} else {
				tr.logger.Info(t.Canary(), "Canary command skipped %s is already running", t)
			}
		}(task)
		return true
	})
}

func (tr *TaskRunner) Start(interval time.Duration, stopCh <-chan struct{}) {
	tickChan := time.NewTicker(interval).C
	for {
		select {
		case <-tickChan:
			tr.runAll()
		case <-stopCh:
			tr.logger.Info("shutting down the task runner")
			return
		}
	}
}

func (tr *TaskRunner) Timeout() time.Duration {
	return tr.timeout
}
