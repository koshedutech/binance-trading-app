# Story 4: Frontend UI - Paper Balance Settings

**Story ID:** PAPER-004
**Epic:** Editable Paper Trading Balance
**Priority:** High
**Estimated Effort:** 5 hours
**Author:** Bob (Scrum Master)
**Status:** Ready for Development

---

## Description

Implement paper balance management UI in the Settings page. Display balance controls only when user is in paper trading mode. Support manual balance entry, sync from Binance, and separate controls for Spot vs Futures trading types.

---

## User Story

> As a trader using paper trading mode,
> I want to edit my paper balance and sync it from my real Binance account via the Settings UI,
> So that I can easily configure my testing environment without technical knowledge.

---

## Acceptance Criteria

### AC4.1: Conditional Visibility - Paper Mode Only

- [ ] Paper balance section visible ONLY when `dry_run_mode === true`
- [ ] Section hidden when `dry_run_mode === false` (real trading mode)
- [ ] Check mode for BOTH Spot and Futures independently
- [ ] No console errors when toggling between modes

**UI Behavior:**
```
User enables Paper Trading (dry_run_mode = true)
  → Paper Balance section appears

User disables Paper Trading (dry_run_mode = false)
  → Paper Balance section disappears
```

---

### AC4.2: Manual Balance Input Field

- [ ] Input field displays current paper balance (e.g., "$10,000.00")
- [ ] Input accepts numbers with optional commas and decimals (e.g., "5,000.50", "5000", "5000.5")
- [ ] Input validates range client-side: $10 minimum, $1,000,000 maximum
- [ ] Show red border + error text if value out of range
- [ ] Format value on blur: "5000" → "$5,000.00"
- [ ] Disable submit during validation errors

**Validation Error Messages:**
- Below minimum: "Balance must be at least $10"
- Above maximum: "Balance cannot exceed $1,000,000"
- Invalid format: "Please enter a valid number"

---

### AC4.3: Sync from Real Balance Button

- [ ] Button labeled: "Sync from Real Balance" with 🔄 icon
- [ ] Button visible next to balance input field
- [ ] Button states:
  - **Default:** Blue, enabled (if API keys configured)
  - **Disabled:** Gray, tooltip "Configure Binance API keys first" (if no API keys)
  - **Loading:** Blue, spinner animation, text "Syncing..."
  - **Success:** Green flash (2 seconds), checkmark icon, text "Synced!"
  - **Error:** Red, text "Sync Failed - Check API Keys"
- [ ] Click triggers POST to `/api/settings/sync-paper-balance/:trading_type`
- [ ] On success, update balance input field with synced value
- [ ] On error, show error toast notification

---

### AC4.4: Success & Error Toast Notifications

**Success Cases:**
- [ ] Manual update success: "Paper balance updated to $5,000.00"
- [ ] Sync success: "Paper balance synced from Binance: $3,547.82"

**Error Cases:**
- [ ] Validation error: "Balance must be between $10 and $1,000,000"
- [ ] API error: "Failed to update balance. Please try again."
- [ ] No API keys: "Binance API credentials not configured. Add them in Settings."
- [ ] Binance API failure: "Failed to sync from Binance. Please check your connection."

**Toast Requirements:**
- [ ] Display for 4 seconds (success) or 6 seconds (error)
- [ ] Dismissible with X button
- [ ] Max 1 toast visible at a time (queue if multiple)
- [ ] ARIA live region for screen reader accessibility

---

### AC4.5: Separate Controls for Spot vs Futures

- [ ] Two independent balance sections (or tabbed interface)
- [ ] **Spot Trading Balance:** Editable when Spot dry_run_mode = true
- [ ] **Futures Trading Balance:** Editable when Futures dry_run_mode = true
- [ ] Each section has its own input + sync button
- [ ] Updating one does NOT affect the other
- [ ] Clear visual distinction (labels, borders, or tabs)

**Suggested UI Layout:**
```
┌─────────────────────────────────────────────┐
│ Paper Trading Settings                      │
├─────────────────────────────────────────────┤
│                                             │
│ ┌─────────────────────────────────────────┐ │
│ │ 📊 Spot Trading Balance                 │ │
│ │                                         │ │
│ │ Current Balance: [$5,000.00  ]  [Save] │ │
│ │ [🔄 Sync from Real Balance]             │ │
│ └─────────────────────────────────────────┘ │
│                                             │
│ ┌─────────────────────────────────────────┐ │
│ │ 📈 Futures Trading Balance              │ │
│ │                                         │ │
│ │ Current Balance: [$10,000.00 ]  [Save] │ │
│ │ [🔄 Sync from Real Balance]             │ │
│ └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

---

## Technical Implementation Notes

### File Structure

```
web/src/
├── components/
│   ├── PaperBalanceSection.tsx          ← NEW: Main component
│   └── PaperBalanceInput.tsx            ← NEW: Reusable input component
├── pages/
│   └── Settings.tsx                     ← MODIFY: Add paper balance section
├── services/
│   └── paperBalanceService.ts           ← NEW: API client methods
└── contexts/
    └── TradingConfigContext.tsx         ← MODIFY: Add paper balance state
```

---

### Component: PaperBalanceSection.tsx

```tsx
import React, { useState, useEffect } from 'react';
import { paperBalanceService } from '../services/paperBalanceService';
import { useToast } from '../hooks/useToast';

interface Props {
  tradingType: 'spot' | 'futures';
  dryRunMode: boolean;
}

export const PaperBalanceSection: React.FC<Props> = ({ tradingType, dryRunMode }) => {
  const [balance, setBalance] = useState<string>('');
  const [inputValue, setInputValue] = useState<string>('');
  const [isEditing, setIsEditing] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { showToast } = useToast();

  useEffect(() => {
    if (dryRunMode) {
      fetchBalance();
    }
  }, [dryRunMode, tradingType]);

  const fetchBalance = async () => {
    try {
      const response = await paperBalanceService.getPaperBalance(tradingType);
      const formattedBalance = formatCurrency(response.paper_balance_usdt);
      setBalance(formattedBalance);
      setInputValue(formattedBalance);
    } catch (err) {
      console.error('Failed to fetch paper balance', err);
    }
  };

  const handleUpdate = async () => {
    const numericValue = parseBalance(inputValue);

    if (numericValue < 10 || numericValue > 1000000) {
      setError('Balance must be between $10 and $1,000,000');
      return;
    }

    try {
      await paperBalanceService.updatePaperBalance(tradingType, numericValue);
      showToast(`Paper balance updated to ${formatCurrency(numericValue)}`, 'success');
      setBalance(formatCurrency(numericValue));
      setIsEditing(false);
      setError(null);
    } catch (err) {
      showToast('Failed to update balance. Please try again.', 'error');
    }
  };

  const handleSync = async () => {
    setIsSyncing(true);
    try {
      const response = await paperBalanceService.syncPaperBalance(tradingType);
      const syncedBalance = formatCurrency(response.paper_balance_usdt);
      setBalance(syncedBalance);
      setInputValue(syncedBalance);
      showToast(`Paper balance synced from Binance: ${syncedBalance}`, 'success');
    } catch (err: any) {
      if (err.response?.status === 400) {
        showToast('Binance API credentials not configured. Add them in Settings.', 'error');
      } else {
        showToast('Failed to sync from Binance. Please check your connection.', 'error');
      }
    } finally {
      setIsSyncing(false);
    }
  };

  const formatCurrency = (value: string | number): string => {
    const num = typeof value === 'string' ? parseFloat(value) : value;
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(num);
  };

  const parseBalance = (value: string): number => {
    return parseFloat(value.replace(/[$,]/g, ''));
  };

  if (!dryRunMode) return null; // AC4.1: Only show in paper mode

  return (
    <div className="paper-balance-section">
      <h3>{tradingType === 'spot' ? '📊 Spot' : '📈 Futures'} Trading Balance</h3>

      <div className="balance-input-group">
        <label htmlFor={`balance-${tradingType}`}>Current Balance:</label>
        <input
          id={`balance-${tradingType}`}
          type="text"
          value={inputValue}
          onChange={(e) => {
            setInputValue(e.target.value);
            setError(null);
          }}
          onBlur={() => {
            const num = parseBalance(inputValue);
            if (!isNaN(num)) {
              setInputValue(formatCurrency(num));
            }
          }}
          className={error ? 'input-error' : ''}
          aria-label={`${tradingType} paper trading balance`}
        />
        <button onClick={handleUpdate} disabled={!!error}>
          Save
        </button>
      </div>

      {error && <p className="error-text">{error}</p>}

      <button
        className="sync-button"
        onClick={handleSync}
        disabled={isSyncing}
        aria-label={`Sync ${tradingType} balance from Binance`}
      >
        {isSyncing ? (
          <>
            <span className="spinner" /> Syncing...
          </>
        ) : (
          <>
            🔄 Sync from Real Balance
          </>
        )}
      </button>
    </div>
  );
};
```

---

### Service: paperBalanceService.ts

```typescript
import api from './api'; // Existing API client

interface PaperBalanceResponse {
  trading_type: string;
  paper_balance_usdt: string;
  dry_run_mode: boolean;
  message?: string;
}

export const paperBalanceService = {
  getPaperBalance: async (tradingType: 'spot' | 'futures'): Promise<PaperBalanceResponse> => {
    const response = await api.get(`/api/settings/paper-balance/${tradingType}`);
    return response.data;
  },

  updatePaperBalance: async (tradingType: 'spot' | 'futures', balance: number): Promise<void> => {
    await api.put(`/api/settings/paper-balance/${tradingType}`, { balance });
  },

  syncPaperBalance: async (tradingType: 'spot' | 'futures'): Promise<PaperBalanceResponse> => {
    const response = await api.post(`/api/settings/sync-paper-balance/${tradingType}`);
    return response.data;
  },
};
```

---

### Update: Settings.tsx

```tsx
import React from 'react';
import { PaperBalanceSection } from '../components/PaperBalanceSection';
import { useTradingConfig } from '../contexts/TradingConfigContext';

export const Settings: React.FC = () => {
  const { spotConfig, futuresConfig } = useTradingConfig();

  return (
    <div className="settings-page">
      <h1>Settings</h1>

      {/* Existing settings sections... */}

      {/* Paper Trading Balance Section */}
      <section className="paper-balance-settings">
        <h2>Paper Trading Balances</h2>
        <p className="description">
          Customize your paper trading balance to match your real account or test different scenarios.
        </p>

        <PaperBalanceSection
          tradingType="spot"
          dryRunMode={spotConfig.dry_run_mode}
        />

        <PaperBalanceSection
          tradingType="futures"
          dryRunMode={futuresConfig.dry_run_mode}
        />
      </section>
    </div>
  );
};
```

---

### CSS Styling (example)

```css
.paper-balance-section {
  border: 1px solid #ddd;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
  background-color: #f9f9f9;
}

.paper-balance-section h3 {
  margin-top: 0;
  color: #333;
}

.balance-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.balance-input-group input {
  flex: 1;
  padding: 8px;
  font-size: 16px;
  border: 1px solid #ccc;
  border-radius: 4px;
}

.balance-input-group input.input-error {
  border-color: #e74c3c;
}

.error-text {
  color: #e74c3c;
  font-size: 14px;
  margin-top: -8px;
  margin-bottom: 8px;
}

.sync-button {
  background-color: #3498db;
  color: white;
  padding: 10px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.sync-button:disabled {
  background-color: #95a5a6;
  cursor: not-allowed;
}

.sync-button .spinner {
  border: 2px solid #f3f3f3;
  border-top: 2px solid #3498db;
  border-radius: 50%;
  width: 16px;
  height: 16px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
```

---

## Testing Requirements

### Unit Tests

**File:** `web/src/components/PaperBalanceSection.test.tsx`

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { PaperBalanceSection } from './PaperBalanceSection';
import { paperBalanceService } from '../services/paperBalanceService';

jest.mock('../services/paperBalanceService');

describe('PaperBalanceSection', () => {
  test('renders when dry_run_mode is true', () => {
    render(<PaperBalanceSection tradingType="spot" dryRunMode={true} />);
    expect(screen.getByText(/Spot Trading Balance/i)).toBeInTheDocument();
  });

  test('does not render when dry_run_mode is false', () => {
    const { container } = render(<PaperBalanceSection tradingType="spot" dryRunMode={false} />);
    expect(container.firstChild).toBeNull();
  });

  test('validates minimum balance ($10)', async () => {
    render(<PaperBalanceSection tradingType="futures" dryRunMode={true} />);
    const input = screen.getByLabelText(/futures paper trading balance/i);

    fireEvent.change(input, { target: { value: '5' } });
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => {
      expect(screen.getByText(/Balance must be at least \$10/i)).toBeInTheDocument();
    });
  });

  test('validates maximum balance ($1M)', async () => {
    render(<PaperBalanceSection tradingType="spot" dryRunMode={true} />);
    const input = screen.getByLabelText(/spot paper trading balance/i);

    fireEvent.change(input, { target: { value: '1000001' } });
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => {
      expect(screen.getByText(/Balance cannot exceed \$1,000,000/i)).toBeInTheDocument();
    });
  });

  test('calls API on save with valid balance', async () => {
    paperBalanceService.updatePaperBalance = jest.fn().mockResolvedValue({});

    render(<PaperBalanceSection tradingType="spot" dryRunMode={true} />);
    const input = screen.getByLabelText(/spot paper trading balance/i);

    fireEvent.change(input, { target: { value: '5000' } });
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => {
      expect(paperBalanceService.updatePaperBalance).toHaveBeenCalledWith('spot', 5000);
    });
  });

  test('sync button triggers API call', async () => {
    paperBalanceService.syncPaperBalance = jest.fn().mockResolvedValue({
      paper_balance_usdt: '3547.82',
      trading_type: 'futures',
    });

    render(<PaperBalanceSection tradingType="futures" dryRunMode={true} />);
    fireEvent.click(screen.getByText(/Sync from Real Balance/i));

    await waitFor(() => {
      expect(paperBalanceService.syncPaperBalance).toHaveBeenCalledWith('futures');
    });
  });

  test('displays error toast on sync failure', async () => {
    paperBalanceService.syncPaperBalance = jest.fn().mockRejectedValue({
      response: { status: 502 },
    });

    render(<PaperBalanceSection tradingType="spot" dryRunMode={true} />);
    fireEvent.click(screen.getByText(/Sync from Real Balance/i));

    await waitFor(() => {
      expect(screen.getByText(/Failed to sync from Binance/i)).toBeInTheDocument();
    });
  });
});
```

---

### E2E Tests (Cypress or Playwright)

```javascript
describe('Paper Balance Settings', () => {
  beforeEach(() => {
    cy.login('test@example.com', 'xxxxxxxx');
    cy.visit('/settings');
  });

  it('shows paper balance section only in paper mode', () => {
    // Enable paper mode
    cy.contains('Enable Paper Trading').click();
    cy.contains('Spot Trading Balance').should('be.visible');

    // Disable paper mode
    cy.contains('Enable Paper Trading').click();
    cy.contains('Spot Trading Balance').should('not.exist');
  });

  it('updates balance manually', () => {
    cy.contains('Enable Paper Trading').click();

    cy.get('input[aria-label*="spot paper trading balance"]')
      .clear()
      .type('7500');

    cy.contains('Save').click();

    cy.contains('Paper balance updated to $7,500.00').should('be.visible');
  });

  it('syncs balance from Binance', () => {
    cy.intercept('POST', '/api/settings/sync-paper-balance/futures', {
      statusCode: 200,
      body: {
        trading_type: 'futures',
        paper_balance_usdt: '4250.75',
        message: 'Synced successfully',
      },
    });

    cy.contains('Enable Paper Trading (Futures)').click();
    cy.contains('Sync from Real Balance').click();

    cy.contains('Paper balance synced from Binance: $4,250.75').should('be.visible');
  });

  it('shows error for invalid balance', () => {
    cy.contains('Enable Paper Trading').click();

    cy.get('input[aria-label*="spot paper trading balance"]')
      .clear()
      .type('5'); // Below minimum

    cy.contains('Save').click();

    cy.contains('Balance must be at least $10').should('be.visible');
  });
});
```

---

### Manual Testing Checklist

- [ ] Paper balance section hidden when paper mode disabled
- [ ] Paper balance section visible when paper mode enabled
- [ ] Input field displays current balance on load
- [ ] Input accepts various formats (5000, 5,000, 5000.50)
- [ ] Input formats to currency on blur ($5,000.00)
- [ ] Validation error shows for balance < $10
- [ ] Validation error shows for balance > $1M
- [ ] Save button updates balance successfully
- [ ] Success toast appears on save
- [ ] Sync button shows loading state during API call
- [ ] Sync button updates balance on success
- [ ] Error toast appears on sync failure (test with invalid API keys)
- [ ] Spot and Futures balances independent (update one, verify other unchanged)
- [ ] Page responsive on mobile (320px width)

---

## Dependencies

### Prerequisites
- **Story 2:** Backend API endpoints implemented and accessible
- **Story 3:** Trading logic updated (for end-to-end verification)

### Blocks
- None (final story in epic)

---

## Definition of Done

- [ ] All acceptance criteria met (AC4.1 - AC4.5)
- [ ] Component renders conditionally based on dry_run_mode
- [ ] Manual balance input working with validation
- [ ] Sync button functional with all states (default, loading, success, error)
- [ ] Toast notifications working for success and error cases
- [ ] Separate controls for Spot and Futures implemented
- [ ] All unit tests passing (>80% coverage)
- [ ] E2E tests passing
- [ ] Manual testing checklist completed
- [ ] Responsive design verified (mobile and desktop)
- [ ] Accessibility verified (keyboard navigation, screen readers)
- [ ] Code review approved
- [ ] No console errors or warnings

---

## Accessibility Requirements

- [ ] All inputs have proper `aria-label` attributes
- [ ] Error messages associated with inputs via `aria-describedby`
- [ ] Toast notifications use ARIA live regions (`role="alert"`)
- [ ] Sync button has descriptive aria-label
- [ ] Keyboard navigation works (Tab, Enter, Escape)
- [ ] Focus indicators visible on all interactive elements
- [ ] Color contrast meets WCAG AA standards (4.5:1 minimum)

---

## Browser Compatibility

Test on:
- [ ] Chrome (latest)
- [ ] Firefox (latest)
- [ ] Safari (latest)
- [ ] Edge (latest)
- [ ] Mobile Safari (iOS 14+)
- [ ] Chrome Mobile (Android)

---

## Notes for Developer

- **API Client:** Use existing `api` service from `web/src/services/api.ts`
- **Toast Hook:** If project doesn't have `useToast`, create one or use existing notification system
- **Formatting:** Use `Intl.NumberFormat` for locale-aware currency formatting
- **State Management:** Consider using React Context if trading config state needs sharing across components
- **Loading States:** Disable sync button during API call to prevent double-submission

---

## Related Stories

- **Story 1:** Database migration (foundational)
- **Story 2:** Backend API endpoints (prerequisite)
- **Story 3:** Trading logic update (parallel/verification)
