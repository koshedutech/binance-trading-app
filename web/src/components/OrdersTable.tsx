import { useStore } from '../store';
import { X } from 'lucide-react';
import { apiService } from '../services/api';
import { formatDistanceToNow } from 'date-fns';

export default function OrdersTable() {
  const { activeOrders } = useStore();

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(value);
  };

  const handleCancelOrder = async (orderId: number) => {
    if (!confirm('Are you sure you want to cancel this order?')) {
      return;
    }

    try {
      await apiService.cancelOrder(orderId);
      alert('Order cancelled successfully');
    } catch (error) {
      alert('Failed to cancel order');
      console.error(error);
    }
  };

  if (activeOrders.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No active orders
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="table">
        <thead>
          <tr>
            <th>Symbol</th>
            <th>Type</th>
            <th>Side</th>
            <th>Price</th>
            <th>Quantity</th>
            <th>Status</th>
            <th>Age</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {activeOrders.map((order) => (
            <tr key={order.id}>
              <td className="font-medium">{order.symbol}</td>
              <td>
                <span className="badge badge-info">{order.order_type}</span>
              </td>
              <td>
                <span
                  className={`badge ${
                    order.side === 'BUY' ? 'badge-success' : 'badge-danger'
                  }`}
                >
                  {order.side}
                </span>
              </td>
              <td>{order.price ? formatCurrency(order.price) : 'MARKET'}</td>
              <td>
                {order.executed_qty > 0
                  ? `${order.executed_qty}/${order.quantity}`
                  : order.quantity}
              </td>
              <td>
                <span className="badge badge-warning">{order.status}</span>
              </td>
              <td className="text-sm">
                {formatDistanceToNow(new Date(order.created_at), { addSuffix: true })}
              </td>
              <td>
                <button
                  onClick={() => handleCancelOrder(order.id)}
                  className="btn-danger text-xs py-1 px-2 flex items-center space-x-1"
                >
                  <X className="w-3 h-3" />
                  <span>Cancel</span>
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
