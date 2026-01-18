// Decision Engine Settings Component Tests
// Epic 11: Position Decision Engine - Story 11.24: Decision Engine Settings UI

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import DecisionEngineSettings from '../DecisionEngineSettings';
import * as decisionEngineApi from '../../api/decisionEngine';

// Mock the API module
vi.mock('../../api/decisionEngine', () => ({
  getDecisionEngineComparison: vi.fn(),
  resetDecisionEngineSettings: vi.fn(),
  resetStrategySettings: vi.fn(),
}));

// Mock comparison response
const mockComparisonResponse = {
  success: true,
  preview: true,
  config_type: 'decision_engine',
  all_match: false,
  total_strategies: 4,
  total_fields: 100,
  matching_fields: 85,
  total_differences: 15,
  strategies: {
    trend_following: {
      strategy_name: 'trend_following',
      enabled: true,
      total_fields: 25,
      matching_fields: 20,
      groups: {
        scoring: {
          group_name: 'scoring',
          total_fields: 5,
          matching_fields: 4,
          all_match: false,
          fields: [
            { path: 'technical_weight', current: 40, default: 40, match: true },
            { path: 'context_weight', current: 35, default: 30, match: false },
            { path: 'llm_weight', current: 20, default: 20, match: true },
            { path: 'history_weight', current: 10, default: 10, match: true },
            { path: 'min_threshold', current: 55, default: 55, match: true },
          ],
        },
        blocking: {
          group_name: 'blocking',
          total_fields: 5,
          matching_fields: 5,
          all_match: true,
          fields: [
            { path: 'adx_minimum', current: 25, default: 25, match: true },
            { path: 'score_minimum', current: 55, default: 55, match: true },
            { path: 'rsi_upper_limit', current: 70, default: 70, match: true },
            { path: 'rsi_lower_limit', current: 30, default: 30, match: true },
            { path: 'win_rate_minimum', current: 45, default: 45, match: true },
          ],
        },
      },
    },
    mean_reversion: {
      strategy_name: 'mean_reversion',
      enabled: false,
      total_fields: 25,
      matching_fields: 25,
      groups: {
        scoring: {
          group_name: 'scoring',
          total_fields: 5,
          matching_fields: 5,
          all_match: true,
          fields: [],
        },
      },
    },
    breakout: {
      strategy_name: 'breakout',
      enabled: true,
      total_fields: 25,
      matching_fields: 20,
      groups: {},
    },
    range_trading: {
      strategy_name: 'range_trading',
      enabled: false,
      total_fields: 25,
      matching_fields: 20,
      groups: {},
    },
  },
  active_strategy: {
    current: 'trend_following',
    default: 'trend_following',
    match: true,
  },
};

describe('DecisionEngineSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state initially', () => {
    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise(() => {})
    );

    render(<DecisionEngineSettings />);
    expect(screen.getByText(/Loading Decision Engine settings/i)).toBeInTheDocument();
  });

  it('renders settings after loading', async () => {
    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockResolvedValue(
      mockComparisonResponse
    );

    render(<DecisionEngineSettings />);

    await waitFor(() => {
      expect(screen.getByText('Decision Engine Settings')).toBeInTheDocument();
    });

    // Check strategy names are displayed
    expect(screen.getByText('Trend Following')).toBeInTheDocument();
    expect(screen.getByText('Mean Reversion')).toBeInTheDocument();
    expect(screen.getByText('Breakout')).toBeInTheDocument();
    expect(screen.getByText('Range Trading')).toBeInTheDocument();
  });

  it('shows comparison statistics', async () => {
    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockResolvedValue(
      mockComparisonResponse
    );

    render(<DecisionEngineSettings />);

    await waitFor(() => {
      expect(screen.getByText(/85\/100 fields match defaults/)).toBeInTheDocument();
    });

    // Should show differences badge
    expect(screen.getByText('15 Differences')).toBeInTheDocument();
  });

  it('shows active strategy', async () => {
    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockResolvedValue(
      mockComparisonResponse
    );

    render(<DecisionEngineSettings />);

    await waitFor(() => {
      expect(screen.getByText('Active Strategy:')).toBeInTheDocument();
      expect(screen.getByText('Trend Following')).toBeInTheDocument();
    });
  });

  it('renders error state on API failure', async () => {
    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('API Error')
    );

    render(<DecisionEngineSettings />);

    await waitFor(() => {
      expect(screen.getByText(/Error: API Error/i)).toBeInTheDocument();
    });

    // Should show retry button
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows Reset All button when there are differences', async () => {
    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockResolvedValue(
      mockComparisonResponse
    );

    render(<DecisionEngineSettings />);

    await waitFor(() => {
      expect(screen.getByText('Reset All Strategies')).toBeInTheDocument();
    });
  });

  it('hides Reset All button when all match', async () => {
    const allMatchResponse = {
      ...mockComparisonResponse,
      all_match: true,
      total_differences: 0,
      matching_fields: 100,
    };

    (decisionEngineApi.getDecisionEngineComparison as ReturnType<typeof vi.fn>).mockResolvedValue(
      allMatchResponse
    );

    render(<DecisionEngineSettings />);

    await waitFor(() => {
      expect(screen.getByText('All Match')).toBeInTheDocument();
    });

    expect(screen.queryByText('Reset All Strategies')).not.toBeInTheDocument();
  });
});
