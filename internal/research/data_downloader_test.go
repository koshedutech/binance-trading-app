package research

import (
	"testing"
	"time"
)

func TestDownloadRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     DownloadRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "missing symbol",
			req: DownloadRequest{
				Symbol:    "",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "symbol too short",
			req: DownloadRequest{
				Symbol:    "BTC",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "symbol with lowercase",
			req: DownloadRequest{
				Symbol:    "btcusdt",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "symbol with special characters",
			req: DownloadRequest{
				Symbol:    "BTC-USDT",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "invalid timeframe",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe("invalid"),
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "missing start time",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe15m,
				StartTime: time.Time{},
				EndTime:   time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "missing end time",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Time{},
			},
			wantErr: true,
		},
		{
			name: "end time before start time",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-1 * time.Hour),
				EndTime:   time.Now().Add(-24 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "end time in future",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe15m,
				StartTime: time.Now().Add(-24 * time.Hour),
				EndTime:   time.Now().Add(24 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadRequest_EstimateCandleCount(t *testing.T) {
	tests := []struct {
		name      string
		req       DownloadRequest
		wantCount int
	}{
		{
			name: "24 hours of 15m candles",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe15m,
				StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			},
			wantCount: 96, // 24 hours * 4 candles per hour
		},
		{
			name: "24 hours of 1h candles",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe1h,
				StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			},
			wantCount: 24,
		},
		{
			name: "1 hour of 1m candles",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe1m,
				StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
			},
			wantCount: 60,
		},
		{
			name: "7 days of 1d candles",
			req: DownloadRequest{
				Symbol:    "BTCUSDT",
				Timeframe: Timeframe1d,
				StartTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
			},
			wantCount: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCount := tt.req.EstimateCandleCount()
			if gotCount != tt.wantCount {
				t.Errorf("EstimateCandleCount() = %v, want %v", gotCount, tt.wantCount)
			}
		})
	}
}

func TestGenerateJobID(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	jobID := generateJobID("BTCUSDT", Timeframe15m, startTime, endTime)

	expected := "BTCUSDT_15m_1704067200_1704153600"
	if jobID != expected {
		t.Errorf("generateJobID() = %v, want %v", jobID, expected)
	}

	// Test that same parameters generate same ID (idempotent)
	jobID2 := generateJobID("BTCUSDT", Timeframe15m, startTime, endTime)
	if jobID != jobID2 {
		t.Errorf("generateJobID() not idempotent: %v != %v", jobID, jobID2)
	}

	// Test different parameters generate different IDs
	jobID3 := generateJobID("ETHUSDT", Timeframe15m, startTime, endTime)
	if jobID == jobID3 {
		t.Errorf("generateJobID() should be different for different symbols")
	}
}

func TestDefaultDownloaderConfig(t *testing.T) {
	config := DefaultDownloaderConfig()

	if config.BatchSize != 1000 {
		t.Errorf("BatchSize = %v, want 1000", config.BatchSize)
	}
	if config.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent = %v, want 3", config.MaxConcurrent)
	}
	if config.RequestDelay != 100*time.Millisecond {
		t.Errorf("RequestDelay = %v, want 100ms", config.RequestDelay)
	}
}

func TestDownloadJobStatus(t *testing.T) {
	// Test that status constants are correct
	if JobStatusPending != "pending" {
		t.Errorf("JobStatusPending = %v, want pending", JobStatusPending)
	}
	if JobStatusRunning != "running" {
		t.Errorf("JobStatusRunning = %v, want running", JobStatusRunning)
	}
	if JobStatusCompleted != "completed" {
		t.Errorf("JobStatusCompleted = %v, want completed", JobStatusCompleted)
	}
	if JobStatusFailed != "failed" {
		t.Errorf("JobStatusFailed = %v, want failed", JobStatusFailed)
	}
	if JobStatusCancelled != "cancelled" {
		t.Errorf("JobStatusCancelled = %v, want cancelled", JobStatusCancelled)
	}
	if JobStatusPaused != "paused" {
		t.Errorf("JobStatusPaused = %v, want paused", JobStatusPaused)
	}
}

func TestNewDataDownloader(t *testing.T) {
	tests := []struct {
		name           string
		config         DataDownloaderConfig
		wantBatchSize  int
		wantDelay      time.Duration
		wantConcurrent int
	}{
		{
			name:           "default config",
			config:         DefaultDownloaderConfig(),
			wantBatchSize:  1000,
			wantDelay:      100 * time.Millisecond,
			wantConcurrent: 3,
		},
		{
			name: "zero batch size gets corrected",
			config: DataDownloaderConfig{
				BatchSize:     0,
				RequestDelay:  50 * time.Millisecond,
				MaxConcurrent: 2,
			},
			wantBatchSize:  1000,
			wantDelay:      50 * time.Millisecond,
			wantConcurrent: 2,
		},
		{
			name: "batch size over 1000 gets corrected",
			config: DataDownloaderConfig{
				BatchSize:     2000,
				RequestDelay:  50 * time.Millisecond,
				MaxConcurrent: 2,
			},
			wantBatchSize:  1000,
			wantDelay:      50 * time.Millisecond,
			wantConcurrent: 2,
		},
		{
			name: "zero delay gets corrected",
			config: DataDownloaderConfig{
				BatchSize:     500,
				RequestDelay:  0,
				MaxConcurrent: 2,
			},
			wantBatchSize:  500,
			wantDelay:      100 * time.Millisecond,
			wantConcurrent: 2,
		},
		{
			name: "zero concurrent gets corrected",
			config: DataDownloaderConfig{
				BatchSize:     500,
				RequestDelay:  50 * time.Millisecond,
				MaxConcurrent: 0,
			},
			wantBatchSize:  500,
			wantDelay:      50 * time.Millisecond,
			wantConcurrent: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create downloader without pool (nil is ok for config tests)
			d := NewDataDownloader(nil, nil, tt.config)

			if d.batchSize != tt.wantBatchSize {
				t.Errorf("batchSize = %v, want %v", d.batchSize, tt.wantBatchSize)
			}
			if d.requestDelay != tt.wantDelay {
				t.Errorf("requestDelay = %v, want %v", d.requestDelay, tt.wantDelay)
			}
			if d.maxConcurrent != tt.wantConcurrent {
				t.Errorf("maxConcurrent = %v, want %v", d.maxConcurrent, tt.wantConcurrent)
			}
			if cap(d.semaphore) != tt.wantConcurrent {
				t.Errorf("semaphore capacity = %v, want %v", cap(d.semaphore), tt.wantConcurrent)
			}
		})
	}
}

func TestDownloadProgress_PercentCalculation(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		downloaded  int
		wantPercent float64
	}{
		{
			name:        "zero total",
			total:       0,
			downloaded:  0,
			wantPercent: 0,
		},
		{
			name:        "half complete",
			total:       100,
			downloaded:  50,
			wantPercent: 50.0,
		},
		{
			name:        "fully complete",
			total:       100,
			downloaded:  100,
			wantPercent: 100.0,
		},
		{
			name:        "over 100 percent capped",
			total:       100,
			downloaded:  150,
			wantPercent: 100.0, // Should be capped at 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := DownloadProgress{
				TotalCandles:      tt.total,
				DownloadedCandles: tt.downloaded,
			}

			// Simulate the percent calculation logic from executeDownload
			var percent float64
			if progress.TotalCandles > 0 {
				percent = float64(progress.DownloadedCandles) / float64(progress.TotalCandles) * 100
				if percent > 100 {
					percent = 100
				}
			}

			if percent != tt.wantPercent {
				t.Errorf("PercentComplete = %v, want %v", percent, tt.wantPercent)
			}
		})
	}
}

func TestBulkInsertResult(t *testing.T) {
	result := BulkInsertResult{
		Inserted: 95,
		Updated:  3,
		Skipped:  0,
		Errors:   2,
		Duration: 100 * time.Millisecond,
	}

	// Test that we can calculate error rate
	totalAttempted := result.Inserted + result.Updated + result.Skipped + result.Errors
	errorRate := float64(result.Errors) / float64(totalAttempted)

	if totalAttempted != 100 {
		t.Errorf("total attempted = %v, want 100", totalAttempted)
	}

	if errorRate != 0.02 {
		t.Errorf("error rate = %v, want 0.02", errorRate)
	}
}

func TestCancelJobNotRunning(t *testing.T) {
	d := NewDataDownloader(nil, nil, DefaultDownloaderConfig())

	err := d.CancelJob("nonexistent-job")
	if err == nil {
		t.Error("CancelJob() should return error for nonexistent job")
	}

	if err.Error() != "job not found or not running: nonexistent-job" {
		t.Errorf("unexpected error message: %v", err)
	}
}
