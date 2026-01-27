// Package research provides data structures and services for research infrastructure.
// Story 15.6: Feature Calculation Background Job - Tests
package research

import (
	"context"
	"testing"
	"time"
)

func TestDefaultFeatureJobConfig(t *testing.T) {
	config := DefaultFeatureJobConfig()

	if config.RecalculationInterval != 30*time.Minute {
		t.Errorf("Expected RecalculationInterval of 30m, got %v", config.RecalculationInterval)
	}

	if config.BatchSize != 500 {
		t.Errorf("Expected BatchSize of 500, got %d", config.BatchSize)
	}

	if config.MaxConcurrentJobs != 2 {
		t.Errorf("Expected MaxConcurrentJobs of 2, got %d", config.MaxConcurrentJobs)
	}

	if config.LookbackCandles != RecommendedLookback {
		t.Errorf("Expected LookbackCandles of %d, got %d", RecommendedLookback, config.LookbackCandles)
	}
}

func TestNewFeatureJobService(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	if service.config.BatchSize != 500 {
		t.Errorf("Expected BatchSize of 500, got %d", service.config.BatchSize)
	}

	if service.running {
		t.Error("Expected service to not be running initially")
	}
}

func TestFeatureJobServiceStartStop(t *testing.T) {
	config := DefaultFeatureJobConfig()
	config.RecalculationInterval = 100 * time.Millisecond // Short interval for testing
	service := NewFeatureJobService(nil, nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the service
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !service.IsRunning() {
		t.Error("Expected service to be running after Start()")
	}

	// Starting again should be a no-op
	err = service.Start(ctx)
	if err != nil {
		t.Fatalf("Second Start() should not fail: %v", err)
	}

	// Stop the service
	service.Stop()

	// Give a moment for goroutine to stop
	time.Sleep(50 * time.Millisecond)

	if service.IsRunning() {
		t.Error("Expected service to not be running after Stop()")
	}
}

func TestFeatureJobServiceTrackPair(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	// Track a pair
	service.TrackPair("BTCUSDT", Timeframe15m)

	pairs := service.GetTrackedPairs()
	if len(pairs) != 1 {
		t.Errorf("Expected 1 tracked pair, got %d", len(pairs))
	}

	if pairs[0] != "BTCUSDT:15m" {
		t.Errorf("Expected 'BTCUSDT:15m', got '%s'", pairs[0])
	}

	// Track another pair
	service.TrackPair("ETHUSDT", Timeframe5m)

	pairs = service.GetTrackedPairs()
	if len(pairs) != 2 {
		t.Errorf("Expected 2 tracked pairs, got %d", len(pairs))
	}

	// Untrack a pair
	service.UntrackPair("BTCUSDT", Timeframe15m)

	pairs = service.GetTrackedPairs()
	if len(pairs) != 1 {
		t.Errorf("Expected 1 tracked pair after untrack, got %d", len(pairs))
	}
}

func TestParsePairKey(t *testing.T) {
	tests := []struct {
		key       string
		symbol    string
		timeframe Timeframe
	}{
		{"BTCUSDT:15m", "BTCUSDT", Timeframe15m},
		{"ETHUSDT:5m", "ETHUSDT", Timeframe5m},
		{"MATICUSDT:1h", "MATICUSDT", Timeframe1h},
		{"invalidkey", "", ""},
	}

	for _, tt := range tests {
		symbol, timeframe := parsePairKey(tt.key)
		if symbol != tt.symbol {
			t.Errorf("parsePairKey(%s): expected symbol %s, got %s", tt.key, tt.symbol, symbol)
		}
		if timeframe != tt.timeframe {
			t.Errorf("parsePairKey(%s): expected timeframe %s, got %s", tt.key, tt.timeframe, timeframe)
		}
	}
}

func TestFeatureJobServiceGetStats(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	// Track some pairs
	service.TrackPair("BTCUSDT", Timeframe15m)
	service.TrackPair("ETHUSDT", Timeframe5m)

	stats := service.GetStats()

	if stats.Running {
		t.Error("Expected service to not be running")
	}

	if stats.TrackedPairsCount != 2 {
		t.Errorf("Expected 2 tracked pairs, got %d", stats.TrackedPairsCount)
	}

	if stats.BatchSize != 500 {
		t.Errorf("Expected BatchSize of 500, got %d", stats.BatchSize)
	}

	if stats.MaxConcurrentJobs != 2 {
		t.Errorf("Expected MaxConcurrentJobs of 2, got %d", stats.MaxConcurrentJobs)
	}
}

func TestFeatureJobServiceGenerateJobID(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	jobID1 := service.generateJobID("BTCUSDT", Timeframe15m)
	jobID2 := service.generateJobID("BTCUSDT", Timeframe15m)

	// Job IDs should be unique
	if jobID1 == jobID2 {
		t.Error("Expected unique job IDs")
	}

	// Job ID should contain symbol and timeframe
	if len(jobID1) < 10 {
		t.Error("Job ID seems too short")
	}
}

func TestFeatureJobCleanupOldJobs(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	// Add some completed jobs manually
	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)
	recentTime := now.Add(-30 * time.Minute)

	service.jobs["old_job"] = &FeatureJob{
		ID:          "old_job",
		Status:      FeatureJobCompleted,
		CompletedAt: &oldTime,
	}

	service.jobs["recent_job"] = &FeatureJob{
		ID:          "recent_job",
		Status:      FeatureJobCompleted,
		CompletedAt: &recentTime,
	}

	service.jobs["running_job"] = &FeatureJob{
		ID:     "running_job",
		Status: FeatureJobRunning,
	}

	// Cleanup jobs older than 1 hour
	removed := service.CleanupOldJobs(1 * time.Hour)

	if removed != 1 {
		t.Errorf("Expected 1 job removed, got %d", removed)
	}

	// Check that the right jobs remain
	if service.jobs["old_job"] != nil {
		t.Error("Expected old_job to be removed")
	}

	if service.jobs["recent_job"] == nil {
		t.Error("Expected recent_job to remain")
	}

	if service.jobs["running_job"] == nil {
		t.Error("Expected running_job to remain")
	}
}

func TestFeatureJobServiceListJobs(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	// Add some jobs manually
	service.jobs["job1"] = &FeatureJob{
		ID:     "job1",
		Status: FeatureJobCompleted,
	}

	service.jobs["job2"] = &FeatureJob{
		ID:     "job2",
		Status: FeatureJobRunning,
	}

	jobs := service.ListJobs()

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

func TestFeatureJobServiceGetJob(t *testing.T) {
	config := DefaultFeatureJobConfig()
	service := NewFeatureJobService(nil, nil, config)

	// Add a job manually
	service.jobs["test_job"] = &FeatureJob{
		ID:     "test_job",
		Status: FeatureJobCompleted,
	}

	job := service.GetJob("test_job")
	if job == nil {
		t.Error("Expected to find job")
	}

	if job.ID != "test_job" {
		t.Errorf("Expected job ID 'test_job', got '%s'", job.ID)
	}

	// Non-existent job
	job = service.GetJob("non_existent")
	if job != nil {
		t.Error("Expected nil for non-existent job")
	}
}
