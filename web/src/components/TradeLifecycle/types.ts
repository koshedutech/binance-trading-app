// Trade Lifecycle Tab - Types for Order Chain Visualization
// Epic 7: Client Order ID & Trade Lifecycle Tracking

// Order type suffixes used in client order IDs
export type OrderTypeSuffix = 'E' | 'TP1' | 'TP2' | 'TP3' | 'RB' | 'DCA1' | 'DCA2' | 'DCA3' | 'H' | 'HSL' | 'HTP' | 'SL';

// Trading mode codes
export type TradingModeCode = 'ULT' | 'SCA' | 'SWI' | 'POS';

// Mapping from mode code to display name
export const MODE_DISPLAY_NAMES: Record<TradingModeCode, string> = {
  ULT: 'Ultra Fast',
  SCA: 'Scalp',
  SWI: 'Swing',
  POS: 'Position',
};

// Order type display configuration
export const ORDER_TYPE_CONFIG: Record<OrderTypeSuffix, { label: string; color: string; bgColor: string; description: string }> = {
  E: { label: 'Entry', color: 'text-green-400', bgColor: 'bg-green-500/20', description: 'Initial entry order' },
  TP1: { label: 'TP1', color: 'text-cyan-400', bgColor: 'bg-cyan-500/20', description: 'Take Profit Level 1' },
  TP2: { label: 'TP2', color: 'text-cyan-400', bgColor: 'bg-cyan-500/20', description: 'Take Profit Level 2' },
  TP3: { label: 'TP3', color: 'text-cyan-400', bgColor: 'bg-cyan-500/20', description: 'Take Profit Level 3' },
  RB: { label: 'Rebuy', color: 'text-purple-400', bgColor: 'bg-purple-500/20', description: 'Position re-entry' },
  DCA1: { label: 'DCA1', color: 'text-blue-400', bgColor: 'bg-blue-500/20', description: 'Dollar Cost Average 1' },
  DCA2: { label: 'DCA2', color: 'text-blue-400', bgColor: 'bg-blue-500/20', description: 'Dollar Cost Average 2' },
  DCA3: { label: 'DCA3', color: 'text-blue-400', bgColor: 'bg-blue-500/20', description: 'Dollar Cost Average 3' },
  H: { label: 'Hedge', color: 'text-yellow-400', bgColor: 'bg-yellow-500/20', description: 'Hedge position' },
  HSL: { label: 'Hedge SL', color: 'text-orange-400', bgColor: 'bg-orange-500/20', description: 'Hedge Stop Loss' },
  HTP: { label: 'Hedge TP', color: 'text-teal-400', bgColor: 'bg-teal-500/20', description: 'Hedge Take Profit' },
  SL: { label: 'SL', color: 'text-red-400', bgColor: 'bg-red-500/20', description: 'Stop Loss' },
};

// Parsed client order ID structure
export interface ParsedClientOrderId {
  raw: string;
  modeCode: TradingModeCode | null;
  dateStr: string | null;
  sequence: number | null;
  orderType: OrderTypeSuffix | null;
  chainId: string | null;
  isFallback: boolean;
  isValid: boolean;
}

// Chain order (individual order within a chain)
export interface ChainOrder {
  orderId: number;
  clientOrderId: string;
  symbol: string;
  side: 'BUY' | 'SELL';
  positionSide: 'LONG' | 'SHORT' | 'BOTH';
  type: string;
  status: string;
  price: number;
  avgPrice?: number;
  origQty: number;
  executedQty: number;
  stopPrice?: number;
  time: number;
  updateTime: number;
  orderType: OrderTypeSuffix | null;
  parsed: ParsedClientOrderId;
}

// Order chain (group of related orders)
export interface OrderChain {
  chainId: string;
  modeCode: TradingModeCode | null;
  dateStr: string | null;
  sequence: number | null;
  symbol: string | null;
  side: 'BUY' | 'SELL' | null;
  positionSide: 'LONG' | 'SHORT' | 'BOTH' | null;
  orders: ChainOrder[];
  entryOrder: ChainOrder | null;
  tpOrders: ChainOrder[];
  slOrder: ChainOrder | null;
  dcaOrders: ChainOrder[];
  rebuyOrder: ChainOrder | null;
  hedgeOrder: ChainOrder | null;
  hedgeSLOrder: ChainOrder | null;
  hedgeTPOrder: ChainOrder | null;
  status: 'active' | 'partial' | 'completed' | 'cancelled';
  totalValue: number;
  filledValue: number;
  pnl?: number;
  createdAt: number;
  updatedAt: number;
  isFallback: boolean;
}

// Filter options for chains
export interface ChainFilters {
  mode: TradingModeCode | 'all';
  status: 'all' | 'active' | 'partial' | 'completed' | 'cancelled';
  symbol: string | 'all';
  side: 'all' | 'LONG' | 'SHORT';
  dateFrom?: string;
  dateTo?: string;
}

// Parse client order ID from string
export function parseClientOrderId(clientOrderId: string): ParsedClientOrderId {
  const result: ParsedClientOrderId = {
    raw: clientOrderId,
    modeCode: null,
    dateStr: null,
    sequence: null,
    orderType: null,
    chainId: null,
    isFallback: false,
    isValid: false,
  };

  if (!clientOrderId) return result;

  const parts = clientOrderId.split('-');
  if (parts.length < 3) return result;

  // Check if valid mode code (case-insensitive)
  const modeCode = parts[0].toUpperCase() as TradingModeCode;
  if (!['ULT', 'SCA', 'SWI', 'POS'].includes(modeCode)) return result;
  result.modeCode = modeCode;

  // Check for fallback format: MODE-FALLBACK-UUID-TYPE (case-insensitive FALLBACK check)
  if (parts[1].toUpperCase() === 'FALLBACK') {
    result.isFallback = true;
    if (parts.length >= 4) {
      // Normalize chainId to uppercase for consistent grouping
      result.chainId = `${modeCode}-FALLBACK-${parts[2]}`;
      const orderType = parts[3].toUpperCase() as OrderTypeSuffix;
      result.orderType = ORDER_TYPE_CONFIG[orderType] ? orderType : null;
      result.isValid = result.orderType !== null;
    }
    return result;
  }

  // Normal format: MODE-DDMMM-NNNNN-TYPE
  if (parts.length >= 4) {
    result.dateStr = parts[1].toUpperCase(); // Normalize date string
    const seqNum = parseInt(parts[2], 10);
    if (!isNaN(seqNum)) {
      result.sequence = seqNum;
    }
    // Normalize chainId to uppercase for consistent grouping
    result.chainId = `${modeCode}-${result.dateStr}-${parts[2]}`;
    const orderType = parts[3].toUpperCase() as OrderTypeSuffix;
    result.orderType = ORDER_TYPE_CONFIG[orderType] ? orderType : null;
    result.isValid = result.orderType !== null;
  }

  return result;
}

// Extract chain ID from client order ID
export function extractChainId(clientOrderId: string): string | null {
  const parsed = parseClientOrderId(clientOrderId);
  return parsed.chainId;
}

// Group orders into chains
export function groupOrdersIntoChains(orders: ChainOrder[]): OrderChain[] {
  const chainMap = new Map<string, OrderChain>();

  for (const order of orders) {
    // Use already-parsed data from order.parsed (avoid duplicate parsing)
    const parsed = order.parsed;
    if (!parsed.chainId) continue;

    let chain = chainMap.get(parsed.chainId);
    if (!chain) {
      chain = {
        chainId: parsed.chainId,
        modeCode: parsed.modeCode,
        dateStr: parsed.dateStr,
        sequence: parsed.sequence,
        symbol: order.symbol,
        side: order.side,
        positionSide: order.positionSide as 'LONG' | 'SHORT' | 'BOTH',
        orders: [],
        entryOrder: null,
        tpOrders: [],
        slOrder: null,
        dcaOrders: [],
        rebuyOrder: null,
        hedgeOrder: null,
        hedgeSLOrder: null,
        hedgeTPOrder: null,
        status: 'active',
        totalValue: 0,
        filledValue: 0,
        createdAt: order.time,
        updatedAt: order.updateTime,
        isFallback: parsed.isFallback,
      };
      chainMap.set(parsed.chainId, chain);
    }

    // Add order to chain (already has parsed info)
    chain.orders.push(order);

    // Categorize order by type
    switch (parsed.orderType) {
      case 'E':
        chain.entryOrder = order;
        break;
      case 'TP1':
      case 'TP2':
      case 'TP3':
        chain.tpOrders.push(order);
        break;
      case 'SL':
        chain.slOrder = order;
        break;
      case 'DCA1':
      case 'DCA2':
      case 'DCA3':
        chain.dcaOrders.push(order);
        break;
      case 'RB':
        chain.rebuyOrder = order;
        break;
      case 'H':
        chain.hedgeOrder = order;
        break;
      case 'HSL':
        chain.hedgeSLOrder = order;
        break;
      case 'HTP':
        chain.hedgeTPOrder = order;
        break;
    }

    // Update chain metadata
    chain.totalValue += order.origQty * order.price;
    chain.filledValue += order.executedQty * (order.avgPrice || order.price);
    chain.createdAt = Math.min(chain.createdAt, order.time);
    chain.updatedAt = Math.max(chain.updatedAt, order.updateTime);
  }

  // Determine chain status
  for (const chain of chainMap.values()) {
    const allCancelled = chain.orders.every(o => o.status === 'CANCELED');
    const allFilled = chain.orders.every(o => o.status === 'FILLED');
    const someFilled = chain.orders.some(o => o.status === 'FILLED');

    if (allCancelled) {
      chain.status = 'cancelled';
    } else if (allFilled) {
      chain.status = 'completed';
    } else if (someFilled) {
      chain.status = 'partial';
    } else {
      chain.status = 'active';
    }

    // Sort TP orders
    chain.tpOrders.sort((a, b) => {
      const aNum = parseInt(a.orderType?.replace('TP', '') || '0');
      const bNum = parseInt(b.orderType?.replace('TP', '') || '0');
      return aNum - bNum;
    });

    // Sort DCA orders
    chain.dcaOrders.sort((a, b) => {
      const aNum = parseInt(a.orderType?.replace('DCA', '') || '0');
      const bNum = parseInt(b.orderType?.replace('DCA', '') || '0');
      return aNum - bNum;
    });
  }

  // Sort chains by creation time (newest first)
  return Array.from(chainMap.values()).sort((a, b) => b.createdAt - a.createdAt);
}
