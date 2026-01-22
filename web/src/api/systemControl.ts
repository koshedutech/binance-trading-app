// System Control API Client
// API functions for system-level control settings (order tracking, position management)

import axios from 'axios';

// Token storage key
const ACCESS_TOKEN_KEY = 'access_token';

// Base URL for system control API
const BASE_URL = '/api/settings/system-control';

// Helper to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// ==================== Types ====================

export type OrderTrackingSystem = 'chain' | 'legacy' | 'both';
export type PositionManagementSystem = 'legacy' | 'chain';
export type EntryDecisionSystem = 'legacy' | 'chain';

export interface SystemControlSettings {
  order_tracking_system: OrderTrackingSystem;
  position_management_system: PositionManagementSystem;
  entry_decision_system: EntryDecisionSystem;
  valid_order_tracking_systems: OrderTrackingSystem[];
  valid_position_management_systems: PositionManagementSystem[];
  valid_entry_decision_systems: EntryDecisionSystem[];
}

export interface UpdateSystemControlRequest {
  order_tracking_system?: OrderTrackingSystem;
  position_management_system?: PositionManagementSystem;
  entry_decision_system?: EntryDecisionSystem;
}

// ==================== GET /api/settings/system-control ====================
// Get current system control settings
export async function getSystemControl(): Promise<SystemControlSettings> {
  const response = await axios.get<SystemControlSettings>(
    BASE_URL,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== PUT /api/settings/system-control ====================
// Update system control settings
export async function updateSystemControl(
  settings: UpdateSystemControlRequest
): Promise<SystemControlSettings> {
  const response = await axios.put<SystemControlSettings>(
    BASE_URL,
    settings,
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// ==================== PUT /api/settings/system-control/entry-decision ====================
// Update only entry decision system
export async function updateEntryDecisionSystem(
  system: EntryDecisionSystem
): Promise<{ entry_decision_system: EntryDecisionSystem; message: string }> {
  const response = await axios.put<{ entry_decision_system: EntryDecisionSystem; message: string }>(
    `${BASE_URL}/entry-decision`,
    { system },
    { headers: getAuthHeaders() }
  );
  return response.data;
}

// Default export for convenience
const systemControlApi = {
  getSystemControl,
  updateSystemControl,
  updateEntryDecisionSystem,
};

export default systemControlApi;
