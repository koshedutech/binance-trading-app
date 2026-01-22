// Mode-Strategy API Client
// Epic 11: Position Decision Engine - Story 11.33: Mode-Strategy UI Update

import axios from 'axios';
import type {
  ModeName,
  StrategyName,
  ModeConfig,
  ModeStrategyConfig,
  GetModeStrategiesResponse,
  GetModeStrategyResponse,
  UpdateModeStrategyRequest,
  UpdateModeStrategyResponse,
  ResetModeStrategyResponse,
  StrategyComparisonResponse,
} from '../types/modeStrategy';

// Token storage key
const ACCESS_TOKEN_KEY = 'access_token';

// Base URL for mode strategy API
const BASE_URL = '/api/futures/modes';

// Helper to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// ==================== GET /api/futures/modes/:mode/strategies ====================
// Get all strategies for a mode
// Backend returns: { mode: string, strategies: ModeStrategyResponse[] }
// We transform it to ModeConfig format for UI consumption
export async function getModeStrategies(mode: ModeName): Promise<ModeConfig> {
  const response = await axios.get<GetModeStrategiesResponse>(
    `${BASE_URL}/${mode}/strategies`,
    { headers: getAuthHeaders() }
  );

  // Transform backend response to ModeConfig format
  const strategies: Record<StrategyName, ModeStrategyConfig> = {} as Record<StrategyName, ModeStrategyConfig>;

  for (const stratResp of response.data.strategies) {
    const strategyName = stratResp.strategy as StrategyName;
    strategies[strategyName] = {
      enabled: stratResp.enabled,
      priority: stratResp.priority,
      supported_regimes: stratResp.supported_regimes,
      leverage: stratResp.settings.leverage,
      max_positions: stratResp.settings.max_positions,
      base_size_usd: stratResp.settings.base_size_usd,
      timeframe: stratResp.settings.timeframe,
      sltp: stratResp.settings.sltp,
      confidence: stratResp.settings.confidence,
      entry_conditions: stratResp.settings.entry_conditions || {},
      exit_conditions: stratResp.settings.exit_conditions,
      scoring: stratResp.settings.scoring,
    };
  }

  // Return ModeConfig structure
  return {
    name: mode,
    enabled: true, // Mode-level enabled is managed elsewhere
    default_strategy: 'trend_following',
    auto_select_strategy: true,
    strategies,
  };
}

// ==================== GET /api/futures/modes/:mode/strategies/:strategy ====================
// Get a specific strategy configuration
// Backend returns ModeStrategyResponse directly
export async function getModeStrategy(
  mode: ModeName,
  strategy: StrategyName
): Promise<ModeStrategyConfig> {
  const response = await axios.get<GetModeStrategyResponse>(
    `${BASE_URL}/${mode}/strategies/${strategy}`,
    { headers: getAuthHeaders() }
  );

  // Transform backend response to ModeStrategyConfig format
  const stratResp = response.data;
  return {
    enabled: stratResp.enabled,
    priority: stratResp.priority,
    supported_regimes: stratResp.supported_regimes,
    leverage: stratResp.settings.leverage,
    max_positions: stratResp.settings.max_positions,
    base_size_usd: stratResp.settings.base_size_usd,
    timeframe: stratResp.settings.timeframe,
    sltp: stratResp.settings.sltp,
    confidence: stratResp.settings.confidence,
    entry_conditions: stratResp.settings.entry_conditions || {},
    exit_conditions: stratResp.settings.exit_conditions,
    scoring: stratResp.settings.scoring,
  };
}

// ==================== PUT /api/futures/modes/:mode/strategies/:strategy ====================
// Update a specific strategy configuration
// Backend expects: { enabled?, priority?, supported_regimes?, settings: {...} }
// Settings must be nested inside a "settings" object
export async function updateModeStrategy(
  mode: ModeName,
  strategy: StrategyName,
  config: UpdateModeStrategyRequest | ModeStrategyConfig
): Promise<UpdateModeStrategyResponse> {
  // Transform flat ModeStrategyConfig to backend-expected format
  // Backend expects settings fields nested inside a "settings" object
  const requestBody: {
    enabled?: boolean;
    priority?: number;
    supported_regimes?: string[];
    settings: Record<string, unknown>;
  } = {
    settings: {},
  };

  // Handle enabled/priority/supported_regimes at top level
  if ('enabled' in config && config.enabled !== undefined) {
    requestBody.enabled = config.enabled;
  }
  if ('priority' in config && config.priority !== undefined) {
    requestBody.priority = config.priority;
  }
  if ('supported_regimes' in config && config.supported_regimes !== undefined) {
    requestBody.supported_regimes = config.supported_regimes;
  }

  // All other settings go inside the "settings" object
  if ('leverage' in config && config.leverage !== undefined) {
    requestBody.settings.leverage = config.leverage;
  }
  if ('max_positions' in config && config.max_positions !== undefined) {
    requestBody.settings.max_positions = config.max_positions;
  }
  if ('base_size_usd' in config && config.base_size_usd !== undefined) {
    requestBody.settings.base_size_usd = config.base_size_usd;
  }
  if ('timeframe' in config && config.timeframe !== undefined) {
    requestBody.settings.timeframe = config.timeframe;
  }
  if ('sltp' in config && config.sltp !== undefined) {
    requestBody.settings.sltp = config.sltp;
  }
  if ('confidence' in config && config.confidence !== undefined) {
    requestBody.settings.confidence = config.confidence;
  }
  if ('entry_conditions' in config && config.entry_conditions !== undefined) {
    requestBody.settings.entry_conditions = config.entry_conditions;
  }
  if ('exit_conditions' in config && config.exit_conditions !== undefined) {
    requestBody.settings.exit_conditions = config.exit_conditions;
  }
  if ('scoring' in config && config.scoring !== undefined) {
    requestBody.settings.scoring = config.scoring;
  }

  const response = await axios.put<UpdateModeStrategyResponse>(
    `${BASE_URL}/${mode}/strategies/${strategy}`,
    requestBody,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/modes/:mode/strategies/:strategy/reset ====================
// Reset a specific strategy to defaults
export async function resetModeStrategy(
  mode: ModeName,
  strategy: StrategyName
): Promise<ResetModeStrategyResponse> {
  const response = await axios.post<ResetModeStrategyResponse>(
    `${BASE_URL}/${mode}/strategies/${strategy}/reset`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/modes/:mode/strategies/:strategy/enable ====================
// Enable a strategy
export async function enableModeStrategy(
  mode: ModeName,
  strategy: StrategyName
): Promise<{ success: boolean; message: string }> {
  const response = await axios.post(
    `${BASE_URL}/${mode}/strategies/${strategy}/enable`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/modes/:mode/strategies/:strategy/disable ====================
// Disable a strategy
export async function disableModeStrategy(
  mode: ModeName,
  strategy: StrategyName
): Promise<{ success: boolean; message: string }> {
  const response = await axios.post(
    `${BASE_URL}/${mode}/strategies/${strategy}/disable`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== GET /api/futures/modes/:mode/strategies/:strategy/compare ====================
// Compare strategy settings with defaults
export async function compareModeStrategy(
  mode: ModeName,
  strategy: StrategyName
): Promise<StrategyComparisonResponse> {
  const response = await axios.get<StrategyComparisonResponse>(
    `${BASE_URL}/${mode}/strategies/${strategy}/compare`,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== POST /api/futures/modes/:mode/reset-all ====================
// Reset all strategies in a mode to defaults
export async function resetAllModeStrategies(
  mode: ModeName
): Promise<{ success: boolean; message: string; strategies_reset: number }> {
  const response = await axios.post(
    `${BASE_URL}/${mode}/reset-all`,
    {},
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== PUT /api/futures/modes/:mode ====================
// Update mode-level settings (enabled, default_strategy, auto_select)
export async function updateModeSettings(
  mode: ModeName,
  settings: {
    enabled?: boolean;
    default_strategy?: StrategyName;
    auto_select_strategy?: boolean;
  }
): Promise<{ success: boolean; message: string; mode: ModeConfig }> {
  const response = await axios.put(
    `${BASE_URL}/${mode}`,
    settings,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== Convenience Functions ====================

// Toggle strategy enabled status
export async function toggleModeStrategy(
  mode: ModeName,
  strategy: StrategyName,
  enabled: boolean
): Promise<{ success: boolean; message: string }> {
  return enabled
    ? enableModeStrategy(mode, strategy)
    : disableModeStrategy(mode, strategy);
}

// Update SLTP settings for a strategy
export async function updateStrategySLTP(
  mode: ModeName,
  strategy: StrategyName,
  sltp: UpdateModeStrategyRequest['sltp']
): Promise<UpdateModeStrategyResponse> {
  return updateModeStrategy(mode, strategy, { sltp });
}

// Update confidence settings for a strategy
export async function updateStrategyConfidence(
  mode: ModeName,
  strategy: StrategyName,
  confidence: UpdateModeStrategyRequest['confidence']
): Promise<UpdateModeStrategyResponse> {
  return updateModeStrategy(mode, strategy, { confidence });
}

// Update entry conditions for a strategy
export async function updateStrategyEntryConditions(
  mode: ModeName,
  strategy: StrategyName,
  entry_conditions: UpdateModeStrategyRequest['entry_conditions']
): Promise<UpdateModeStrategyResponse> {
  return updateModeStrategy(mode, strategy, { entry_conditions });
}

// Update exit conditions for a strategy
export async function updateStrategyExitConditions(
  mode: ModeName,
  strategy: StrategyName,
  exit_conditions: UpdateModeStrategyRequest['exit_conditions']
): Promise<UpdateModeStrategyResponse> {
  return updateModeStrategy(mode, strategy, { exit_conditions });
}

// Update scoring weights for a strategy
export async function updateStrategyScoring(
  mode: ModeName,
  strategy: StrategyName,
  scoring: UpdateModeStrategyRequest['scoring']
): Promise<UpdateModeStrategyResponse> {
  return updateModeStrategy(mode, strategy, { scoring });
}

// Update position settings (leverage, max_positions, base_size)
export async function updateStrategyPosition(
  mode: ModeName,
  strategy: StrategyName,
  position: {
    leverage?: number;
    max_positions?: number;
    base_size_usd?: number;
  }
): Promise<UpdateModeStrategyResponse> {
  return updateModeStrategy(mode, strategy, position);
}

// Default export for convenience
const modeStrategyApi = {
  getModeStrategies,
  getModeStrategy,
  updateModeStrategy,
  resetModeStrategy,
  resetAllModeStrategies,
  enableModeStrategy,
  disableModeStrategy,
  compareModeStrategy,
  updateModeSettings,
  toggleModeStrategy,
  updateStrategySLTP,
  updateStrategyConfidence,
  updateStrategyEntryConditions,
  updateStrategyExitConditions,
  updateStrategyScoring,
  updateStrategyPosition,
};

export default modeStrategyApi;
