// Package database provides the EfficiencyMetricsRepository for trade efficiency data persistence.
// Epic 10: Exit Optimization Engine
// Story 10.1: Database Implementation for Trade Efficiency Metrics
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TradeEfficiencyMetric represents a trade efficiency record in the database.
// This maps to the trade_efficiency_metrics table and stores exit efficiency
// data for historical baseline calculation.
type TradeEfficiencyMetric struct {
	ID             int64     `json:"id"`
	FuturesTradeID *int64    `json:"futures_trade_id,omitempty"`
	UserID         string    `json:"user_id"`
	Symbol         string    `json:"symbol"`
	Mode           string    `json:"mode"`

	// Entry/Exit Data
	EntryPrice float64   `json:"entry_price"`
	ExitPrice  float64   `json:"exit_price"`
	EntryTime  time.Time `json:"entry_time"`
	ExitTime   time.Time `json:"exit_time"`

	// Quantity
	OriginalQty float64 `json:"original_qty"`
	ExitQty     float64 `json:"exit_qty"`

	// Efficiency Data
	PeakProfitPct  float64 `json:"peak_profit_pct"`
	ExitProfitPct  float64 `json:"exit_profit_pct"`
	ExitEfficiency float64 `json:"exit_efficiency"`

	// Exit Details
	ExitReason  string  `json:"exit_reason"`
	ExitUrgency *string `json:"exit_urgency,omitempty"`

	// Stage Data
	BreakevenAchieved bool       `json:"breakeven_achieved"`
	BreakevenTime     *time.Time `json:"breakeven_time,omitempty"`
	TP1Hit            bool       `json:"tp1_hit"`
	TP1Time           *time.Time `json:"tp1_time,omitempty"`
	TP1Qty            *float64   `json:"tp1_qty,omitempty"`
	TP1Profit         *float64   `json:"tp1_profit,omitempty"`

	// Decision Engine Data
	DecisionMode  string  `json:"decision_mode"`
	EntryStrategy *string `json:"entry_strategy,omitempty"`
	EntryRegime   *string `json:"entry_regime,omitempty"`
	ExitRegime    *string `json:"exit_regime,omitempty"`

	// Indicator Scores at Exit
	TrendScore      *float64 `json:"trend_score,omitempty"`
	MomentumScore   *float64 `json:"momentum_score,omitempty"`
	VolatilityScore *float64 `json:"volatility_score,omitempty"`
	VolumeScore     *float64 `json:"volume_score,omitempty"`

	// Classic Indicators at Exit
	ADXAtExit      *float64 `json:"adx_at_exit,omitempty"`
	RSIAtExit      *float64 `json:"rsi_at_exit,omitempty"`
	ATRPctAtExit   *float64 `json:"atr_pct_at_exit,omitempty"`
	TrendDirection *string  `json:"trend_direction,omitempty"`
	TrendStrength  *float64 `json:"trend_strength,omitempty"`

	// Category
	TradeCategory int `json:"trade_category"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// EfficiencyMetricsRepository defines the interface for efficiency metrics operations.
// This can be used for dependency injection and testing.
type EfficiencyMetricsRepository interface {
	CreateEfficiencyMetric(ctx context.Context, metric *TradeEfficiencyMetric) error
	GetEfficiencyMetricByTradeID(ctx context.Context, tradeID int64) (*TradeEfficiencyMetric, error)
	GetRecentEfficiencyMetricsByUserMode(ctx context.Context, userID, mode string, windowHours int) ([]TradeEfficiencyMetric, error)
	GetAverageExitEfficiency(ctx context.Context, userID, mode string, windowHours int) (float64, int, error)
	GetEfficiencyMetricsBySymbol(ctx context.Context, userID, symbol string, limit int) ([]TradeEfficiencyMetric, error)
	GetEfficiencyMetricsByCategory(ctx context.Context, userID string, category int, limit int) ([]TradeEfficiencyMetric, error)
	GetEfficiencyMetricsByStrategy(ctx context.Context, userID, strategy string, limit int) ([]TradeEfficiencyMetric, error)
}

// Ensure DB implements EfficiencyMetricsRepository
var _ EfficiencyMetricsRepository = (*DB)(nil)

// CreateEfficiencyMetric inserts a new efficiency metric record after trade close.
func (db *DB) CreateEfficiencyMetric(ctx context.Context, metric *TradeEfficiencyMetric) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	query := `
		INSERT INTO trade_efficiency_metrics (
			futures_trade_id, user_id, symbol, mode,
			entry_price, exit_price, entry_time, exit_time,
			original_qty, exit_qty,
			peak_profit_pct, exit_profit_pct, exit_efficiency,
			exit_reason, exit_urgency,
			breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
			decision_mode, entry_strategy, entry_regime, exit_regime,
			trend_score, momentum_score, volatility_score, volume_score,
			adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
			trade_category, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35
		) RETURNING id, created_at`

	now := time.Now()
	if metric.CreatedAt.IsZero() {
		metric.CreatedAt = now
	}

	err := db.Pool.QueryRow(ctx, query,
		metric.FuturesTradeID,
		metric.UserID,
		metric.Symbol,
		metric.Mode,
		metric.EntryPrice,
		metric.ExitPrice,
		metric.EntryTime,
		metric.ExitTime,
		metric.OriginalQty,
		metric.ExitQty,
		metric.PeakProfitPct,
		metric.ExitProfitPct,
		metric.ExitEfficiency,
		metric.ExitReason,
		metric.ExitUrgency,
		metric.BreakevenAchieved,
		metric.BreakevenTime,
		metric.TP1Hit,
		metric.TP1Time,
		metric.TP1Qty,
		metric.TP1Profit,
		metric.DecisionMode,
		metric.EntryStrategy,
		metric.EntryRegime,
		metric.ExitRegime,
		metric.TrendScore,
		metric.MomentumScore,
		metric.VolatilityScore,
		metric.VolumeScore,
		metric.ADXAtExit,
		metric.RSIAtExit,
		metric.ATRPctAtExit,
		metric.TrendDirection,
		metric.TrendStrength,
		metric.TradeCategory,
		metric.CreatedAt,
	).Scan(&metric.ID, &metric.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create efficiency metric: %w", err)
	}

	return nil
}

// GetEfficiencyMetricByTradeID retrieves an efficiency metric by its linked futures trade ID.
func (db *DB) GetEfficiencyMetricByTradeID(ctx context.Context, tradeID int64) (*TradeEfficiencyMetric, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, futures_trade_id, user_id, symbol, mode,
			entry_price, exit_price, entry_time, exit_time,
			original_qty, exit_qty,
			peak_profit_pct, exit_profit_pct, exit_efficiency,
			exit_reason, exit_urgency,
			breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
			decision_mode, entry_strategy, entry_regime, exit_regime,
			trend_score, momentum_score, volatility_score, volume_score,
			adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
			trade_category, created_at
		FROM trade_efficiency_metrics
		WHERE futures_trade_id = $1`

	metric := &TradeEfficiencyMetric{}
	err := db.Pool.QueryRow(ctx, query, tradeID).Scan(
		&metric.ID,
		&metric.FuturesTradeID,
		&metric.UserID,
		&metric.Symbol,
		&metric.Mode,
		&metric.EntryPrice,
		&metric.ExitPrice,
		&metric.EntryTime,
		&metric.ExitTime,
		&metric.OriginalQty,
		&metric.ExitQty,
		&metric.PeakProfitPct,
		&metric.ExitProfitPct,
		&metric.ExitEfficiency,
		&metric.ExitReason,
		&metric.ExitUrgency,
		&metric.BreakevenAchieved,
		&metric.BreakevenTime,
		&metric.TP1Hit,
		&metric.TP1Time,
		&metric.TP1Qty,
		&metric.TP1Profit,
		&metric.DecisionMode,
		&metric.EntryStrategy,
		&metric.EntryRegime,
		&metric.ExitRegime,
		&metric.TrendScore,
		&metric.MomentumScore,
		&metric.VolatilityScore,
		&metric.VolumeScore,
		&metric.ADXAtExit,
		&metric.RSIAtExit,
		&metric.ATRPctAtExit,
		&metric.TrendDirection,
		&metric.TrendStrength,
		&metric.TradeCategory,
		&metric.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency metric by trade ID: %w", err)
	}

	return metric, nil
}

// GetRecentEfficiencyMetricsByUserMode retrieves recent efficiency metrics for baseline calculation.
// Returns metrics within the specified time window for the given user and trading mode.
func (db *DB) GetRecentEfficiencyMetricsByUserMode(ctx context.Context, userID, mode string, windowHours int) ([]TradeEfficiencyMetric, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, futures_trade_id, user_id, symbol, mode,
			entry_price, exit_price, entry_time, exit_time,
			original_qty, exit_qty,
			peak_profit_pct, exit_profit_pct, exit_efficiency,
			exit_reason, exit_urgency,
			breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
			decision_mode, entry_strategy, entry_regime, exit_regime,
			trend_score, momentum_score, volatility_score, volume_score,
			adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
			trade_category, created_at
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND mode = $2
			AND created_at >= NOW() - INTERVAL '1 hour' * $3
		ORDER BY created_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID, mode, windowHours)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent efficiency metrics: %w", err)
	}
	defer rows.Close()

	var metrics []TradeEfficiencyMetric
	for rows.Next() {
		metric, err := scanEfficiencyMetricRow(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, *metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating efficiency metric rows: %w", err)
	}

	return metrics, nil
}

// GetAverageExitEfficiency calculates the average exit efficiency for baseline calculation.
// Returns the average exit efficiency and the number of trades used in the calculation.
// Uses direct SQL aggregation for performance.
func (db *DB) GetAverageExitEfficiency(ctx context.Context, userID, mode string, windowHours int) (float64, int, error) {
	if db.Pool == nil {
		return 0, 0, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
			   COUNT(*) as trade_count
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND mode = $2
			AND created_at >= NOW() - INTERVAL '1 hour' * $3`

	var avgEfficiency float64
	var tradeCount int

	err := db.Pool.QueryRow(ctx, query, userID, mode, windowHours).Scan(&avgEfficiency, &tradeCount)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to calculate average exit efficiency: %w", err)
	}

	return avgEfficiency, tradeCount, nil
}

// GetEfficiencyMetricsBySymbol retrieves efficiency metrics for a specific symbol.
func (db *DB) GetEfficiencyMetricsBySymbol(ctx context.Context, userID, symbol string, limit int) ([]TradeEfficiencyMetric, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, futures_trade_id, user_id, symbol, mode,
			entry_price, exit_price, entry_time, exit_time,
			original_qty, exit_qty,
			peak_profit_pct, exit_profit_pct, exit_efficiency,
			exit_reason, exit_urgency,
			breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
			decision_mode, entry_strategy, entry_regime, exit_regime,
			trend_score, momentum_score, volatility_score, volume_score,
			adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
			trade_category, created_at
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND symbol = $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := db.Pool.Query(ctx, query, userID, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency metrics by symbol: %w", err)
	}
	defer rows.Close()

	var metrics []TradeEfficiencyMetric
	for rows.Next() {
		metric, err := scanEfficiencyMetricRow(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, *metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating efficiency metric rows: %w", err)
	}

	return metrics, nil
}

// GetEfficiencyMetricsByCategory retrieves efficiency metrics for a specific trade category.
func (db *DB) GetEfficiencyMetricsByCategory(ctx context.Context, userID string, category int, limit int) ([]TradeEfficiencyMetric, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, futures_trade_id, user_id, symbol, mode,
			entry_price, exit_price, entry_time, exit_time,
			original_qty, exit_qty,
			peak_profit_pct, exit_profit_pct, exit_efficiency,
			exit_reason, exit_urgency,
			breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
			decision_mode, entry_strategy, entry_regime, exit_regime,
			trend_score, momentum_score, volatility_score, volume_score,
			adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
			trade_category, created_at
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND trade_category = $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := db.Pool.Query(ctx, query, userID, category, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency metrics by category: %w", err)
	}
	defer rows.Close()

	var metrics []TradeEfficiencyMetric
	for rows.Next() {
		metric, err := scanEfficiencyMetricRow(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, *metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating efficiency metric rows: %w", err)
	}

	return metrics, nil
}

// GetEfficiencyMetricsByStrategy retrieves efficiency metrics for a specific entry strategy.
func (db *DB) GetEfficiencyMetricsByStrategy(ctx context.Context, userID, strategy string, limit int) ([]TradeEfficiencyMetric, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, futures_trade_id, user_id, symbol, mode,
			entry_price, exit_price, entry_time, exit_time,
			original_qty, exit_qty,
			peak_profit_pct, exit_profit_pct, exit_efficiency,
			exit_reason, exit_urgency,
			breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
			decision_mode, entry_strategy, entry_regime, exit_regime,
			trend_score, momentum_score, volatility_score, volume_score,
			adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
			trade_category, created_at
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND entry_strategy = $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := db.Pool.Query(ctx, query, userID, strategy, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency metrics by strategy: %w", err)
	}
	defer rows.Close()

	var metrics []TradeEfficiencyMetric
	for rows.Next() {
		metric, err := scanEfficiencyMetricRow(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, *metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating efficiency metric rows: %w", err)
	}

	return metrics, nil
}

// scanEfficiencyMetricRow is a helper function to scan an efficiency metric row from the database.
func scanEfficiencyMetricRow(rows pgx.Rows) (*TradeEfficiencyMetric, error) {
	metric := &TradeEfficiencyMetric{}
	err := rows.Scan(
		&metric.ID,
		&metric.FuturesTradeID,
		&metric.UserID,
		&metric.Symbol,
		&metric.Mode,
		&metric.EntryPrice,
		&metric.ExitPrice,
		&metric.EntryTime,
		&metric.ExitTime,
		&metric.OriginalQty,
		&metric.ExitQty,
		&metric.PeakProfitPct,
		&metric.ExitProfitPct,
		&metric.ExitEfficiency,
		&metric.ExitReason,
		&metric.ExitUrgency,
		&metric.BreakevenAchieved,
		&metric.BreakevenTime,
		&metric.TP1Hit,
		&metric.TP1Time,
		&metric.TP1Qty,
		&metric.TP1Profit,
		&metric.DecisionMode,
		&metric.EntryStrategy,
		&metric.EntryRegime,
		&metric.ExitRegime,
		&metric.TrendScore,
		&metric.MomentumScore,
		&metric.VolatilityScore,
		&metric.VolumeScore,
		&metric.ADXAtExit,
		&metric.RSIAtExit,
		&metric.ATRPctAtExit,
		&metric.TrendDirection,
		&metric.TrendStrength,
		&metric.TradeCategory,
		&metric.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan efficiency metric row: %w", err)
	}
	return metric, nil
}

// GetAverageExitEfficiencyByCategory calculates average exit efficiency grouped by trade category.
// Useful for comparing efficiency across different trade types.
func (db *DB) GetAverageExitEfficiencyByCategory(ctx context.Context, userID, mode string, windowHours int) (map[int]float64, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT trade_category, AVG(exit_efficiency) as avg_efficiency
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND mode = $2
			AND created_at >= NOW() - INTERVAL '1 hour' * $3
		GROUP BY trade_category`

	rows, err := db.Pool.Query(ctx, query, userID, mode, windowHours)
	if err != nil {
		return nil, fmt.Errorf("failed to get average efficiency by category: %w", err)
	}
	defer rows.Close()

	results := make(map[int]float64)
	for rows.Next() {
		var category int
		var avgEfficiency float64
		if err := rows.Scan(&category, &avgEfficiency); err != nil {
			return nil, fmt.Errorf("failed to scan category efficiency: %w", err)
		}
		results[category] = avgEfficiency
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating category efficiency rows: %w", err)
	}

	return results, nil
}

// GetEfficiencyStatsByRegime calculates efficiency statistics grouped by market regime.
func (db *DB) GetEfficiencyStatsByRegime(ctx context.Context, userID, mode string, windowHours int) (map[string]EfficiencyStats, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT entry_regime,
			   COUNT(*) as trade_count,
			   AVG(exit_efficiency) as avg_efficiency,
			   MIN(exit_efficiency) as min_efficiency,
			   MAX(exit_efficiency) as max_efficiency,
			   AVG(exit_profit_pct) as avg_profit
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND mode = $2
			AND created_at >= NOW() - INTERVAL '1 hour' * $3
			AND entry_regime IS NOT NULL
		GROUP BY entry_regime`

	rows, err := db.Pool.Query(ctx, query, userID, mode, windowHours)
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency stats by regime: %w", err)
	}
	defer rows.Close()

	results := make(map[string]EfficiencyStats)
	for rows.Next() {
		var regime string
		var stats EfficiencyStats
		if err := rows.Scan(&regime, &stats.TradeCount, &stats.AvgEfficiency,
			&stats.MinEfficiency, &stats.MaxEfficiency, &stats.AvgProfit); err != nil {
			return nil, fmt.Errorf("failed to scan regime stats: %w", err)
		}
		results[regime] = stats
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating regime stats rows: %w", err)
	}

	return results, nil
}

// EfficiencyStats holds aggregated efficiency statistics.
type EfficiencyStats struct {
	TradeCount    int     `json:"trade_count"`
	AvgEfficiency float64 `json:"avg_efficiency"`
	MinEfficiency float64 `json:"min_efficiency"`
	MaxEfficiency float64 `json:"max_efficiency"`
	AvgProfit     float64 `json:"avg_profit"`
}

// DeleteEfficiencyMetricsByUser deletes all efficiency metrics for a user.
// Used for testing/cleanup purposes.
func (db *DB) DeleteEfficiencyMetricsByUser(ctx context.Context, userID string) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	query := `DELETE FROM trade_efficiency_metrics WHERE user_id = $1`
	_, err := db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete efficiency metrics: %w", err)
	}

	return nil
}

// ==================== Story 10.2: Position Analytics Dashboard ====================

// EfficiencyTimelinePoint represents a single data point in efficiency timeline.
type EfficiencyTimelinePoint struct {
	Timestamp     time.Time `json:"timestamp"`
	AvgEfficiency float64   `json:"avg_efficiency"`
	TradeCount    int       `json:"trade_count"`
	WinRate       float64   `json:"win_rate"`
	AvgPnL        float64   `json:"avg_pnl"`
}

// GetEfficiencyTimeline returns efficiency metrics aggregated over time.
// Aggregation is daily for 30d+ ranges, hourly for 7d range.
func (db *DB) GetEfficiencyTimeline(ctx context.Context, userID string, startTime, endTime time.Time, hourly bool) ([]EfficiencyTimelinePoint, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	var query string
	if hourly {
		query = `
			SELECT
				DATE_TRUNC('hour', exit_time) as period,
				COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
				COUNT(*) as trade_count,
				COALESCE(AVG(CASE WHEN exit_profit_pct > 0 THEN 1.0 ELSE 0.0 END) * 100, 0) as win_rate,
				COALESCE(AVG(exit_profit_pct), 0) as avg_pnl
			FROM trade_efficiency_metrics
			WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
			GROUP BY DATE_TRUNC('hour', exit_time)
			ORDER BY period`
	} else {
		query = `
			SELECT
				DATE(exit_time) as period,
				COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
				COUNT(*) as trade_count,
				COALESCE(AVG(CASE WHEN exit_profit_pct > 0 THEN 1.0 ELSE 0.0 END) * 100, 0) as win_rate,
				COALESCE(AVG(exit_profit_pct), 0) as avg_pnl
			FROM trade_efficiency_metrics
			WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
			GROUP BY DATE(exit_time)
			ORDER BY period`
	}

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency timeline: %w", err)
	}
	defer rows.Close()

	var points []EfficiencyTimelinePoint
	for rows.Next() {
		var point EfficiencyTimelinePoint
		if err := rows.Scan(&point.Timestamp, &point.AvgEfficiency, &point.TradeCount, &point.WinRate, &point.AvgPnL); err != nil {
			return nil, fmt.Errorf("failed to scan timeline point: %w", err)
		}
		points = append(points, point)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating timeline rows: %w", err)
	}

	return points, nil
}

// ModeDistributionItem represents trade distribution by mode.
type ModeDistributionItem struct {
	Mode       string  `json:"mode"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// GetTradeDistributionByMode returns trade count distribution by trading mode.
func (db *DB) GetTradeDistributionByMode(ctx context.Context, userID string, startTime, endTime time.Time) ([]ModeDistributionItem, int, error) {
	if db.Pool == nil {
		return nil, 0, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			mode,
			COUNT(*) as count
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY mode
		ORDER BY count DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mode distribution: %w", err)
	}
	defer rows.Close()

	var items []ModeDistributionItem
	var total int
	for rows.Next() {
		var item ModeDistributionItem
		if err := rows.Scan(&item.Mode, &item.Count); err != nil {
			return nil, 0, fmt.Errorf("failed to scan mode distribution: %w", err)
		}
		total += item.Count
		items = append(items, item)
	}

	// Calculate percentages
	for i := range items {
		if total > 0 {
			items[i].Percentage = float64(items[i].Count) / float64(total) * 100
		}
	}

	return items, total, nil
}

// ExitReasonDistributionItem represents trade distribution by exit reason.
type ExitReasonDistributionItem struct {
	ExitReason string  `json:"exit_reason"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// GetTradeDistributionByExitReason returns trade count distribution by exit reason.
func (db *DB) GetTradeDistributionByExitReason(ctx context.Context, userID string, startTime, endTime time.Time) ([]ExitReasonDistributionItem, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			COALESCE(exit_reason, 'UNKNOWN') as exit_reason,
			COUNT(*) as count
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY exit_reason
		ORDER BY count DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get exit reason distribution: %w", err)
	}
	defer rows.Close()

	var items []ExitReasonDistributionItem
	var total int
	for rows.Next() {
		var item ExitReasonDistributionItem
		if err := rows.Scan(&item.ExitReason, &item.Count); err != nil {
			return nil, fmt.Errorf("failed to scan exit reason distribution: %w", err)
		}
		total += item.Count
		items = append(items, item)
	}

	// Calculate percentages
	for i := range items {
		if total > 0 {
			items[i].Percentage = float64(items[i].Count) / float64(total) * 100
		}
	}

	return items, nil
}

// StrategyDistributionItem represents trade distribution by entry strategy.
type StrategyDistributionItem struct {
	Strategy   string  `json:"strategy"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// GetTradeDistributionByStrategy returns trade count distribution by entry strategy.
func (db *DB) GetTradeDistributionByStrategy(ctx context.Context, userID string, startTime, endTime time.Time) ([]StrategyDistributionItem, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			COALESCE(entry_strategy, 'unknown') as strategy,
			COUNT(*) as count
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY entry_strategy
		ORDER BY count DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy distribution: %w", err)
	}
	defer rows.Close()

	var items []StrategyDistributionItem
	var total int
	for rows.Next() {
		var item StrategyDistributionItem
		if err := rows.Scan(&item.Strategy, &item.Count); err != nil {
			return nil, fmt.Errorf("failed to scan strategy distribution: %w", err)
		}
		total += item.Count
		items = append(items, item)
	}

	// Calculate percentages
	for i := range items {
		if total > 0 {
			items[i].Percentage = float64(items[i].Count) / float64(total) * 100
		}
	}

	return items, nil
}

// DecisionModeWinLoss represents win/loss breakdown by decision mode.
type DecisionModeWinLoss struct {
	DecisionMode string  `json:"decision_mode"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"win_rate"`
	TotalTrades  int     `json:"total_trades"`
}

// GetWinLossByDecisionMode returns win/loss breakdown by decision mode (classic vs new_engine).
func (db *DB) GetWinLossByDecisionMode(ctx context.Context, userID string, startTime, endTime time.Time) ([]DecisionModeWinLoss, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			COALESCE(decision_mode, 'classic') as decision_mode,
			COUNT(*) FILTER (WHERE exit_profit_pct > 0) as wins,
			COUNT(*) FILTER (WHERE exit_profit_pct <= 0) as losses,
			COUNT(*) as total_trades
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY decision_mode
		ORDER BY total_trades DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get decision mode win/loss: %w", err)
	}
	defer rows.Close()

	var items []DecisionModeWinLoss
	for rows.Next() {
		var item DecisionModeWinLoss
		if err := rows.Scan(&item.DecisionMode, &item.Wins, &item.Losses, &item.TotalTrades); err != nil {
			return nil, fmt.Errorf("failed to scan decision mode win/loss: %w", err)
		}
		if item.TotalTrades > 0 {
			item.WinRate = float64(item.Wins) / float64(item.TotalTrades) * 100
		}
		items = append(items, item)
	}

	return items, nil
}

// ModePerformanceMetrics contains detailed performance metrics by mode.
type ModePerformanceMetrics struct {
	Mode           string  `json:"mode"`
	TotalTrades    int     `json:"total_trades"`
	WinCount       int     `json:"win_count"`
	LossCount      int     `json:"loss_count"`
	WinRate        float64 `json:"win_rate"`
	AvgEfficiency  float64 `json:"avg_efficiency"`
	AvgPeakProfit  float64 `json:"avg_peak_profit"`
	AvgExitProfit  float64 `json:"avg_exit_profit"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnL         float64 `json:"avg_pnl"`
	ProfitFactor   float64 `json:"profit_factor"`
	BreakevenRate  float64 `json:"breakeven_rate"`
	TP1HitRate     float64 `json:"tp1_hit_rate"`
	AvgHoldMinutes float64 `json:"avg_hold_minutes"`
}

// GetModePerformanceMetrics returns detailed performance metrics grouped by trading mode.
func (db *DB) GetModePerformanceMetrics(ctx context.Context, userID string, startTime, endTime time.Time) ([]ModePerformanceMetrics, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			mode,
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE exit_profit_pct > 0) as win_count,
			COUNT(*) FILTER (WHERE exit_profit_pct <= 0) as loss_count,
			COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
			COALESCE(AVG(peak_profit_pct), 0) as avg_peak_profit,
			COALESCE(AVG(exit_profit_pct), 0) as avg_exit_profit,
			COALESCE(SUM(exit_profit_pct), 0) as total_pnl,
			COALESCE(SUM(exit_profit_pct) FILTER (WHERE exit_profit_pct > 0), 0) as gross_profit,
			COALESCE(ABS(SUM(exit_profit_pct) FILTER (WHERE exit_profit_pct < 0)), 0.001) as gross_loss,
			COALESCE(AVG(CASE WHEN breakeven_achieved THEN 1.0 ELSE 0.0 END) * 100, 0) as breakeven_rate,
			COALESCE(AVG(CASE WHEN tp1_hit THEN 1.0 ELSE 0.0 END) * 100, 0) as tp1_hit_rate,
			COALESCE(AVG(EXTRACT(EPOCH FROM (exit_time - entry_time)) / 60), 0) as avg_hold_minutes
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY mode
		ORDER BY total_trades DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode performance metrics: %w", err)
	}
	defer rows.Close()

	var metrics []ModePerformanceMetrics
	for rows.Next() {
		var m ModePerformanceMetrics
		var grossProfit, grossLoss float64
		if err := rows.Scan(
			&m.Mode, &m.TotalTrades, &m.WinCount, &m.LossCount,
			&m.AvgEfficiency, &m.AvgPeakProfit, &m.AvgExitProfit, &m.TotalPnL,
			&grossProfit, &grossLoss, &m.BreakevenRate, &m.TP1HitRate, &m.AvgHoldMinutes,
		); err != nil {
			return nil, fmt.Errorf("failed to scan mode performance: %w", err)
		}
		if m.TotalTrades > 0 {
			m.WinRate = float64(m.WinCount) / float64(m.TotalTrades) * 100
			m.AvgPnL = m.TotalPnL / float64(m.TotalTrades)
		}
		if grossLoss > 0 {
			m.ProfitFactor = grossProfit / grossLoss
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// StrategyPerformanceMetrics contains detailed performance metrics by strategy.
type StrategyPerformanceMetrics struct {
	Strategy       string  `json:"strategy"`
	TotalTrades    int     `json:"total_trades"`
	WinCount       int     `json:"win_count"`
	LossCount      int     `json:"loss_count"`
	WinRate        float64 `json:"win_rate"`
	AvgEfficiency  float64 `json:"avg_efficiency"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnL         float64 `json:"avg_pnl"`
	AvgHoldMinutes float64 `json:"avg_hold_minutes"`
	BreakevenRate  float64 `json:"breakeven_rate"`
	TP1HitRate     float64 `json:"tp1_hit_rate"`
}

// GetStrategyPerformanceMetrics returns detailed performance metrics grouped by entry strategy.
func (db *DB) GetStrategyPerformanceMetrics(ctx context.Context, userID string, startTime, endTime time.Time) ([]StrategyPerformanceMetrics, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			COALESCE(entry_strategy, 'unknown') as strategy,
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE exit_profit_pct > 0) as win_count,
			COUNT(*) FILTER (WHERE exit_profit_pct <= 0) as loss_count,
			COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
			COALESCE(SUM(exit_profit_pct), 0) as total_pnl,
			COALESCE(AVG(EXTRACT(EPOCH FROM (exit_time - entry_time)) / 60), 0) as avg_hold_minutes,
			COALESCE(AVG(CASE WHEN breakeven_achieved THEN 1.0 ELSE 0.0 END) * 100, 0) as breakeven_rate,
			COALESCE(AVG(CASE WHEN tp1_hit THEN 1.0 ELSE 0.0 END) * 100, 0) as tp1_hit_rate
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY entry_strategy
		ORDER BY total_trades DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy performance metrics: %w", err)
	}
	defer rows.Close()

	var metrics []StrategyPerformanceMetrics
	for rows.Next() {
		var m StrategyPerformanceMetrics
		if err := rows.Scan(
			&m.Strategy, &m.TotalTrades, &m.WinCount, &m.LossCount,
			&m.AvgEfficiency, &m.TotalPnL, &m.AvgHoldMinutes, &m.BreakevenRate, &m.TP1HitRate,
		); err != nil {
			return nil, fmt.Errorf("failed to scan strategy performance: %w", err)
		}
		if m.TotalTrades > 0 {
			m.WinRate = float64(m.WinCount) / float64(m.TotalTrades) * 100
			m.AvgPnL = m.TotalPnL / float64(m.TotalTrades)
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// RegimePerformanceMetrics contains performance metrics by market regime.
type RegimePerformanceMetrics struct {
	Regime        string  `json:"regime"`
	TotalTrades   int     `json:"total_trades"`
	WinCount      int     `json:"win_count"`
	LossCount     int     `json:"loss_count"`
	WinRate       float64 `json:"win_rate"`
	AvgEfficiency float64 `json:"avg_efficiency"`
	TotalPnL      float64 `json:"total_pnl"`
	AvgPnL        float64 `json:"avg_pnl"`
}

// GetRegimePerformanceMetrics returns performance metrics grouped by market regime.
func (db *DB) GetRegimePerformanceMetrics(ctx context.Context, userID string, startTime, endTime time.Time) ([]RegimePerformanceMetrics, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			COALESCE(entry_regime, 'UNKNOWN') as regime,
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE exit_profit_pct > 0) as win_count,
			COUNT(*) FILTER (WHERE exit_profit_pct <= 0) as loss_count,
			COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
			COALESCE(SUM(exit_profit_pct), 0) as total_pnl
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY entry_regime
		ORDER BY total_trades DESC`

	rows, err := db.Pool.Query(ctx, query, userID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get regime performance metrics: %w", err)
	}
	defer rows.Close()

	var metrics []RegimePerformanceMetrics
	for rows.Next() {
		var m RegimePerformanceMetrics
		if err := rows.Scan(
			&m.Regime, &m.TotalTrades, &m.WinCount, &m.LossCount,
			&m.AvgEfficiency, &m.TotalPnL,
		); err != nil {
			return nil, fmt.Errorf("failed to scan regime performance: %w", err)
		}
		if m.TotalTrades > 0 {
			m.WinRate = float64(m.WinCount) / float64(m.TotalTrades) * 100
			m.AvgPnL = m.TotalPnL / float64(m.TotalTrades)
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// PositionAnalyticsSummary contains overall analytics summary.
type PositionAnalyticsSummary struct {
	TotalTrades       int     `json:"total_trades"`
	OverallWinRate    float64 `json:"overall_win_rate"`
	OverallEfficiency float64 `json:"overall_efficiency"`
	TotalPnL          float64 `json:"total_pnl"`
	AvgPnL            float64 `json:"avg_pnl"`
	BestMode          string  `json:"best_mode"`
	BestStrategy      string  `json:"best_strategy"`
}

// GetPositionAnalyticsSummary returns overall analytics summary for a user.
func (db *DB) GetPositionAnalyticsSummary(ctx context.Context, userID string, startTime, endTime time.Time) (*PositionAnalyticsSummary, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT
			COUNT(*) as total_trades,
			COALESCE(AVG(CASE WHEN exit_profit_pct > 0 THEN 1.0 ELSE 0.0 END) * 100, 0) as win_rate,
			COALESCE(AVG(exit_efficiency), 0) as avg_efficiency,
			COALESCE(SUM(exit_profit_pct), 0) as total_pnl
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3`

	summary := &PositionAnalyticsSummary{}
	err := db.Pool.QueryRow(ctx, query, userID, startTime, endTime).Scan(
		&summary.TotalTrades, &summary.OverallWinRate, &summary.OverallEfficiency, &summary.TotalPnL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics summary: %w", err)
	}

	if summary.TotalTrades > 0 {
		summary.AvgPnL = summary.TotalPnL / float64(summary.TotalTrades)
	}

	// Get best mode by efficiency
	bestModeQuery := `
		SELECT mode
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY mode
		ORDER BY AVG(exit_efficiency) DESC
		LIMIT 1`
	_ = db.Pool.QueryRow(ctx, bestModeQuery, userID, startTime, endTime).Scan(&summary.BestMode)

	// Get best strategy by efficiency
	bestStrategyQuery := `
		SELECT COALESCE(entry_strategy, 'unknown')
		FROM trade_efficiency_metrics
		WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
		GROUP BY entry_strategy
		ORDER BY AVG(exit_efficiency) DESC
		LIMIT 1`
	_ = db.Pool.QueryRow(ctx, bestStrategyQuery, userID, startTime, endTime).Scan(&summary.BestStrategy)

	return summary, nil
}

// GetAllEfficiencyMetricsForExport returns all efficiency metrics for a user within a time range.
// Used for CSV/JSON export functionality.
func (db *DB) GetAllEfficiencyMetricsForExport(ctx context.Context, userID string, startTime, endTime time.Time, mode string) ([]TradeEfficiencyMetric, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	var query string
	var args []interface{}

	if mode != "" {
		query = `
			SELECT id, futures_trade_id, user_id, symbol, mode,
				entry_price, exit_price, entry_time, exit_time,
				original_qty, exit_qty,
				peak_profit_pct, exit_profit_pct, exit_efficiency,
				exit_reason, exit_urgency,
				breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
				decision_mode, entry_strategy, entry_regime, exit_regime,
				trend_score, momentum_score, volatility_score, volume_score,
				adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
				trade_category, created_at
			FROM trade_efficiency_metrics
			WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3 AND mode = $4
			ORDER BY exit_time DESC`
		args = []interface{}{userID, startTime, endTime, mode}
	} else {
		query = `
			SELECT id, futures_trade_id, user_id, symbol, mode,
				entry_price, exit_price, entry_time, exit_time,
				original_qty, exit_qty,
				peak_profit_pct, exit_profit_pct, exit_efficiency,
				exit_reason, exit_urgency,
				breakeven_achieved, breakeven_time, tp1_hit, tp1_time, tp1_qty, tp1_profit,
				decision_mode, entry_strategy, entry_regime, exit_regime,
				trend_score, momentum_score, volatility_score, volume_score,
				adx_at_exit, rsi_at_exit, atr_pct_at_exit, trend_direction, trend_strength,
				trade_category, created_at
			FROM trade_efficiency_metrics
			WHERE user_id = $1 AND exit_time >= $2 AND exit_time <= $3
			ORDER BY exit_time DESC`
		args = []interface{}{userID, startTime, endTime}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get efficiency metrics for export: %w", err)
	}
	defer rows.Close()

	var metrics []TradeEfficiencyMetric
	for rows.Next() {
		metric, err := scanEfficiencyMetricRow(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, *metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating export rows: %w", err)
	}

	return metrics, nil
}
