# GiniePanel ROI Target Testing Guide

## Feature Added to GiniePanel

The per-position custom ROI% target feature has been added to the **GiniePanel's Positions tab** for consistency with the main Futures Positions Table.

---

## Access the GiniePanel

**Location**: Dashboard → Ginie AI Trading Panel → Positions Tab

---

## UI Components

### Position Card Display

Each position in the GiniePanel is displayed as a **card** (not a table row).

#### Card Header (Always Visible)
- **Symbol**: Trading pair name (e.g., AVA, BTC)
- **Side**: LONG or SHORT (color-coded)
- **Mode**: SCA (Scalp), SWI (Swing), POS (Position)
- **Source Badge**: AI or Strategy
- **Trailing Badge**: TRAIL (if active)
- **🎯 ROI Target Badge** ← **NEW**: Shows custom ROI if set, yellow background
- **PnL**: Realized + Unrealized PnL with percentage

#### Expanded View (Click to Expand)
When expanded, shows a 5-column grid:
1. **Entry**: Entry price
2. **Qty**: Remaining/Original quantity
3. **SL**: Stop Loss price
4. **Leverage**: Leverage multiplier
5. **🎯 ROI Target** ← **NEW COLUMN**: ROI Target display and editor

---

## ROI Target Column in Expanded View

### Display States

**State 1: No Custom ROI (Default)**
```
🎯 ROI Target
    -
```
- Displays "-" in gray text
- Clicking opens edit mode
- Position uses mode-based threshold (SWING=8%, SCALP=5%)

**State 2: Custom ROI Set**
```
🎯 ROI Target
  3.50%
```
- Displays ROI value in yellow text
- Clicking opens edit mode to change value
- Position will close when ROI reaches this threshold

**State 3: Editing Mode**
```
🎯 ROI Target
┌──────────┐
│   3.5    │  ← Input field
├──────────┤
│ Save  Cancel  ← Buttons
└──────────┘
```
- Input field for entering ROI % (0-1000)
- Save button: Submits to backend (green)
- Cancel button: Closes edit without saving (gray)

---

## Testing Steps

### Test 1: Verify ROI Target Column Visible in GiniePanel

**Steps**:
1. Open dashboard at http://localhost:8094
2. Navigate to Ginie AI Trading Panel (usually on left panel)
3. Click on **Positions** tab
4. Click on any position card to expand it
5. Look for the 5-column grid in the expanded view
6. Verify **🎯 ROI Target** column is visible in the 5th position

**Expected Result**: Column displays "-" for positions without custom ROI ✓

---

### Test 2: Set ROI Target on Ginie Position

**Steps**:
1. Find an open position in GiniePanel Positions tab
2. Expand the position card (click on it)
3. Locate the **🎯 ROI Target** column on the right
4. Click the "-" (or existing value) to enter edit mode
5. Type a value (e.g., "3.5" for 3.5% ROI)
6. Click **Save** button
7. Observe the edit mode closing

**Expected Result**:
- Edit mode closes
- ROI Target shows "3.50%" in yellow
- Badge appears in card header: "ROI: 3.50%"
- Position will close when ROI reaches 3.5%

**Verification Command**:
```bash
curl -X POST http://localhost:8094/api/futures/ginie/positions/AVAXUSDT/roi-target \
  -H "Content-Type: application/json" \
  -d '{"roi_percent": 3.5, "save_for_future": false}'
```

---

### Test 3: Update Existing ROI Target

**Steps**:
1. Click on a position with existing ROI target
2. Expand the position card
3. Click the ROI Target value to edit
4. Change the value (e.g., from 3.5 to 4.0)
5. Click **Save**

**Expected Result**:
- Value updates immediately
- Header badge updates to new value
- New ROI target takes effect

---

### Test 4: Clear ROI Target

**Steps**:
1. Click on a position with custom ROI target
2. Expand the position card
3. Click the ROI Target value to edit
4. Delete the input value (clear field)
5. Click **Save**

**Expected Result**:
- ROI Target reverts to "-"
- Badge disappears from header
- Position uses mode-based threshold

---

### Test 5: Cancel Edit

**Steps**:
1. Click on a position
2. Expand the position card
3. Click the ROI Target value
4. Start typing a new value
5. Click **Cancel** button

**Expected Result**:
- Edit mode closes without saving
- Original ROI value displayed
- No API call made

---

## Visual Design

### Color Scheme
- **Yellow**: ROI target values and indicators (#FBBF24)
- **Gray**: Default state ("-")
- **Green**: Save button
- **Gray**: Cancel button

### Header Badge (When ROI Set)
```
ROI: 3.50%  (yellow background, dark text)
```

### Input Field
- Gray background
- White text
- Subtle border
- Spinner controls for increment/decrement
- Number type with step="0.1"

### Buttons (In Edit Mode)
- **Save**: Green button (bg-green-600), full width
- **Cancel**: Gray button (bg-gray-600), full width
- Buttons stack vertically below input

---

## Differences from FuturesPositionsTable

| Feature | FuturesPositionsTable | GiniePanel | Notes |
|---------|----------------------|-----------|-------|
| Display Format | Table rows | Card format | GiniePanel uses expandable cards |
| ROI Column | Fixed column | In expanded view | Only visible when card is expanded |
| Header Badge | N/A | Shows custom ROI | Quick visual indicator |
| Edit Trigger | Yellow button | Click on value | More direct click-to-edit |
| "Save for future" | Yes | No | GiniePanel always temporary |
| Auto-refresh | Yes (full table) | Card only | Efficient local update |

---

## Interaction Flow

### Setting Custom ROI in GiniePanel

1. **Expand Position Card**
   ```
   Click on position card
   ↓
   Card expands to show 5-column grid
   ```

2. **Edit ROI Target**
   ```
   Click on ROI Target value
   ↓
   Input field appears with current value (or empty)
   ```

3. **Enter Value**
   ```
   Type ROI % (0-1000)
   ↓
   Value appears in input field
   ```

4. **Submit**
   ```
   Click Save button
   ↓
   API call sent to backend
   ↓
   Edit mode closes
   ↓
   New value displays in yellow
   ↓
   Header badge updates
   ```

---

## API Integration

### Endpoint Called
```
POST /api/futures/ginie/positions/:symbol/roi-target
```

### Request Body
```json
{
  "roi_percent": 3.5,
  "save_for_future": false
}
```

### Response Expected
```json
{
  "success": true,
  "message": "Custom ROI target set for AVAXUSDT",
  "symbol": "AVAXUSDT",
  "roi_percent": 3.5,
  "save_for_future": false
}
```

---

## State Management

The ROI editing state is **local to the PositionCard component**:
- `editingROI`: Boolean - tracks if edit mode is active
- `roiValue`: String - holds input field value
- `savingROI`: Boolean - shows loading state during save

State is **not persisted** across component unmounting:
- Collapsing and re-expanding the card resets edit state
- Browser navigation clears edit state
- Page refresh clears edit state

---

## Expected Behaviors

### Position Closes at ROI Target
- When position's ROI reaches or exceeds the custom target
- `monitorAllPositions()` check runs every ~15 seconds
- Position closes automatically
- New position may open if autopilot is active

### ROI Target vs Default Threshold
- **Custom ROI**: Always takes priority when set
- **Mode Default**: Used when no custom ROI set (SWING=8%, SCALP=5%)
- **Settings**: Can be saved to symbol settings (future positions)

### Badge Visibility
- Badge shows only when custom ROI is set
- Format: "ROI: X.XX%"
- Yellow background, bold font
- Appears in position card header for quick visibility

---

## Testing Checklist

- [ ] ROI Target column visible in expanded position view
- [ ] Display shows "-" for positions without custom ROI
- [ ] Display shows "X.XX%" for positions with custom ROI
- [ ] Click on ROI Target opens edit mode
- [ ] Input field accepts 0-1000 values
- [ ] Save button submits to backend
- [ ] Cancel button closes without saving
- [ ] Custom ROI persists on page refresh
- [ ] Badge shows in position header when ROI set
- [ ] Position closes when ROI target reached
- [ ] Styling matches theme (yellow for ROI)
- [ ] Responsive on different screen sizes
- [ ] No console errors when editing ROI
- [ ] Error handling for invalid values
- [ ] Loading state shown while saving

---

## Troubleshooting

### Issue: ROI Target column not visible
**Solution**:
- Expand the position card (click on it)
- Ensure frontend build is up to date
- Refresh page with Ctrl+F5

### Issue: Edit mode won't open
**Solution**:
- Make sure position card is expanded first
- Try clicking directly on the ROI value
- Refresh page if still stuck

### Issue: Save button disabled
**Solution**:
- Wait for previous save to complete (shows "Saving...")
- Check browser console for errors
- Verify server is running

### Issue: Value shows error message
**Solution**:
- Ensure value is between 0-1000
- Use decimal notation (3.5 not 3,5)
- Try clearing and re-entering value

---

## Notes

- ROI targets in GiniePanel are **always temporary** (not saved for future)
- Use FuturesPositionsTable's "Save for future" feature for persistent settings
- GiniePanel is designed for quick monitoring and adjustment
- Both GiniePanel and FuturesPositionsTable share the same backend API
- Changes in one UI are immediately visible in the other

---

## Summary

The GiniePanel now includes full ROI Target editing capability in the expanded position view. Users can quickly set custom ROI targets for any position without navigating to the FuturesPositionsTable. The feature integrates seamlessly with the existing GiniePanel card layout and position monitoring system.

---

**Frontend Version**: 1.0
**Build Date**: 2025-12-24
**Bundle Size**: 906.82 KB (227.38 KB gzipped)
