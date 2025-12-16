package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

type contextKey string

const (
	loggerKey  contextKey = "logger"
	traceIDKey contextKey = "trace_id"
)

// GenerateTraceID generates a new trace ID
func GenerateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// FromContext retrieves the logger from context
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		return l
	}
	return Default()
}

// NewContext creates a new context with the logger
func NewContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithTraceContext adds a trace ID to the context and returns a logger with it
func WithTraceContext(ctx context.Context) (context.Context, *Logger) {
	traceID := GenerateTraceID()
	l := Default().WithTraceID(traceID)
	newCtx := context.WithValue(ctx, traceIDKey, traceID)
	newCtx = context.WithValue(newCtx, loggerKey, l)
	return newCtx, l
}

// TradeContext creates a logger context for trade operations
func TradeContext(symbol, side string, quantity, price float64) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol":   symbol,
		"side":     side,
		"quantity": quantity,
		"price":    price,
	}).WithComponent("trade")
}

// OrderContext creates a logger context for order operations
func OrderContext(orderID int64, symbol, side, orderType string) *Logger {
	return Default().WithFields(map[string]interface{}{
		"order_id":   orderID,
		"symbol":     symbol,
		"side":       side,
		"order_type": orderType,
	}).WithComponent("order")
}

// PositionContext creates a logger context for position operations
func PositionContext(symbol, side string, entryPrice, quantity float64) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol":      symbol,
		"side":        side,
		"entry_price": entryPrice,
		"quantity":    quantity,
	}).WithComponent("position")
}

// PatternContext creates a logger context for pattern detection
func PatternContext(symbol, timeframe, patternType string) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol":       symbol,
		"timeframe":    timeframe,
		"pattern_type": patternType,
	}).WithComponent("pattern")
}

// SignalContext creates a logger context for trading signals
func SignalContext(symbol, side string, confidence float64) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol":     symbol,
		"side":       side,
		"confidence": confidence,
	}).WithComponent("signal")
}

// BacktestContext creates a logger context for backtesting
func BacktestContext(symbol string, startDate, endDate time.Time) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol":     symbol,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	}).WithComponent("backtest")
}

// RiskContext creates a logger context for risk management
func RiskContext(symbol string, riskPercent, positionSize float64) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol":        symbol,
		"risk_percent":  riskPercent,
		"position_size": positionSize,
	}).WithComponent("risk")
}

// APIContext creates a logger context for API operations
func APIContext(method, path string, statusCode int) *Logger {
	return Default().WithFields(map[string]interface{}{
		"method":      method,
		"path":        path,
		"status_code": statusCode,
	}).WithComponent("api")
}

// WebSocketContext creates a logger context for WebSocket operations
func WebSocketContext(symbol, stream string) *Logger {
	return Default().WithFields(map[string]interface{}{
		"symbol": symbol,
		"stream": stream,
	}).WithComponent("websocket")
}

// HTTPMiddleware is a middleware that adds logging to HTTP requests
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = GenerateTraceID()
		}

		// Create logger with request context
		l := Default().WithTraceID(traceID).WithFields(map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).WithComponent("http")

		// Add logger to context
		ctx := NewContext(r.Context(), l)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Log request completion
		duration := time.Since(start)
		l.WithDuration(duration).WithField("status_code", wrapped.statusCode).Info("Request completed")
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// BinanceAPIContext creates a logger context for Binance API calls
func BinanceAPIContext(endpoint string, params map[string]interface{}) *Logger {
	l := Default().WithFields(map[string]interface{}{
		"endpoint": endpoint,
	}).WithComponent("binance")

	// Add safe params (exclude sensitive data)
	for k, v := range params {
		if k != "signature" && k != "apiKey" {
			l = l.WithField(k, v)
		}
	}

	return l
}

// DatabaseContext creates a logger context for database operations
func DatabaseContext(operation, table string) *Logger {
	return Default().WithFields(map[string]interface{}{
		"operation": operation,
		"table":     table,
	}).WithComponent("database")
}

// NotificationContext creates a logger context for notifications
func NotificationContext(provider, recipient string) *Logger {
	return Default().WithFields(map[string]interface{}{
		"provider":  provider,
		"recipient": recipient,
	}).WithComponent("notification")
}
