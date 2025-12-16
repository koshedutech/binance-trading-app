package strategy

import (
	"encoding/json"
	"fmt"
	"time"

	"binance-trading-bot/internal/binance"
)

// VisualStrategy implements Strategy interface by interpreting visual flow
type VisualStrategy struct {
	name     string
	symbol   string
	interval string
	flowDef  *VisualFlowDefinition
}

// VisualFlowDefinition represents the complete visual flow
type VisualFlowDefinition struct {
	Version  string              `json:"version"`
	Nodes    []FlowNode          `json:"nodes"`
	Edges    []FlowEdge          `json:"edges"`
	Settings VisualFlowSettings  `json:"settings"`
}

// FlowNode represents a single node in the flow
type FlowNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // entry, exit, indicator, condition, action
	Position map[string]float64     `json:"position"`
	Data     map[string]interface{} `json:"data"`
}

// FlowEdge represents a connection between nodes
type FlowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// VisualFlowSettings contains flow-level configuration
type VisualFlowSettings struct {
	Symbol         string                 `json:"symbol"`
	Interval       string                 `json:"interval"`
	StopLoss       *RiskSetting           `json:"stopLoss,omitempty"`
	TakeProfit     *RiskSetting           `json:"takeProfit,omitempty"`
	RiskManagement map[string]interface{} `json:"riskManagement,omitempty"`
}

// RiskSetting defines stop loss or take profit configuration
type RiskSetting struct {
	Enabled bool    `json:"enabled"`
	Type    string  `json:"type"` // percentage, absolute, atr
	Value   float64 `json:"value"`
}

// NewVisualStrategy creates a visual strategy from flow definition
func NewVisualStrategy(name string, flowDefJSON map[string]interface{}) (*VisualStrategy, error) {
	// Convert map to VisualFlowDefinition
	flowBytes, err := json.Marshal(flowDefJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flow definition: %w", err)
	}

	var flowDef VisualFlowDefinition
	if err := json.Unmarshal(flowBytes, &flowDef); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow definition: %w", err)
	}

	return &VisualStrategy{
		name:     name,
		symbol:   flowDef.Settings.Symbol,
		interval: flowDef.Settings.Interval,
		flowDef:  &flowDef,
	}, nil
}

// Evaluate implements the Strategy interface
func (vs *VisualStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < 14 {
		return nil, fmt.Errorf("insufficient klines for evaluation")
	}

	// Find entry nodes
	for _, node := range vs.flowDef.Nodes {
		if node.Type != "entry" {
			continue
		}

		// Evaluate entry conditions
		triggered, reason := vs.evaluateEntryNode(node, klines)
		if triggered {
			// Get action from node data
			action := "BUY" // Default
			if actionVal, ok := node.Data["action"].(string); ok {
				action = actionVal
			}

			// Calculate stop loss and take profit
			// Use the currentPrice parameter passed to Evaluate
			var stopLoss, takeProfit float64

			if vs.flowDef.Settings.StopLoss != nil && vs.flowDef.Settings.StopLoss.Enabled {
				stopLoss = vs.calculateStopLoss(currentPrice, vs.flowDef.Settings.StopLoss)
			}

			if vs.flowDef.Settings.TakeProfit != nil && vs.flowDef.Settings.TakeProfit.Enabled {
				takeProfit = vs.calculateTakeProfit(currentPrice, vs.flowDef.Settings.TakeProfit)
			}

			// Convert action string to SignalType
			var signalType SignalType
			if action == "BUY" {
				signalType = SignalBuy
			} else if action == "SELL" {
				signalType = SignalSell
			} else {
				signalType = SignalNone
			}

			return &Signal{
				Symbol:     vs.symbol,
				Type:       signalType,
				EntryPrice: currentPrice,
				StopLoss:   stopLoss,
				TakeProfit: takeProfit,
				Quantity:   0, // Will be calculated by position sizing logic
				Reason:     reason,
				OrderType:  "MARKET",
				Side:       action,
				Timestamp:  time.Now(),
			}, nil
		}
	}

	return nil, nil
}

// evaluateEntryNode checks if entry conditions are met
func (vs *VisualStrategy) evaluateEntryNode(node FlowNode, klines []binance.Kline) (bool, string) {
	// Get condition group from node data
	conditionGroupData, ok := node.Data["conditionGroup"].(map[string]interface{})
	if !ok {
		return false, "no condition group found"
	}

	// Evaluate condition group
	return vs.evaluateConditionGroup(conditionGroupData, klines)
}

// evaluateConditionGroup recursively evaluates a group of conditions
func (vs *VisualStrategy) evaluateConditionGroup(groupData map[string]interface{}, klines []binance.Kline) (bool, string) {
	operator, _ := groupData["operator"].(string)
	conditions, _ := groupData["conditions"].([]interface{})

	results := []bool{}
	reasons := []string{}

	// Evaluate individual conditions
	for _, condData := range conditions {
		condMap, ok := condData.(map[string]interface{})
		if !ok {
			continue
		}

		result, reason := vs.evaluateCondition(condMap, klines)
		results = append(results, result)
		if result {
			reasons = append(reasons, reason)
		}
	}

	// Apply operator
	if operator == "AND" {
		for _, r := range results {
			if !r {
				return false, ""
			}
		}
		return len(results) > 0, fmt.Sprintf("All conditions met: %v", reasons)
	} else { // OR
		for i, r := range results {
			if r {
				return true, reasons[i]
			}
		}
		return false, ""
	}
}

// evaluateCondition evaluates a single condition
func (vs *VisualStrategy) evaluateCondition(condition map[string]interface{}, klines []binance.Kline) (bool, string) {
	// Check if this is the new operand-based condition format (from ConditionBuilder)
	if leftOperand, hasLeft := condition["leftOperand"]; hasLeft {
		if operator, hasOp := condition["operator"]; hasOp {
			if rightOperand, hasRight := condition["rightOperand"]; hasRight {
				return vs.evaluateOperandCondition(
					leftOperand.(map[string]interface{}),
					operator.(string),
					rightOperand.(map[string]interface{}),
					klines,
				)
			}
		}
	}

	// Fallback to legacy condition format
	condType, _ := condition["type"].(string)

	switch condType {
	case "indicator_comparison":
		return vs.evaluateIndicatorComparison(condition, klines)
	case "candle_property":
		return vs.evaluateCandleProperty(condition, klines)
	default:
		return false, ""
	}
}

// evaluateOperandCondition evaluates the new operand-based condition format
func (vs *VisualStrategy) evaluateOperandCondition(
	leftOperand map[string]interface{},
	operator string,
	rightOperand map[string]interface{},
	klines []binance.Kline,
) (bool, string) {
	// Get left value
	leftValue, leftDesc := vs.evaluateOperand(leftOperand, klines)

	// Get right value
	rightValue, rightDesc := vs.evaluateOperand(rightOperand, klines)

	// Compare values
	if compareValues(leftValue, operator, rightValue) {
		return true, fmt.Sprintf("%s %s %s (%.2f %s %.2f)", leftDesc, operator, rightDesc, leftValue, operator, rightValue)
	}

	return false, ""
}

// evaluateOperand evaluates an operand (left or right) and returns its value and description
func (vs *VisualStrategy) evaluateOperand(operand map[string]interface{}, klines []binance.Kline) (float64, string) {
	operandType, _ := operand["type"].(string)

	switch operandType {
	case "price":
		currentPrice := klines[len(klines)-1].Close
		return currentPrice, "LTP"

	case "value":
		value, _ := operand["value"].(float64)
		return value, fmt.Sprintf("%.2f", value)

	case "indicator":
		indicator, _ := operand["indicator"].(string)
		params, _ := operand["params"].(map[string]interface{})
		return vs.calculateIndicatorValue(indicator, params, klines)
	}

	return 0, "unknown"
}

// calculateIndicatorValue calculates the value of an indicator
func (vs *VisualStrategy) calculateIndicatorValue(indicator string, params map[string]interface{}, klines []binance.Kline) (float64, string) {
	switch indicator {
	case "RSI":
		period := getIntParam(params, "period", 14)
		value := CalculateRSI(klines, period)
		return value, fmt.Sprintf("RSI(%d)", period)

	case "SMA":
		period := getIntParam(params, "period", 20)
		value := CalculateSMA(klines, period)
		return value, fmt.Sprintf("SMA(%d)", period)

	case "EMA":
		period := getIntParam(params, "period", 20)
		value := CalculateEMA(klines, period)
		return value, fmt.Sprintf("EMA(%d)", period)

	case "MACD":
		fastPeriod := getIntParam(params, "fastPeriod", 12)
		slowPeriod := getIntParam(params, "slowPeriod", 26)
		signalPeriod := getIntParam(params, "signalPeriod", 9)
		macdType := getStringParam(params, "type", "histogram")
		macdResult := CalculateMACD(klines, fastPeriod, slowPeriod, signalPeriod)

		var value float64
		switch macdType {
		case "macd":
			value = macdResult.MACD
		case "signal":
			value = macdResult.Signal
		default:
			value = macdResult.Histogram
		}
		return value, fmt.Sprintf("MACD_%s", macdType)

	case "BollingerBands":
		period := getIntParam(params, "period", 20)
		stdDev := getFloatParam(params, "stdDev", 2.0)
		bandType := getStringParam(params, "band", "lower")
		bb := CalculateBollingerBands(klines, period, stdDev)

		var value float64
		switch bandType {
		case "upper":
			value = bb.Upper
		case "middle":
			value = bb.Middle
		default:
			value = bb.Lower
		}
		return value, fmt.Sprintf("BB_%s(%d)", bandType, period)

	case "Stochastic":
		kPeriod := getIntParam(params, "kPeriod", 14)
		dPeriod := getIntParam(params, "dPeriod", 3)
		stochType := getStringParam(params, "type", "k")
		stoch := CalculateStochastic(klines, kPeriod, dPeriod)

		var value float64
		if stochType == "d" {
			value = stoch.D
		} else {
			value = stoch.K
		}
		return value, fmt.Sprintf("Stoch_%s", stochType)

	case "ATR":
		period := getIntParam(params, "period", 14)
		value := CalculateATR(klines, period)
		return value, fmt.Sprintf("ATR(%d)", period)

	case "ADX":
		period := getIntParam(params, "period", 14)
		value := CalculateADX(klines, period)
		return value, fmt.Sprintf("ADX(%d)", period)

	case "Volume":
		period := getIntParam(params, "period", 20)
		value := CalculateAverageVolume(klines, period)
		return value, fmt.Sprintf("Vol(%d)", period)
	}

	return 0, "unknown"
}

// evaluateIndicatorComparison evaluates indicator-based conditions
func (vs *VisualStrategy) evaluateIndicatorComparison(condition map[string]interface{}, klines []binance.Kline) (bool, string) {
	indicator, _ := condition["indicator"].(string)
	comparison, _ := condition["comparison"].(string)
	value, _ := condition["value"].(float64)
	params, _ := condition["params"].(map[string]interface{})

	switch indicator {
	case "RSI":
		period := getIntParam(params, "period", 14)
		currentValue := CalculateRSI(klines, period)
		if compareValues(currentValue, comparison, value) {
			return true, fmt.Sprintf("RSI(%d)=%.2f %s %.2f", period, currentValue, comparison, value)
		}

	case "SMA":
		period := getIntParam(params, "period", 20)
		currentValue := CalculateSMA(klines, period)
		currentPrice := klines[len(klines)-1].Close
		if compareValues(currentPrice, comparison, currentValue) {
			return true, fmt.Sprintf("Price(%.2f) %s SMA(%d)=%.2f", currentPrice, comparison, period, currentValue)
		}

	case "EMA":
		period := getIntParam(params, "period", 20)
		currentValue := CalculateEMA(klines, period)
		currentPrice := klines[len(klines)-1].Close
		if compareValues(currentPrice, comparison, currentValue) {
			return true, fmt.Sprintf("Price(%.2f) %s EMA(%d)=%.2f", currentPrice, comparison, period, currentValue)
		}

	case "MACD":
		fastPeriod := getIntParam(params, "fastPeriod", 12)
		slowPeriod := getIntParam(params, "slowPeriod", 26)
		signalPeriod := getIntParam(params, "signalPeriod", 9)
		macdResult := CalculateMACD(klines, fastPeriod, slowPeriod, signalPeriod)

		// Check which MACD value to compare (macd, signal, or histogram)
		macdType := getStringParam(params, "type", "histogram")
		var currentValue float64
		switch macdType {
		case "macd":
			currentValue = macdResult.MACD
		case "signal":
			currentValue = macdResult.Signal
		default:
			currentValue = macdResult.Histogram
		}

		if compareValues(currentValue, comparison, value) {
			return true, fmt.Sprintf("MACD_%s(%.4f) %s %.4f", macdType, currentValue, comparison, value)
		}

	case "BollingerBands":
		period := getIntParam(params, "period", 20)
		stdDev := getFloatParam(params, "stdDev", 2.0)
		bb := CalculateBollingerBands(klines, period, stdDev)
		currentPrice := klines[len(klines)-1].Close

		// Check which band to compare
		bandType := getStringParam(params, "band", "lower")
		var bandValue float64
		switch bandType {
		case "upper":
			bandValue = bb.Upper
		case "middle":
			bandValue = bb.Middle
		default:
			bandValue = bb.Lower
		}

		if compareValues(currentPrice, comparison, bandValue) {
			return true, fmt.Sprintf("Price(%.2f) %s BB_%s(%.2f)", currentPrice, comparison, bandType, bandValue)
		}

	case "Stochastic":
		kPeriod := getIntParam(params, "kPeriod", 14)
		dPeriod := getIntParam(params, "dPeriod", 3)
		stoch := CalculateStochastic(klines, kPeriod, dPeriod)

		stochType := getStringParam(params, "type", "k")
		var currentValue float64
		if stochType == "d" {
			currentValue = stoch.D
		} else {
			currentValue = stoch.K
		}

		if compareValues(currentValue, comparison, value) {
			return true, fmt.Sprintf("Stoch_%s(%.2f) %s %.2f", stochType, currentValue, comparison, value)
		}

	case "ATR":
		period := getIntParam(params, "period", 14)
		currentValue := CalculateATR(klines, period)
		if compareValues(currentValue, comparison, value) {
			return true, fmt.Sprintf("ATR(%d)=%.4f %s %.4f", period, currentValue, comparison, value)
		}

	case "ADX":
		period := getIntParam(params, "period", 14)
		currentValue := CalculateADX(klines, period)
		if compareValues(currentValue, comparison, value) {
			return true, fmt.Sprintf("ADX(%d)=%.2f %s %.2f", period, currentValue, comparison, value)
		}

	case "Volume":
		period := getIntParam(params, "period", 20)
		avgVolume := CalculateAverageVolume(klines, period)
		currentVolume := klines[len(klines)-1].Volume
		if compareValues(currentVolume, comparison, avgVolume*value) {
			return true, fmt.Sprintf("Volume(%.0f) %s %.0fx_avg", currentVolume, comparison, value)
		}
	}

	return false, ""
}

// Helper functions to extract parameters
func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func getFloatParam(params map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := params[key].(float64); ok {
		return val
	}
	return defaultValue
}

func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

// evaluateCandleProperty evaluates candle-based conditions
func (vs *VisualStrategy) evaluateCandleProperty(condition map[string]interface{}, klines []binance.Kline) (bool, string) {
	property, _ := condition["property"].(string)
	comparison, _ := condition["comparison"].(string)
	reference, _ := condition["reference"].(string)

	if len(klines) < 2 {
		return false, ""
	}

	currentCandle := klines[len(klines)-1]
	previousCandle := klines[len(klines)-2]

	var currentValue, referenceValue float64

	// Get current value
	switch property {
	case "close":
		currentValue = currentCandle.Close
	case "high":
		currentValue = currentCandle.High
	case "low":
		currentValue = currentCandle.Low
	case "open":
		currentValue = currentCandle.Open
	}

	// Get reference value
	switch reference {
	case "previous.high":
		referenceValue = previousCandle.High
	case "previous.low":
		referenceValue = previousCandle.Low
	case "previous.close":
		referenceValue = previousCandle.Close
	case "previous.open":
		referenceValue = previousCandle.Open
	}

	result := compareValues(currentValue, comparison, referenceValue)
	if result {
		return true, fmt.Sprintf("%s(%.2f) %s previous.%s(%.2f)", property, currentValue, comparison, reference, referenceValue)
	}

	return false, ""
}

// calculateStopLoss calculates stop loss price
func (vs *VisualStrategy) calculateStopLoss(currentPrice float64, setting *RiskSetting) float64 {
	switch setting.Type {
	case "percentage":
		return currentPrice * (1 - setting.Value/100)
	case "absolute":
		return currentPrice - setting.Value
	default:
		return currentPrice * 0.98 // Default 2% stop loss
	}
}

// calculateTakeProfit calculates take profit price
func (vs *VisualStrategy) calculateTakeProfit(currentPrice float64, setting *RiskSetting) float64 {
	switch setting.Type {
	case "percentage":
		return currentPrice * (1 + setting.Value/100)
	case "absolute":
		return currentPrice + setting.Value
	default:
		return currentPrice * 1.03 // Default 3% take profit
	}
}

// compareValues compares two values based on operator
func compareValues(a float64, op string, b float64) bool {
	switch op {
	case ">":
		return a > b
	case "<":
		return a < b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	case "==":
		return a == b
	default:
		return false
	}
}

// Name returns strategy name
func (vs *VisualStrategy) Name() string {
	return vs.name
}

// GetSymbol returns trading symbol
func (vs *VisualStrategy) GetSymbol() string {
	return vs.symbol
}

// GetInterval returns timeframe
func (vs *VisualStrategy) GetInterval() string {
	return vs.interval
}
