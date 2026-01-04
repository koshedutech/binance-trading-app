package database

import (
	"context"
	"fmt"
	"log"
	"time"
)

// SymbolRequirements represents Binance exchange requirements for a trading symbol
type SymbolRequirements struct {
	ID                 int64     `json:"id"`
	Symbol             string    `json:"symbol"`
	PricePrecision     int       `json:"price_precision"`
	QuantityPrecision  int       `json:"quantity_precision"`
	TickSize           float64   `json:"tick_size"`
	MinPrice           float64   `json:"min_price"`
	MaxPrice           float64   `json:"max_price"`
	StepSize           float64   `json:"step_size"`
	MinQty             float64   `json:"min_qty"`
	MaxQty             float64   `json:"max_qty"`
	MinNotional        float64   `json:"min_notional"`
	MarketMinQty       float64   `json:"market_min_qty"`
	MarketMaxQty       float64   `json:"market_max_qty"`
	MarketStepSize     float64   `json:"market_step_size"`
	BaseAsset          string    `json:"base_asset"`
	QuoteAsset         string    `json:"quote_asset"`
	MarginAsset        string    `json:"margin_asset"`
	ContractType       string    `json:"contract_type"`
	Status             string    `json:"status"`
	LastSyncedAt       time.Time `json:"last_synced_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// SymbolRequirementsRepository handles database operations for symbol requirements
type SymbolRequirementsRepository struct {
	db *DB
}

// NewSymbolRequirementsRepository creates a new repository instance
func NewSymbolRequirementsRepository(db *DB) *SymbolRequirementsRepository {
	return &SymbolRequirementsRepository{db: db}
}

// UpsertSymbol inserts or updates symbol requirements
func (r *SymbolRequirementsRepository) UpsertSymbol(ctx context.Context, req *SymbolRequirements) error {
	query := `
		INSERT INTO symbol_requirements (
			symbol, price_precision, quantity_precision, tick_size, min_price, max_price,
			step_size, min_qty, max_qty, min_notional, market_min_qty, market_max_qty,
			market_step_size, base_asset, quote_asset, margin_asset, contract_type,
			status, last_synced_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $19
		)
		ON CONFLICT (symbol) DO UPDATE SET
			price_precision = EXCLUDED.price_precision,
			quantity_precision = EXCLUDED.quantity_precision,
			tick_size = EXCLUDED.tick_size,
			min_price = EXCLUDED.min_price,
			max_price = EXCLUDED.max_price,
			step_size = EXCLUDED.step_size,
			min_qty = EXCLUDED.min_qty,
			max_qty = EXCLUDED.max_qty,
			min_notional = EXCLUDED.min_notional,
			market_min_qty = EXCLUDED.market_min_qty,
			market_max_qty = EXCLUDED.market_max_qty,
			market_step_size = EXCLUDED.market_step_size,
			base_asset = EXCLUDED.base_asset,
			quote_asset = EXCLUDED.quote_asset,
			margin_asset = EXCLUDED.margin_asset,
			contract_type = EXCLUDED.contract_type,
			status = EXCLUDED.status,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.Pool.Exec(ctx, query,
		req.Symbol, req.PricePrecision, req.QuantityPrecision, req.TickSize,
		req.MinPrice, req.MaxPrice, req.StepSize, req.MinQty, req.MaxQty,
		req.MinNotional, req.MarketMinQty, req.MarketMaxQty, req.MarketStepSize,
		req.BaseAsset, req.QuoteAsset, req.MarginAsset, req.ContractType,
		req.Status, time.Now(),
	)

	return err
}

// BulkUpsert inserts or updates multiple symbols efficiently
func (r *SymbolRequirementsRepository) BulkUpsert(ctx context.Context, requirements []*SymbolRequirements) (int, error) {
	if len(requirements) == 0 {
		return 0, nil
	}

	// Use a transaction for atomic bulk insert
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	count := 0
	for _, req := range requirements {
		query := `
			INSERT INTO symbol_requirements (
				symbol, price_precision, quantity_precision, tick_size, min_price, max_price,
				step_size, min_qty, max_qty, min_notional, market_min_qty, market_max_qty,
				market_step_size, base_asset, quote_asset, margin_asset, contract_type,
				status, last_synced_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $19
			)
			ON CONFLICT (symbol) DO UPDATE SET
				price_precision = EXCLUDED.price_precision,
				quantity_precision = EXCLUDED.quantity_precision,
				tick_size = EXCLUDED.tick_size,
				min_price = EXCLUDED.min_price,
				max_price = EXCLUDED.max_price,
				step_size = EXCLUDED.step_size,
				min_qty = EXCLUDED.min_qty,
				max_qty = EXCLUDED.max_qty,
				min_notional = EXCLUDED.min_notional,
				market_min_qty = EXCLUDED.market_min_qty,
				market_max_qty = EXCLUDED.market_max_qty,
				market_step_size = EXCLUDED.market_step_size,
				base_asset = EXCLUDED.base_asset,
				quote_asset = EXCLUDED.quote_asset,
				margin_asset = EXCLUDED.margin_asset,
				contract_type = EXCLUDED.contract_type,
				status = EXCLUDED.status,
				last_synced_at = EXCLUDED.last_synced_at,
				updated_at = EXCLUDED.updated_at
		`

		_, err := tx.Exec(ctx, query,
			req.Symbol, req.PricePrecision, req.QuantityPrecision, req.TickSize,
			req.MinPrice, req.MaxPrice, req.StepSize, req.MinQty, req.MaxQty,
			req.MinNotional, req.MarketMinQty, req.MarketMaxQty, req.MarketStepSize,
			req.BaseAsset, req.QuoteAsset, req.MarginAsset, req.ContractType,
			req.Status, time.Now(),
		)
		if err != nil {
			log.Printf("[SYMBOL-SYNC] Failed to upsert %s: %v", req.Symbol, err)
			continue
		}
		count++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return count, nil
}

// GetBySymbol retrieves requirements for a specific symbol
func (r *SymbolRequirementsRepository) GetBySymbol(ctx context.Context, symbol string) (*SymbolRequirements, error) {
	query := `
		SELECT id, symbol, price_precision, quantity_precision, tick_size, min_price,
			max_price, step_size, min_qty, max_qty, min_notional, market_min_qty,
			market_max_qty, market_step_size, base_asset, quote_asset, margin_asset,
			contract_type, status, last_synced_at, created_at, updated_at
		FROM symbol_requirements
		WHERE symbol = $1
	`

	var req SymbolRequirements
	err := r.db.Pool.QueryRow(ctx, query, symbol).Scan(
		&req.ID, &req.Symbol, &req.PricePrecision, &req.QuantityPrecision,
		&req.TickSize, &req.MinPrice, &req.MaxPrice, &req.StepSize,
		&req.MinQty, &req.MaxQty, &req.MinNotional, &req.MarketMinQty,
		&req.MarketMaxQty, &req.MarketStepSize, &req.BaseAsset, &req.QuoteAsset,
		&req.MarginAsset, &req.ContractType, &req.Status, &req.LastSyncedAt,
		&req.CreatedAt, &req.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &req, nil
}

// GetAllActive retrieves all trading symbols
func (r *SymbolRequirementsRepository) GetAllActive(ctx context.Context) ([]*SymbolRequirements, error) {
	query := `
		SELECT id, symbol, price_precision, quantity_precision, tick_size, min_price,
			max_price, step_size, min_qty, max_qty, min_notional, market_min_qty,
			market_max_qty, market_step_size, base_asset, quote_asset, margin_asset,
			contract_type, status, last_synced_at, created_at, updated_at
		FROM symbol_requirements
		WHERE status = 'TRADING'
		ORDER BY symbol
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*SymbolRequirements
	for rows.Next() {
		var req SymbolRequirements
		err := rows.Scan(
			&req.ID, &req.Symbol, &req.PricePrecision, &req.QuantityPrecision,
			&req.TickSize, &req.MinPrice, &req.MaxPrice, &req.StepSize,
			&req.MinQty, &req.MaxQty, &req.MinNotional, &req.MarketMinQty,
			&req.MarketMaxQty, &req.MarketStepSize, &req.BaseAsset, &req.QuoteAsset,
			&req.MarginAsset, &req.ContractType, &req.Status, &req.LastSyncedAt,
			&req.CreatedAt, &req.UpdatedAt,
		)
		if err != nil {
			continue
		}
		results = append(results, &req)
	}

	return results, nil
}

// GetSymbolCount returns the total count of symbols in the database
func (r *SymbolRequirementsRepository) GetSymbolCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM symbol_requirements").Scan(&count)
	return count, err
}

// GetLastSyncTime returns the most recent sync time
func (r *SymbolRequirementsRepository) GetLastSyncTime(ctx context.Context) (*time.Time, error) {
	var lastSync time.Time
	err := r.db.Pool.QueryRow(ctx, "SELECT MAX(last_synced_at) FROM symbol_requirements").Scan(&lastSync)
	if err != nil {
		return nil, err
	}
	return &lastSync, nil
}
