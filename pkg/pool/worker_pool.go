package pool

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
)

type TaskFunc func(ctx context.Context) error

type Task struct {
	ID   string
	Exec TaskFunc
	Ctx  context.Context
}

type WorkerPool struct {
	numWorkers     int
	taskQueue      chan Task
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	activeWorkers  atomic.Int64
	processedCount atomic.Int64
}

var GlobalPool *WorkerPool

func NewWorkerPool(numWorkers, queueSize int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 16
	}
	if queueSize <= 0 {
		queueSize = 1000
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &WorkerPool{
		numWorkers: numWorkers,
		taskQueue:  make(chan Task, queueSize),
		ctx:        ctx,
		cancel:     cancel,
	}

	for i := 0; i < numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	GlobalPool = p
	return p
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			p.runTask(task)
		}
	}
}

func (p *WorkerPool) runTask(task Task) {
	p.activeWorkers.Add(1)
	defer func() {
		p.activeWorkers.Add(-1)
		p.processedCount.Add(1)
		if r := recover(); r != nil {
			log.Printf("worker recovered from panic in task %s: %v", task.ID, r)
		}
	}()

	taskCtx := task.Ctx
	if taskCtx == nil {
		taskCtx = p.ctx
	}

	if err := task.Exec(taskCtx); err != nil {
		log.Printf("task %s returned error: %v", task.ID, err)
	}
}

func (p *WorkerPool) Submit(task Task) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.taskQueue <- task:
		return true
	default:
		// Queue full: apply backpressure
		return false
	}
}

func (p *WorkerPool) SubmitFunc(id string, fn TaskFunc) bool {
	return p.Submit(Task{
		ID:   id,
		Exec: fn,
		Ctx:  p.ctx,
	})
}

func (p *WorkerPool) ActiveWorkers() int64 {
	return p.activeWorkers.Load()
}

func (p *WorkerPool) TotalProcessed() int64 {
	return p.processedCount.Load()
}

func (p *WorkerPool) QueueLength() int {
	return len(p.taskQueue)
}

func (p *WorkerPool) Shutdown() {
	p.cancel()
	p.wg.Wait()
}
