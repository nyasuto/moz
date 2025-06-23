package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

// Job represents a work item for the process pool
type Job struct {
	ID        string
	Command   string
	Arguments []string
	ResultCh  chan JobResult
	Timestamp time.Time
}

// JobResult represents the result of a job
type JobResult struct {
	ID       string
	Success  bool
	Result   interface{}
	Error    error
	Duration time.Duration
	WorkerID int
}

// Worker represents a worker in the process pool
type Worker struct {
	ID     int
	store  *kvstore.KVStore
	jobCh  chan Job
	quitCh chan struct{}
	wg     *sync.WaitGroup
}

// ProcessPool manages a pool of workers for processing jobs
type ProcessPool struct {
	workers    []*Worker
	jobQueue   chan Job
	workerSize int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	running    bool
	stats      PoolStats
}

// PoolStats tracks pool performance statistics
type PoolStats struct {
	TotalJobs      int64         `json:"total_jobs"`
	CompletedJobs  int64         `json:"completed_jobs"`
	FailedJobs     int64         `json:"failed_jobs"`
	AverageLatency time.Duration `json:"average_latency"`
	TotalDuration  time.Duration `json:"total_duration"`
	ActiveWorkers  int           `json:"active_workers"`
	QueueLength    int           `json:"queue_length"`
}

// NewProcessPool creates a new process pool
func NewProcessPool(workerSize int, queueSize int, store *kvstore.KVStore) *ProcessPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &ProcessPool{
		workerSize: workerSize,
		jobQueue:   make(chan Job, queueSize),
		ctx:        ctx,
		cancel:     cancel,
		workers:    make([]*Worker, workerSize),
	}

	// Create workers
	for i := 0; i < workerSize; i++ {
		worker := &Worker{
			ID:     i,
			store:  store,
			jobCh:  make(chan Job),
			quitCh: make(chan struct{}),
			wg:     &pool.wg,
		}
		pool.workers[i] = worker
	}

	return pool
}

// Start starts the process pool
func (p *ProcessPool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("process pool already running")
	}

	// Start dispatcher
	p.wg.Add(1)
	go p.dispatcher()

	// Start workers
	for _, worker := range p.workers {
		p.wg.Add(1)
		go worker.start()
	}

	p.running = true
	p.stats.ActiveWorkers = p.workerSize

	return nil
}

// Stop stops the process pool
func (p *ProcessPool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	// Cancel context to stop dispatcher
	p.cancel()

	// Stop workers
	for _, worker := range p.workers {
		close(worker.quitCh)
	}

	// Wait for all goroutines to finish
	p.wg.Wait()

	p.running = false
	p.stats.ActiveWorkers = 0

	return nil
}

// SubmitJob submits a job to the pool
func (p *ProcessPool) SubmitJob(command string, args ...string) (*JobResult, error) {
	if !p.IsRunning() {
		return nil, fmt.Errorf("process pool not running")
	}

	job := Job{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		Command:   command,
		Arguments: args,
		ResultCh:  make(chan JobResult, 1),
		Timestamp: time.Now(),
	}

	select {
	case p.jobQueue <- job:
		// Job queued successfully
	case <-p.ctx.Done():
		return nil, fmt.Errorf("process pool shutting down")
	default:
		return nil, fmt.Errorf("job queue full")
	}

	// Wait for result
	select {
	case result := <-job.ResultCh:
		p.updateStats(&result)
		return &result, nil
	case <-p.ctx.Done():
		return nil, fmt.Errorf("process pool shutting down")
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("job timeout")
	}
}

// SubmitJobAsync submits a job asynchronously
func (p *ProcessPool) SubmitJobAsync(command string, args ...string) (chan JobResult, error) {
	if !p.IsRunning() {
		return nil, fmt.Errorf("process pool not running")
	}

	job := Job{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		Command:   command,
		Arguments: args,
		ResultCh:  make(chan JobResult, 1),
		Timestamp: time.Now(),
	}

	select {
	case p.jobQueue <- job:
		return job.ResultCh, nil
	case <-p.ctx.Done():
		return nil, fmt.Errorf("process pool shutting down")
	default:
		return nil, fmt.Errorf("job queue full")
	}
}

// IsRunning checks if the pool is running
func (p *ProcessPool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetStats returns pool statistics
func (p *ProcessPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := p.stats
	stats.QueueLength = len(p.jobQueue)
	return stats
}

// dispatcher distributes jobs to workers
func (p *ProcessPool) dispatcher() {
	defer p.wg.Done()

	for {
		select {
		case job := <-p.jobQueue:
			// Find available worker
			select {
			case p.workers[0].jobCh <- job:
			case p.workers[1%len(p.workers)].jobCh <- job:
			case p.workers[2%len(p.workers)].jobCh <- job:
			default:
				// Round-robin assignment
				workerIndex := int(p.stats.TotalJobs) % len(p.workers)
				p.workers[workerIndex].jobCh <- job
			}

			p.stats.TotalJobs++

		case <-p.ctx.Done():
			return
		}
	}
}

// updateStats updates pool statistics
func (p *ProcessPool) updateStats(result *JobResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if result.Success {
		p.stats.CompletedJobs++
	} else {
		p.stats.FailedJobs++
	}

	p.stats.TotalDuration += result.Duration

	if p.stats.CompletedJobs > 0 {
		p.stats.AverageLatency = p.stats.TotalDuration / time.Duration(p.stats.CompletedJobs)
	}
}

// start starts a worker
func (w *Worker) start() {
	defer w.wg.Done()

	for {
		select {
		case job := <-w.jobCh:
			result := w.processJob(job)
			job.ResultCh <- result

		case <-w.quitCh:
			return
		}
	}
}

// processJob processes a single job
func (w *Worker) processJob(job Job) JobResult {
	start := time.Now()

	result := JobResult{
		ID:       job.ID,
		WorkerID: w.ID,
	}

	switch job.Command {
	case "put":
		if len(job.Arguments) != 2 {
			result.Success = false
			result.Error = fmt.Errorf("put requires exactly 2 arguments")
		} else {
			err := w.store.Put(job.Arguments[0], job.Arguments[1])
			if err != nil {
				result.Success = false
				result.Error = err
			} else {
				result.Success = true
				result.Result = "OK"
			}
		}

	case "get":
		if len(job.Arguments) != 1 {
			result.Success = false
			result.Error = fmt.Errorf("get requires exactly 1 argument")
		} else {
			value, err := w.store.Get(job.Arguments[0])
			if err != nil {
				result.Success = false
				result.Error = err
			} else {
				result.Success = true
				result.Result = value
			}
		}

	case "delete":
		if len(job.Arguments) != 1 {
			result.Success = false
			result.Error = fmt.Errorf("delete requires exactly 1 argument")
		} else {
			err := w.store.Delete(job.Arguments[0])
			if err != nil {
				result.Success = false
				result.Error = err
			} else {
				result.Success = true
				result.Result = "OK"
			}
		}

	case "list":
		entries, err := w.store.List()
		if err != nil {
			result.Success = false
			result.Error = err
		} else {
			result.Success = true
			result.Result = entries
		}

	case "compact":
		err := w.store.Compact()
		if err != nil {
			result.Success = false
			result.Error = err
		} else {
			result.Success = true
			result.Result = "Compaction completed"
		}

	case "stats":
		stats, err := w.store.GetCompactionStats()
		if err != nil {
			result.Success = false
			result.Error = err
		} else {
			result.Success = true
			result.Result = stats
		}

	default:
		result.Success = false
		result.Error = fmt.Errorf("unknown command: %s", job.Command)
	}

	result.Duration = time.Since(start)
	return result
}
