package kvstore

import (
	"sync"
	"time"
)

// BackgroundTask represents a task that can be executed by a background worker
type BackgroundTask func() error

// BackgroundWorker manages background tasks with configurable intervals
type BackgroundWorker struct {
	name     string
	task     BackgroundTask
	interval time.Duration

	// Control channels
	triggerCh chan struct{}
	stopCh    chan struct{}
	doneCh    chan struct{}

	// State
	mu      sync.RWMutex
	running bool
	stats   BackgroundWorkerStats

	// Wait group for graceful shutdown
	wg sync.WaitGroup
}

// BackgroundWorkerStats holds statistics about background worker operations
type BackgroundWorkerStats struct {
	TaskCount       uint64
	SuccessCount    uint64
	ErrorCount      uint64
	LastRunTime     time.Time
	LastRunDuration time.Duration
	TotalRunTime    time.Duration
	AverageRunTime  time.Duration
}

// NewBackgroundWorker creates a new background worker
func NewBackgroundWorker(name string, interval time.Duration, task BackgroundTask) *BackgroundWorker {
	return &BackgroundWorker{
		name:      name,
		task:      task,
		interval:  interval,
		triggerCh: make(chan struct{}, 1), // Buffered to avoid blocking
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start starts the background worker
func (bw *BackgroundWorker) Start() {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.running {
		return // Already running
	}

	bw.running = true
	bw.wg.Add(1)
	go bw.run()
}

// Stop stops the background worker gracefully
func (bw *BackgroundWorker) Stop() {
	bw.mu.Lock()
	if !bw.running {
		bw.mu.Unlock()
		return
	}
	bw.mu.Unlock()

	// Signal stop
	close(bw.stopCh)

	// Wait for worker to finish
	bw.wg.Wait()

	bw.mu.Lock()
	bw.running = false
	close(bw.doneCh)
	bw.mu.Unlock()
}

// TriggerNow triggers the worker to run immediately
func (bw *BackgroundWorker) TriggerNow() {
	select {
	case bw.triggerCh <- struct{}{}:
		// Trigger sent successfully
	default:
		// Trigger already pending, no need to send another
	}
}

// IsRunning returns true if the worker is currently running
func (bw *BackgroundWorker) IsRunning() bool {
	bw.mu.RLock()
	defer bw.mu.RUnlock()
	return bw.running
}

// GetStats returns current worker statistics
func (bw *BackgroundWorker) GetStats() BackgroundWorkerStats {
	bw.mu.RLock()
	defer bw.mu.RUnlock()

	stats := bw.stats
	if stats.TaskCount > 0 {
		stats.AverageRunTime = time.Duration(int64(stats.TotalRunTime) / int64(stats.TaskCount))
	}

	return stats
}

// run is the main worker loop
func (bw *BackgroundWorker) run() {
	defer bw.wg.Done()

	ticker := time.NewTicker(bw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-bw.stopCh:
			// Graceful shutdown
			return

		case <-bw.triggerCh:
			// Manual trigger
			bw.executeTask()

		case <-ticker.C:
			// Scheduled execution
			bw.executeTask()
		}
	}
}

// executeTask executes the background task and updates statistics
func (bw *BackgroundWorker) executeTask() {
	startTime := time.Now()

	// Execute the task
	err := bw.task()

	duration := time.Since(startTime)

	// Update statistics
	bw.mu.Lock()
	bw.stats.TaskCount++
	bw.stats.LastRunTime = startTime
	bw.stats.LastRunDuration = duration
	bw.stats.TotalRunTime += duration

	if err != nil {
		bw.stats.ErrorCount++
	} else {
		bw.stats.SuccessCount++
	}
	bw.mu.Unlock()
}

// BackgroundWorkerManager manages multiple background workers
type BackgroundWorkerManager struct {
	mu      sync.RWMutex
	workers map[string]*BackgroundWorker
}

// NewBackgroundWorkerManager creates a new worker manager
func NewBackgroundWorkerManager() *BackgroundWorkerManager {
	return &BackgroundWorkerManager{
		workers: make(map[string]*BackgroundWorker),
	}
}

// AddWorker adds a new worker to the manager
func (bwm *BackgroundWorkerManager) AddWorker(name string, worker *BackgroundWorker) {
	bwm.mu.Lock()
	defer bwm.mu.Unlock()
	bwm.workers[name] = worker
}

// RemoveWorker removes a worker from the manager
func (bwm *BackgroundWorkerManager) RemoveWorker(name string) {
	bwm.mu.Lock()
	defer bwm.mu.Unlock()

	if worker, exists := bwm.workers[name]; exists {
		worker.Stop()
		delete(bwm.workers, name)
	}
}

// StartAll starts all workers
func (bwm *BackgroundWorkerManager) StartAll() {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	for _, worker := range bwm.workers {
		worker.Start()
	}
}

// StopAll stops all workers gracefully
func (bwm *BackgroundWorkerManager) StopAll() {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	for _, worker := range bwm.workers {
		worker.Stop()
	}
}

// GetWorker returns a worker by name
func (bwm *BackgroundWorkerManager) GetWorker(name string) (*BackgroundWorker, bool) {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	worker, exists := bwm.workers[name]
	return worker, exists
}

// GetAllStats returns statistics for all workers
func (bwm *BackgroundWorkerManager) GetAllStats() map[string]BackgroundWorkerStats {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	stats := make(map[string]BackgroundWorkerStats)
	for name, worker := range bwm.workers {
		stats[name] = worker.GetStats()
	}

	return stats
}

// ListWorkers returns the names of all workers
func (bwm *BackgroundWorkerManager) ListWorkers() []string {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	names := make([]string, 0, len(bwm.workers))
	for name := range bwm.workers {
		names = append(names, name)
	}

	return names
}

// TriggerWorker triggers a specific worker to run now
func (bwm *BackgroundWorkerManager) TriggerWorker(name string) bool {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	if worker, exists := bwm.workers[name]; exists {
		worker.TriggerNow()
		return true
	}

	return false
}

// GetRunningCount returns the number of currently running workers
func (bwm *BackgroundWorkerManager) GetRunningCount() int {
	bwm.mu.RLock()
	defer bwm.mu.RUnlock()

	count := 0
	for _, worker := range bwm.workers {
		if worker.IsRunning() {
			count++
		}
	}

	return count
}
