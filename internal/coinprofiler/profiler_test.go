package coinprofiler

import (
	"testing"
	"time"
)

func TestNewCoinProfiler(t *testing.T) {
	t.Run("creates with default config when nil", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)
		if cp == nil {
			t.Fatal("expected non-nil CoinProfiler")
		}
		if cp.config == nil {
			t.Error("expected config to be initialized")
		}
		if cp.config.WebSocketEndpoint != "wss://fstream.binance.com/ws" {
			t.Errorf("unexpected WebSocket endpoint: %s", cp.config.WebSocketEndpoint)
		}
	})

	t.Run("creates with custom config", func(t *testing.T) {
		customConfig := &CoinProfilerConfig{
			WebSocketEndpoint: "wss://custom.endpoint/ws",
			ReconnectDelay:    5 * time.Second,
		}
		cp := NewCoinProfiler(customConfig, nil)
		if cp.config.WebSocketEndpoint != "wss://custom.endpoint/ws" {
			t.Errorf("expected custom endpoint, got: %s", cp.config.WebSocketEndpoint)
		}
	})
}

func TestCoinProfiler_StartStop(t *testing.T) {
	t.Run("starts and stops successfully", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		// Should not be running initially
		if cp.IsRunning() {
			t.Error("expected profiler to not be running initially")
		}

		// Start should succeed
		err := cp.Start()
		if err != nil {
			t.Fatalf("failed to start: %v", err)
		}

		// Should be running after start
		if !cp.IsRunning() {
			t.Error("expected profiler to be running after start")
		}

		// Second start should fail
		err = cp.Start()
		if err == nil {
			t.Error("expected error when starting already running profiler")
		}

		// Stop should succeed
		err = cp.Stop()
		if err != nil {
			t.Fatalf("failed to stop: %v", err)
		}

		// Should not be running after stop
		if cp.IsRunning() {
			t.Error("expected profiler to not be running after stop")
		}

		// Second stop should fail
		err = cp.Stop()
		if err == nil {
			t.Error("expected error when stopping already stopped profiler")
		}
	})

	t.Run("can restart after stop", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		err := cp.Start()
		if err != nil {
			t.Fatalf("first start failed: %v", err)
		}

		err = cp.Stop()
		if err != nil {
			t.Fatalf("stop failed: %v", err)
		}

		// Should be able to start again
		err = cp.Start()
		if err != nil {
			t.Fatalf("second start failed: %v", err)
		}

		// Clean up
		cp.Stop()
	})
}

func TestCoinProfiler_Status(t *testing.T) {
	t.Run("returns correct status when not running", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		status := cp.GetStatus()
		if status.Running {
			t.Error("expected Running to be false")
		}
		if status.Connected {
			t.Error("expected Connected to be false")
		}
	})

	t.Run("returns correct status when running", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)
		cp.Start()
		defer cp.Stop()

		// Give it a moment to initialize
		time.Sleep(100 * time.Millisecond)

		status := cp.GetStatus()
		if !status.Running {
			t.Error("expected Running to be true")
		}
		if status.StartedAt.IsZero() {
			t.Error("expected StartedAt to be set")
		}
		if status.Uptime == "" {
			t.Error("expected Uptime to be calculated")
		}
	})
}

func TestCoinProfiler_Config(t *testing.T) {
	t.Run("get config returns copy", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)
		config1 := cp.GetConfig()
		config2 := cp.GetConfig()

		// Modify one config
		config1.DebugMode = true

		// Other config should be unchanged
		if config2.DebugMode {
			t.Error("expected config to be a copy, not reference")
		}
	})

	t.Run("set config updates configuration", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		newConfig := &CoinProfilerConfig{
			WebSocketEndpoint: "wss://new.endpoint/ws",
			DebugMode:         true,
		}
		cp.SetConfig(newConfig)

		config := cp.GetConfig()
		if config.WebSocketEndpoint != "wss://new.endpoint/ws" {
			t.Error("expected config to be updated")
		}
	})

	t.Run("set nil config is ignored", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)
		originalEndpoint := cp.config.WebSocketEndpoint

		cp.SetConfig(nil)

		if cp.config.WebSocketEndpoint != originalEndpoint {
			t.Error("expected nil config to be ignored")
		}
	})
}

func TestCoinProfiler_Subscriptions(t *testing.T) {
	t.Run("adds subscription", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		req := &SubscriptionRequest{
			Symbol:     "BTCUSDT",
			Timeframes: []string{"5m", "15m"},
			Source:     DataSourceStrategy,
		}

		err := cp.AddSubscription(req)
		if err != nil {
			t.Fatalf("failed to add subscription: %v", err)
		}

		subs := cp.GetSubscriptions()
		if len(subs) != 1 {
			t.Errorf("expected 1 subscription, got %d", len(subs))
		}
		if subs["BTCUSDT"] == nil {
			t.Error("expected BTCUSDT subscription")
		}
	})

	t.Run("merges timeframes for same symbol", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		req1 := &SubscriptionRequest{
			Symbol:     "BTCUSDT",
			Timeframes: []string{"5m"},
			Source:     DataSourceStrategy,
		}
		req2 := &SubscriptionRequest{
			Symbol:     "BTCUSDT",
			Timeframes: []string{"15m", "1h"},
			Source:     DataSourcePosition,
		}

		cp.AddSubscription(req1)
		cp.AddSubscription(req2)

		subs := cp.GetSubscriptions()
		if len(subs) != 1 {
			t.Errorf("expected 1 subscription after merge, got %d", len(subs))
		}

		btcSub := subs["BTCUSDT"]
		if btcSub == nil {
			t.Fatal("expected BTCUSDT subscription")
		}

		// Should have merged timeframes
		if len(btcSub.Timeframes) != 3 {
			t.Errorf("expected 3 timeframes, got %d", len(btcSub.Timeframes))
		}

		// Source should be "both"
		if btcSub.Source != DataSourceBoth {
			t.Errorf("expected source to be 'both', got %s", btcSub.Source)
		}
	})

	t.Run("removes subscription", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		req := &SubscriptionRequest{
			Symbol:     "BTCUSDT",
			Timeframes: []string{"5m"},
			Source:     DataSourceStrategy,
		}
		cp.AddSubscription(req)

		cp.RemoveSubscription("BTCUSDT")

		subs := cp.GetSubscriptions()
		if len(subs) != 0 {
			t.Errorf("expected 0 subscriptions after removal, got %d", len(subs))
		}
	})

	t.Run("rejects invalid subscription", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		// Nil request
		err := cp.AddSubscription(nil)
		if err == nil {
			t.Error("expected error for nil request")
		}

		// Empty symbol
		err = cp.AddSubscription(&SubscriptionRequest{Symbol: ""})
		if err == nil {
			t.Error("expected error for empty symbol")
		}
	})
}

func TestCoinProfiler_CoinData(t *testing.T) {
	t.Run("returns nil for non-existent symbol", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		data := cp.GetCoinData("BTCUSDT")
		if data != nil {
			t.Error("expected nil for non-existent symbol")
		}
	})

	t.Run("get all coin data returns empty map initially", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		allData := cp.GetAllCoinData()
		if allData == nil {
			t.Error("expected non-nil map")
		}
		if len(allData) != 0 {
			t.Errorf("expected empty map, got %d items", len(allData))
		}
	})

	t.Run("get coin data returns deep copy", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		// Manually add data for testing
		cp.mu.Lock()
		cp.coinData["BTCUSDT"] = &CoinData{
			Symbol:     "BTCUSDT",
			Price:      50000.0,
			Strategies: []string{"strategy1", "strategy2"},
			Timeframes: map[string]*TimeframeData{
				"5m": {
					Timeframe: "5m",
					Open:      49000.0,
					High:      51000.0,
					Low:       48000.0,
					Close:     50000.0,
				},
			},
		}
		cp.mu.Unlock()

		// Get the data
		data := cp.GetCoinData("BTCUSDT")
		if data == nil {
			t.Fatal("expected non-nil data")
		}

		// Modify the returned copy
		data.Price = 99999.0
		data.Strategies[0] = "modified"
		data.Timeframes["5m"].Close = 99999.0

		// Verify original is not modified
		cp.mu.RLock()
		original := cp.coinData["BTCUSDT"]
		cp.mu.RUnlock()

		if original.Price != 50000.0 {
			t.Errorf("original price was modified: got %f, want 50000.0", original.Price)
		}
		if original.Strategies[0] != "strategy1" {
			t.Errorf("original strategies was modified: got %s, want strategy1", original.Strategies[0])
		}
		if original.Timeframes["5m"].Close != 50000.0 {
			t.Errorf("original timeframe was modified: got %f, want 50000.0", original.Timeframes["5m"].Close)
		}
	})

	t.Run("get all coin data returns deep copies", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		// Manually add data for testing
		cp.mu.Lock()
		cp.coinData["ETHUSDT"] = &CoinData{
			Symbol:     "ETHUSDT",
			Price:      3000.0,
			Strategies: []string{"strat1"},
			Timeframes: map[string]*TimeframeData{
				"1h": {
					Timeframe: "1h",
					Close:     3000.0,
				},
			},
		}
		cp.mu.Unlock()

		// Get all data
		allData := cp.GetAllCoinData()
		if len(allData) != 1 {
			t.Fatalf("expected 1 item, got %d", len(allData))
		}

		// Modify the returned copy
		allData["ETHUSDT"].Price = 88888.0
		allData["ETHUSDT"].Timeframes["1h"].Close = 88888.0

		// Verify original is not modified
		cp.mu.RLock()
		original := cp.coinData["ETHUSDT"]
		cp.mu.RUnlock()

		if original.Price != 3000.0 {
			t.Errorf("original price was modified: got %f, want 3000.0", original.Price)
		}
		if original.Timeframes["1h"].Close != 3000.0 {
			t.Errorf("original timeframe was modified: got %f, want 3000.0", original.Timeframes["1h"].Close)
		}
	})
}

func TestCoinProfiler_GracefulShutdown(t *testing.T) {
	t.Run("stop completes within timeout", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)
		cp.Start()

		done := make(chan struct{})
		go func() {
			cp.Stop()
			close(done)
		}()

		select {
		case <-done:
			// Success - stop completed
		case <-time.After(15 * time.Second):
			t.Error("stop did not complete within timeout")
		}
	})
}

func TestDefaultCoinProfilerConfig(t *testing.T) {
	config := DefaultCoinProfilerConfig()

	t.Run("has valid WebSocket endpoint", func(t *testing.T) {
		if config.WebSocketEndpoint == "" {
			t.Error("expected non-empty WebSocket endpoint")
		}
		if config.WebSocketEndpoint != "wss://fstream.binance.com/ws" {
			t.Errorf("unexpected endpoint: %s", config.WebSocketEndpoint)
		}
	})

	t.Run("has reasonable reconnect settings", func(t *testing.T) {
		if config.ReconnectDelay <= 0 {
			t.Error("expected positive reconnect delay")
		}
		if config.MaxReconnectDelay < config.ReconnectDelay {
			t.Error("max reconnect delay should be >= reconnect delay")
		}
		if config.ReconnectBackoff <= 1 {
			t.Error("reconnect backoff should be > 1 for exponential backoff")
		}
	})

	t.Run("has reasonable subscription limits", func(t *testing.T) {
		if config.MaxSubscriptions <= 0 {
			t.Error("expected positive max subscriptions")
		}
		// Binance has a limit of 200 subscriptions per WebSocket connection
		if config.MaxSubscriptions > 200 {
			t.Error("max subscriptions should not exceed Binance limit of 200")
		}
	})

	t.Run("has delta updates enabled by default", func(t *testing.T) {
		if !config.EnableDeltaUpdates {
			t.Error("expected delta updates to be enabled by default")
		}
	})
}

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionStateConnecting, "connecting"},
		{ConnectionStateConnected, "connected"},
		{ConnectionStateReconnecting, "reconnecting"},
		{ConnectionState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m 1s"},
		{90061 * time.Second, "1d 1h 1m 1s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatUptime(tt.duration)
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCoinProfiler_ConcurrentStartStop(t *testing.T) {
	t.Run("concurrent stop calls do not panic", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)
		cp.Start()

		// Give goroutines time to start
		time.Sleep(50 * time.Millisecond)

		// Call Stop concurrently from multiple goroutines
		done := make(chan struct{})
		for i := 0; i < 10; i++ {
			go func() {
				cp.Stop() // Should not panic even if called multiple times
			}()
		}

		// Wait a bit and signal completion
		go func() {
			time.Sleep(200 * time.Millisecond)
			close(done)
		}()

		select {
		case <-done:
			// Success - no panic occurred
		case <-time.After(5 * time.Second):
			t.Error("concurrent stop test timed out")
		}
	})

	t.Run("rapid start-stop cycles do not panic", func(t *testing.T) {
		cp := NewCoinProfiler(nil, nil)

		for i := 0; i < 5; i++ {
			err := cp.Start()
			if err != nil && i > 0 {
				// Only first start should work if previous stop hasn't completed
				continue
			}
			time.Sleep(10 * time.Millisecond)
			cp.Stop()
			time.Sleep(10 * time.Millisecond)
		}
		// If we reach here without panic, test passes
	})
}
