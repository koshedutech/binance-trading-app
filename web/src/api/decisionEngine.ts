// Decision Engine API Client
// Epic 11: Position Decision Engine - Story 11.24: Decision Engine Settings UI

import axios from 'axios';
import type {
  DecisionEngineSettings,
  DecisionEngineComparisonResponse,
  GetDecisionSettingsResponse,
  UpdateDecisionSettingsRequest,
  UpdateDecisionSettingsResponse,
  ResetDecisionSettingsRequest,
  ResetDecisionSettingsResponse,
  ListStrategiesResponse,
  SetActiveStrategyRequest,
  SetActiveStrategyResponse,
  StrategySettings,
  StrategyName,
} from '../types/decision-engine';

// Token storage key
const ACCESS_TOKEN_KEY = 'access_token';

// Base URL for decision engine API
const BASE_URL = '/api/futures/decision-engine';

// Helper to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// ==================== GET /api/futures/decision-engine/settings ====================
// Get user's complete decision engine settings
export async function getDecisionEngineSettings(): Promise<DecisionEngineSettings> {
  const response = await axios.get<GetDecisionSettingsResponse>(
    `${BASE_URL}/settings`,
    { headers: getAuthHeaders() }
  );
  return response.data.settings;
}

// ==================== PUT /api/futures/decision-engine/settings ====================
// Update user's decision engine settings
export async function updateDecisionEngineSettings(
  request: UpdateDecisionSettingsRequest
): Promise<UpdateDecisionSettingsResponse> {
  const response = await axios.put<UpdateDecisionSettingsResponse>(
    `${BASE_URL}/settings`,
    request,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/decision-engine/settings/reset ====================
// Reset settings to defaults (all or specific strategy/group)
export async function resetDecisionEngineSettings(
  request?: ResetDecisionSettingsRequest
): Promise<ResetDecisionSettingsResponse> {
  const response = await axios.post<ResetDecisionSettingsResponse>(
    `${BASE_URL}/settings/reset`,
    request || {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== GET /api/futures/decision-engine/settings/compare ====================
// Compare user settings vs defaults for Settings Comparison UI
export async function getDecisionEngineComparison(): Promise<DecisionEngineComparisonResponse> {
  const response = await axios.get<DecisionEngineComparisonResponse>(
    `${BASE_URL}/settings/compare`,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== GET /api/futures/decision-engine/strategies ====================
// List available strategies with metadata
export async function listDecisionEngineStrategies(): Promise<ListStrategiesResponse> {
  const response = await axios.get<ListStrategiesResponse>(
    `${BASE_URL}/strategies`,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== PUT /api/futures/decision-engine/strategy/:name ====================
// Update a specific strategy's settings
export async function updateStrategySettings(
  strategyName: StrategyName,
  settings: Partial<StrategySettings>
): Promise<{ success: boolean; message: string; strategy: StrategySettings }> {
  const response = await axios.put(
    `${BASE_URL}/strategy/${strategyName}`,
    settings,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/decision-engine/strategy/:name/reset ====================
// Reset a specific strategy to defaults
export async function resetStrategySettings(
  strategyName: StrategyName
): Promise<ResetDecisionSettingsResponse> {
  const response = await axios.post<ResetDecisionSettingsResponse>(
    `${BASE_URL}/strategy/${strategyName}/reset`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== PUT /api/futures/decision-engine/active-strategy ====================
// Set the active strategy
export async function setActiveStrategy(
  strategyName: StrategyName
): Promise<SetActiveStrategyResponse> {
  const request: SetActiveStrategyRequest = { strategy: strategyName };
  const response = await axios.put<SetActiveStrategyResponse>(
    `${BASE_URL}/active-strategy`,
    request,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/decision-engine/strategy/:name/enable ====================
// Enable a strategy
export async function enableStrategy(
  strategyName: StrategyName
): Promise<{ success: boolean; message: string }> {
  const response = await axios.post(
    `${BASE_URL}/strategy/${strategyName}/enable`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/decision-engine/strategy/:name/disable ====================
// Disable a strategy
export async function disableStrategy(
  strategyName: StrategyName
): Promise<{ success: boolean; message: string }> {
  const response = await axios.post(
    `${BASE_URL}/strategy/${strategyName}/disable`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== Convenience Functions ====================

// Reset all strategies to defaults
export async function resetAllStrategies(): Promise<ResetDecisionSettingsResponse> {
  return resetDecisionEngineSettings();
}

// Reset a specific group within a strategy
// Note: Group-level reset is not yet implemented on the backend.
// This function currently behaves the same as resetStrategySettings.
export async function resetStrategyGroup(
  strategyName: StrategyName,
  _groupName: string
): Promise<ResetDecisionSettingsResponse> {
  // Group-level reset not yet supported - reset entire strategy instead
  return resetDecisionEngineSettings({
    strategy: strategyName,
  });
}

// Update scoring config for a strategy
export async function updateScoringConfig(
  strategyName: StrategyName,
  scoring: Partial<StrategySettings['scoring']>
): Promise<{ success: boolean; message: string; strategy: StrategySettings }> {
  return updateStrategySettings(strategyName, { scoring } as Partial<StrategySettings>);
}

// Update blocking config for a strategy
export async function updateBlockingConfig(
  strategyName: StrategyName,
  blocking: Partial<StrategySettings['blocking']>
): Promise<{ success: boolean; message: string; strategy: StrategySettings }> {
  return updateStrategySettings(strategyName, { blocking } as Partial<StrategySettings>);
}

// Update entry conditions for a strategy
export async function updateEntryConditions(
  strategyName: StrategyName,
  entry: Partial<StrategySettings['entry']>
): Promise<{ success: boolean; message: string; strategy: StrategySettings }> {
  return updateStrategySettings(strategyName, { entry } as Partial<StrategySettings>);
}

// Update exit conditions for a strategy
export async function updateExitConditions(
  strategyName: StrategyName,
  exit: Partial<StrategySettings['exit']>
): Promise<{ success: boolean; message: string; strategy: StrategySettings }> {
  return updateStrategySettings(strategyName, { exit } as Partial<StrategySettings>);
}

// Toggle strategy enabled status
export async function toggleStrategy(
  strategyName: StrategyName,
  enabled: boolean
): Promise<{ success: boolean; message: string }> {
  return enabled ? enableStrategy(strategyName) : disableStrategy(strategyName);
}

// ============================================================================
// Story 11.26: Calibration Data Lifecycle API
// ============================================================================

// Calibration API Base URL
const CALIBRATION_BASE_URL = '/api/futures/calibration';

// Types for calibration API
export interface CalibrationConfidence {
  level: 'insufficient' | 'low' | 'medium' | 'high';
  total_trades: number;
  trades_needed: number;
  next_level: string;
  is_reliable: boolean;
  reliability_pct: number;
  bucket_breakdown: Record<string, number>;
}

export interface CalibrationHistoryRecord {
  id: number;
  user_id: string;
  strategy: string;
  indicator_hash: string;
  indicators: string[];
  archived_at: string;
  archive_reason: string;
  total_trades: number;
  buckets: Record<string, ScoreBucket>;
}

export interface ScoreBucket {
  total_trades: number;
  winning_trades: number;
  win_rate: number;
  total_pnl_percent: number;
  avg_pnl_percent: number;
  score_range: { min: number; max: number };
}

export interface ResetCalibrationRequest {
  strategy: string;
  indicator_hash?: string;
}

export interface GetCalibrationConfidenceResponse {
  success: boolean;
  strategy: string;
  indicator_hash: string;
  confidence: CalibrationConfidence;
}

export interface GetCalibrationDataResponse {
  success: boolean;
  strategy: string;
  indicator_hash: string;
  total_trades: number;
  confidence: CalibrationConfidence;
  buckets: Record<string, number>;
}

export interface GetCalibrationHistoryResponse {
  success: boolean;
  strategy: string;
  history: CalibrationHistoryRecord[];
}

// ==================== POST /api/futures/calibration/reset ====================
// Manually reset calibration data
export async function resetCalibration(
  request: ResetCalibrationRequest
): Promise<{ success: boolean; message: string }> {
  const response = await axios.post(
    `${CALIBRATION_BASE_URL}/reset`,
    request,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== GET /api/futures/calibration/confidence/:strategy ====================
// Get calibration confidence level for a strategy
export async function getCalibrationConfidence(
  strategy: string,
  indicatorHash?: string
): Promise<GetCalibrationConfidenceResponse> {
  const params = indicatorHash ? { indicator_hash: indicatorHash } : {};
  const response = await axios.get<GetCalibrationConfidenceResponse>(
    `${CALIBRATION_BASE_URL}/confidence/${strategy}`,
    { headers: getAuthHeaders(), params }
  );
  return response.data;
}

// ==================== GET /api/futures/calibration/data/:strategy ====================
// Get current calibration data for a strategy
export async function getCalibrationData(
  strategy: string,
  indicatorHash?: string
): Promise<GetCalibrationDataResponse> {
  const params = indicatorHash ? { indicator_hash: indicatorHash } : {};
  const response = await axios.get<GetCalibrationDataResponse>(
    `${CALIBRATION_BASE_URL}/data/${strategy}`,
    { headers: getAuthHeaders(), params }
  );
  return response.data;
}

// ==================== GET /api/futures/calibration/history/:strategy ====================
// Get calibration history for a strategy
export async function getCalibrationHistory(
  strategy: string
): Promise<GetCalibrationHistoryResponse> {
  const response = await axios.get<GetCalibrationHistoryResponse>(
    `${CALIBRATION_BASE_URL}/history/${strategy}`,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// Default export for convenience
const decisionEngineApi = {
  getSettings: getDecisionEngineSettings,
  updateSettings: updateDecisionEngineSettings,
  resetSettings: resetDecisionEngineSettings,
  getComparison: getDecisionEngineComparison,
  listStrategies: listDecisionEngineStrategies,
  updateStrategy: updateStrategySettings,
  resetStrategy: resetStrategySettings,
  setActiveStrategy,
  enableStrategy,
  disableStrategy,
  resetAllStrategies,
  resetStrategyGroup,
  updateScoringConfig,
  updateBlockingConfig,
  updateEntryConditions,
  updateExitConditions,
  toggleStrategy,
  // Calibration API (Story 11.26)
  resetCalibration,
  getCalibrationConfidence,
  getCalibrationData,
  getCalibrationHistory,
};

export default decisionEngineApi;
