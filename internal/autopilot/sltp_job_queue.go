package autopilot

import (
	"fmt"
	"sync"
	"time"
)

// SLTPJobStatus represents the current state of a SLTP recalculation job
type SLTPJobStatus string

const (
	JobStatusPending   SLTPJobStatus = "pending"
	JobStatusRunning   SLTPJobStatus = "running"
	JobStatusCompleted SLTPJobStatus = "completed"
	JobStatusFailed    SLTPJobStatus = "failed"
)

// SLTPJob represents an async SLTP recalculation job
type SLTPJob struct {
	ID                string                   `json:"id"`
	Status            SLTPJobStatus            `json:"status"`
	CreatedAt         time.Time                `json:"created_at"`
	StartedAt         *time.Time               `json:"started_at,omitempty"`
	CompletedAt       *time.Time               `json:"completed_at,omitempty"`
	TotalPositions    int                      `json:"total_positions"`
	ProcessedCount    int                      `json:"processed_count"`
	SuccessCount      int                      `json:"success_count"`
	FailedCount       int                      `json:"failed_count"`
	ProgressPercent   float64                  `json:"progress_percent"`
	CurrentPosition   string                  `json:"current_position,omitempty"`
	Error             string                  `json:"error,omitempty"`
	Results           []*GiniePosition         `json:"results,omitempty"`
	ElapsedSeconds    float64                  `json:"elapsed_seconds"`
	EstimatedSecondsRemaining int             `json:"estimated_seconds_remaining"`
}

// SLTPJobQueue manages async SLTP recalculation jobs
type SLTPJobQueue struct {
	mu            sync.RWMutex
	jobs          map[string]*SLTPJob
	maxJobs       int // Keep last N jobs in memory
	jobIDCounter  int
}

// NewSLTPJobQueue creates a new job queue
func NewSLTPJobQueue(maxJobs int) *SLTPJobQueue {
	return &SLTPJobQueue{
		jobs:    make(map[string]*SLTPJob),
		maxJobs: maxJobs,
	}
}

// CreateJob creates a new SLTP recalculation job
func (q *SLTPJobQueue) CreateJob(totalPositions int) *SLTPJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.jobIDCounter++
	jobID := fmt.Sprintf("sltp_%d_%d", time.Now().Unix(), q.jobIDCounter)

	job := &SLTPJob{
		ID:             jobID,
		Status:         JobStatusPending,
		CreatedAt:      time.Now(),
		TotalPositions: totalPositions,
		ProcessedCount: 0,
		SuccessCount:   0,
		FailedCount:    0,
	}

	q.jobs[jobID] = job

	// Cleanup old jobs if we exceed max
	if len(q.jobs) > q.maxJobs {
		q.cleanupOldJobs()
	}

	return job
}

// GetJob retrieves a job by ID
func (q *SLTPJobQueue) GetJob(jobID string) *SLTPJob {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.jobs[jobID]
}

// UpdateJobProgress updates progress for a running job
func (q *SLTPJobQueue) UpdateJobProgress(jobID string, currentPos string, processed, success, failed int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return
	}

	job.CurrentPosition = currentPos
	job.ProcessedCount = processed
	job.SuccessCount = success
	job.FailedCount = failed

	if job.TotalPositions > 0 {
		job.ProgressPercent = float64(processed) / float64(job.TotalPositions) * 100

		// Estimate remaining time based on average time per position
		if processed > 0 && job.StartedAt != nil {
			elapsedSecs := time.Since(*job.StartedAt).Seconds()
			avgPerPosition := elapsedSecs / float64(processed)
			remaining := job.TotalPositions - processed
			job.EstimatedSecondsRemaining = int(avgPerPosition * float64(remaining))
		}
	}

	job.ElapsedSeconds = time.Since(job.CreatedAt).Seconds()
}

// StartJob marks a job as running
func (q *SLTPJobQueue) StartJob(jobID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return
	}

	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
}

// CompleteJob marks a job as completed with results
func (q *SLTPJobQueue) CompleteJob(jobID string, results []*GiniePosition) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return
	}

	job.Status = JobStatusCompleted
	job.Results = results
	job.ProgressPercent = 100
	now := time.Now()
	job.CompletedAt = &now
	job.ElapsedSeconds = now.Sub(job.CreatedAt).Seconds()
}

// FailJob marks a job as failed with an error
func (q *SLTPJobQueue) FailJob(jobID string, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return
	}

	job.Status = JobStatusFailed
	job.Error = errMsg
	now := time.Now()
	job.CompletedAt = &now
	job.ElapsedSeconds = now.Sub(job.CreatedAt).Seconds()
}

// GetRecentJobs returns the most recent jobs
func (q *SLTPJobQueue) GetRecentJobs(limit int) []*SLTPJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var jobs []*SLTPJob
	for _, job := range q.jobs {
		jobs = append(jobs, job)
	}

	// Sort by creation time (newest first)
	for i := 0; i < len(jobs)-1; i++ {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[j].CreatedAt.After(jobs[i].CreatedAt) {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}

	if limit > 0 && len(jobs) > limit {
		return jobs[:limit]
	}
	return jobs
}

// cleanupOldJobs removes old completed/failed jobs to stay under maxJobs limit
func (q *SLTPJobQueue) cleanupOldJobs() {
	if len(q.jobs) <= q.maxJobs {
		return
	}

	// Find oldest completed/failed job and remove it
	var oldestKey string
	var oldestTime time.Time

	for key, job := range q.jobs {
		// Skip pending/running jobs
		if job.Status == JobStatusPending || job.Status == JobStatusRunning {
			continue
		}

		if oldestTime.IsZero() || job.CreatedAt.Before(oldestTime) {
			oldestTime = job.CreatedAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(q.jobs, oldestKey)
	}
}

// GetActiveJobs returns jobs that are currently running
func (q *SLTPJobQueue) GetActiveJobs() []*SLTPJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var active []*SLTPJob
	for _, job := range q.jobs {
		if job.Status == JobStatusRunning || job.Status == JobStatusPending {
			active = append(active, job)
		}
	}
	return active
}
