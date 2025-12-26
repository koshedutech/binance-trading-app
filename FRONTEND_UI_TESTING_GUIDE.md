# Frontend UI Testing Guide - Custom ROI% Targets

## Access the Dashboard

**URL**: `http://localhost:8094`

---

## Expected UI Components

### 1. Futures Positions Table

Location: Main dashboard → Futures Autopilot section

#### Column Layout (Left to Right)
1. **Symbol** - Trading pair (e.g., AVAXUSDT, BTCUSDT)
2. **Side** - SHORT or LONG
3. **Entry Price** - Position entry price
4. **Qty** - Quantity
5. **Leverage** - Leverage multiplier (e.g., 5x)
6. **Unrealized PnL** - Current profit/loss in USD
7. **SL** - Stop Loss price
8. **TP** - Take Profit price levels
9. **🎯 ROI Target %** ← **NEW COLUMN**
10. **Actions** - Edit, Close buttons

#### ROI Target % Column Details

**Visual Indicator**:
- 🎯 Yellow TrendingUp icon
- Column header: "ROI Target %"

**Display States**:

**State 1: No Custom ROI (Default)**
```
       -
    (auto)
```
- Displays "-" on first line
- Gray "(auto)" label below
- Means: Using mode-based threshold (SWING=8%, SCALP=5%)

**State 2: Custom ROI Set**
```
     3.50%
   (custom)
```
- Displays ROI value in yellow text
- Yellow "(custom)" label below
- Example: Position set to close at 3.50% ROI

**State 3: Editing Mode**
```
┌─────────────┐
│    3.5      │  ← Input field
├─────────────┤
☑ Save       ← Checkbox
```
- Input field for ROI % (0-1000)
- "Save for future" checkbox below
- Save & Cancel buttons appear in Actions column

---

## UI Testing Steps

### Test 1: Verify ROI Target Column Visible

**Steps**:
1. Open dashboard at http://localhost:8094
2. Navigate to Futures section
3. Look for "🎯 ROI Target %" column header in yellow
4. Verify all positions show either "-" (auto) or a percentage value (custom)

**Expected Result**: Column is visible with proper styling ✓

---

### Test 2: Set Temporary ROI Target

**Steps**:
1. Find a position with positive unrealized PnL (e.g., AVAXUSDT)
2. Click the **yellow TrendingUp button** in the ROI Target column
3. Button changes to **edit mode**
4. A text input field appears for ROI %
5. Type a value (e.g., "3.5" for 3.5% ROI)
6. Leave "Save for future" **unchecked** (temporary)
7. Click **Save** button

**Expected Result**:
- Input field closes
- ROI Target cell shows "3.50%" with "(custom)" label
- Backend API receives POST request with success response
- Position will close when ROI reaches 3.5%

**Verification Command**:
```bash
curl -X POST http://localhost:8094/api/futures/ginie/positions/AVAXUSDT/roi-target \
  -H "Content-Type: application/json" \
  -d '{"roi_percent": 3.5, "save_for_future": false}'
```

---

### Test 3: Set Persistent ROI Target (Save for Future)

**Steps**:
1. Find a position (e.g., HYPEUSDT)
2. Click the **yellow TrendingUp button** in the ROI Target column
3. Enter ROI % value (e.g., "5" for 5%)
4. **CHECK** the "Save for future" checkbox
5. Click **Save** button

**Expected Result**:
- ROI Target cell shows "5.00%" with "(custom)" label
- Backend saves to `autopilot_settings.json`
- All future HYPEUSDT positions will inherit 5% ROI target
- Setting persists across server restarts

**Verification in Settings File**:
```bash
grep -A 5 '"HYPEUSDT"' "D:\Apps\binance-trading-bot\autopilot_settings.json" | grep custom_roi
```
Should output: `"custom_roi_percent": 5`

---

### Test 4: Edit Existing ROI Target

**Steps**:
1. Click on a position that already has a custom ROI target
2. The ROI Target cell enters edit mode
3. Current value appears in input field
4. Change the value (e.g., from 3.5 to 4.0)
5. Click **Save**

**Expected Result**:
- Value updates immediately
- API accepts the change
- Position closure threshold updates to new value

---

### Test 5: Clear Custom ROI Target

**Steps**:
1. Click on a position with custom ROI target
2. Clear the input field (delete value)
3. Uncheck "Save for future"
4. Click **Save**

**Expected Result**:
- ROI Target cell returns to "-" with "(auto)" label
- Position reverts to mode-based threshold
- No API error occurs

---

### Test 6: Cancel Edit

**Steps**:
1. Click ROI Target button on a position
2. Start entering a value
3. Click **Cancel** button (or press Escape)

**Expected Result**:
- Edit mode closes without changes
- Original ROI value restored
- No API call made

---

### Test 7: Position Closure at ROI Target

**Steps**:
1. Set ROI target to a value slightly above current ROI
2. Example: Current ROI = 4.1%, Set target = 4.3%
3. Wait for position monitoring cycle (~15 seconds)
4. Observe position in table

**Expected Result**:
- Position disappears from table (closed)
- NEW position appears in table for same symbol
- Confirms early profit booking triggered

---

## UI Styling Verification

### Color Scheme
- **Yellow (#FCD34D or similar)**: Active ROI targets and TrendingUp icon
- **Gray (text-gray-500)**: "auto" label for mode defaults
- **Cyan/Blue**: Edit field and input areas (matching existing TP/SL styling)

### Button States

**ROI Target Button (Normal)**:
- Yellow TrendingUp icon
- Background color: transparent with hover effect
- Tooltip: "Set ROI Target"

**ROI Target Button (Edit Mode)**:
- Button text: "Save" and "Cancel"
- Colors: Green (Save), Gray (Cancel)
- Located in Actions column

### Input Field

**Appearance**:
- White background with subtle border
- Gray border on focus
- Placeholder text: "ROI %"
- Spinner controls for increment/decrement
- Input type: number with step="0.1"

**Validation**:
- Minimum: 0
- Maximum: 1000
- Step: 0.1
- Red error message if outside range

---

## Responsive Design

### Desktop (> 1024px)
- ROI Target column visible in full width
- Edit mode displays inline with input field
- All elements properly spaced

### Tablet (768px - 1024px)
- Column headers may abbreviate to "ROI %"
- Edit controls stack vertically
- "Save for future" checkbox still visible

### Mobile (< 768px)
- Column may be hidden or scrollable
- Edit mode uses full width
- Touch-friendly button sizes

---

## Integration with Existing Features

### Compatibility with TP/SL Editing
- TP/SL editing and ROI editing use separate state variables
- Cannot edit both simultaneously
- Buttons properly hide/show based on active edit mode

### API Integration
- **Endpoint**: `POST /api/futures/ginie/positions/:symbol/roi-target`
- **Request Body**: `{ roi_percent: number, save_for_future: boolean }`
- **Response**: `{ success: boolean, message: string, symbol: string, roi_percent: number, save_for_future: boolean }`

### State Management
- React hooks manage edit state
- Loading state prevents double-submission
- Error handling displays user-friendly messages
- Auto-refresh after successful save

---

## Known UI Behaviors

### Auto-Refresh
- After setting ROI target, position list refreshes automatically
- Fetches latest data from backend
- Ensures UI stays in sync with server state

### Position Replacement
- When position closes at ROI target, it disappears from table
- New position may appear in its place (if autopilot is running)
- This is normal behavior - system is re-trading the symbol

### Edit State Persistence
- Edit state only in memory (not persisted to URL or storage)
- Closing browser loses edit state (expected)
- Position data itself is saved on backend

---

## Common Issues & Troubleshooting

### Issue: ROI Target column not visible
**Solution**:
- Refresh page (Ctrl+F5)
- Clear browser cache
- Verify frontend build is up to date: `npm run build`

### Issue: Can't click ROI Target button
**Solution**:
- Check if another edit mode is active (TP/SL editing)
- Click Cancel on other edit modes first
- Refresh page if unresponsive

### Issue: Input field won't accept values
**Solution**:
- Verify input is between 0-1000
- Check decimal format (use "." not ",")
- Try refreshing page and retry

### Issue: Save button doesn't work
**Solution**:
- Check browser console for errors (F12)
- Verify server is running: `curl http://localhost:8094/api/health`
- Check network tab for failed API requests
- Restart server if needed

### Issue: ROI Target shows but position doesn't close
**Solution**:
- ROI threshold may not be reached yet
- Market may be moving against position
- Check position's unrealized PnL vs ROI target
- Wait for next monitoring cycle (~15 seconds)

---

## UI Checklist

- [ ] ROI Target % column visible with yellow icon
- [ ] Display shows "-" (auto) for positions without custom ROI
- [ ] Display shows percentage (custom) for positions with custom ROI
- [ ] Click button opens edit mode with input field
- [ ] Input field accepts 0-1000 values
- [ ] "Save for future" checkbox visible and clickable
- [ ] Save button submits to backend
- [ ] Cancel button closes edit without changes
- [ ] Custom ROI persists on page refresh
- [ ] Position closes when ROI reaches target
- [ ] Styling matches dashboard theme (yellow for ROI)
- [ ] Responsive on different screen sizes
- [ ] No console errors in browser

---

## Testing Summary

**Total Test Cases**: 7 main tests
**Expected Pass Rate**: 100% ✓

**Quick Verification**:
1. Open http://localhost:8094
2. Look for yellow 🎯 ROI Target % column
3. Click button on any position
4. Enter "3.5" and uncheck "Save for future"
5. Click Save
6. Verify custom_roi_percent appears in position data

---

## Notes for Developer Testing

- All test steps above can be automated with Selenium/Cypress
- Screenshots of expected UI states available in test reports
- API responses are JSON serializable for validation
- State changes are atomic and idempotent

---

**Frontend UI Version**: 1.0
**Last Updated**: 2025-12-24
