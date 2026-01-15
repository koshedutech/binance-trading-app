import { useState } from 'react';
import { createPortal } from 'react-dom';
import {
  Server,
  Activity,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Loader2,
  RefreshCw,
  Power,
  Wifi,
  WifiOff,
  ChevronUp,
  ChevronDown,
} from 'lucide-react';
import {
  useInstanceStatus,
  useTakeControl,
  useReleaseControl,
  InstanceStatus,
} from '../hooks/useInstanceControl';

// Toast notification component
interface ToastProps {
  message: string;
  type: 'success' | 'error' | 'info';
  onClose: () => void;
}

function Toast({ message, type, onClose }: ToastProps) {
  const bgColor = type === 'success' ? 'bg-green-500' : type === 'error' ? 'bg-red-500' : 'bg-blue-500';
  const Icon = type === 'success' ? CheckCircle : type === 'error' ? XCircle : Activity;

  return createPortal(
    <div className={`fixed bottom-4 right-4 ${bgColor} text-white px-4 py-3 rounded-lg shadow-lg flex items-center gap-3 z-50 animate-slide-up`}>
      <Icon className="w-5 h-5" />
      <span>{message}</span>
      <button onClick={onClose} className="ml-2 hover:opacity-80">
        &times;
      </button>
    </div>,
    document.body
  );
}

// Confirmation dialog component
interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  cancelLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}

function ConfirmDialog({
  isOpen,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Cancel',
  onConfirm,
  onCancel,
  isLoading,
}: ConfirmDialogProps) {
  if (!isOpen) return null;

  return createPortal(
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-gray-800 rounded-lg border border-gray-700 max-w-md w-full mx-4 p-6 shadow-xl">
        <div className="flex items-center gap-3 mb-4">
          <AlertTriangle className="w-6 h-6 text-yellow-500" />
          <h3 className="text-lg font-semibold text-white">{title}</h3>
        </div>
        <p className="text-gray-300 mb-6">{description}</p>
        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            disabled={isLoading}
            className="px-4 py-2 bg-gray-700 text-gray-200 rounded-lg hover:bg-gray-600 transition-colors disabled:opacity-50"
          >
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            disabled={isLoading}
            className="px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isLoading && <Loader2 className="w-4 h-4 animate-spin" />}
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}

// Progress indicator for takeover
interface TakeoverProgressProps {
  waitSeconds: number;
  message: string;
}

function TakeoverProgress({ waitSeconds, message }: TakeoverProgressProps) {
  return (
    <div className="mt-4 bg-gray-900 rounded-lg p-3">
      <div className="flex items-center gap-3 mb-2">
        <Loader2 className="w-5 h-5 animate-spin text-primary-500" />
        <span className="text-sm text-gray-300">{message}</span>
      </div>
      <div className="w-full bg-gray-700 rounded-full h-2 overflow-hidden">
        <div
          className="h-full bg-primary-500 animate-pulse"
          style={{ width: '50%' }}
        />
      </div>
      {waitSeconds > 0 && (
        <p className="text-xs text-gray-500 mt-2 text-center">
          Estimated wait: ~{waitSeconds} seconds
        </p>
      )}
    </div>
  );
}

// Format instance ID for display
function formatInstanceId(id: string): string {
  return id.toUpperCase();
}

// Format time ago
function formatTimeAgo(timestamp: string): string {
  if (!timestamp || timestamp === '0001-01-01T00:00:00Z') return 'Never';
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);

  if (diffSecs < 5) return 'Just now';
  if (diffSecs < 60) return `${diffSecs}s ago`;
  const diffMins = Math.floor(diffSecs / 60);
  if (diffMins < 60) return `${diffMins}m ago`;
  const diffHours = Math.floor(diffMins / 60);
  return `${diffHours}h ${diffMins % 60}m ago`;
}

export default function InstanceControlPanel() {
  const { data: status, isLoading, error, refetch } = useInstanceStatus();
  const takeControl = useTakeControl();
  const releaseControl = useReleaseControl();

  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' | 'info' } | null>(null);

  // Handle take control action
  const handleTakeControl = async () => {
    setShowConfirmDialog(false);
    try {
      const result = await takeControl.mutate({ force: false });
      if (result.success) {
        setToast({ message: result.message || 'Control transferred successfully', type: 'success' });
        refetch();
      } else {
        setToast({ message: result.message || 'Failed to take control', type: 'error' });
      }
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to take control';
      setToast({ message, type: 'error' });
    }
  };

  // Handle release control action
  const handleReleaseControl = async () => {
    try {
      const result = await releaseControl.mutate();
      if (result.success) {
        setToast({ message: result.message || 'Control released successfully', type: 'success' });
        refetch();
      } else {
        setToast({ message: result.message || 'Failed to release control', type: 'error' });
      }
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to release control';
      setToast({ message, type: 'error' });
    }
  };

  // Loading state
  if (isLoading && !status) {
    return (
      <div className="bg-gray-800 rounded-lg border border-gray-700 p-4 mb-4">
        <div className="flex items-center gap-3">
          <Loader2 className="w-5 h-5 animate-spin text-gray-400" />
          <span className="text-gray-400">Loading instance status...</span>
        </div>
      </div>
    );
  }

  // Error state
  if (error && !status) {
    return (
      <div className="bg-gray-800 rounded-lg border border-red-500/30 p-4 mb-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3 text-red-500">
            <XCircle className="w-5 h-5" />
            <span className="text-sm">{error}</span>
          </div>
          <button
            onClick={refetch}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Retry"
          >
            <RefreshCw className="w-4 h-4 text-gray-400" />
          </button>
        </div>
      </div>
    );
  }

  const isActive = status?.is_active ?? false;
  const instanceId = status?.instance_id ?? 'unknown';
  const activeInstance = status?.active_instance ?? 'unknown';
  const otherAlive = status?.other_alive ?? false;
  const canTakeControl = status?.can_take_control ?? false;
  const lastHeartbeat = status?.last_heartbeat ?? '';

  return (
    <>
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden mb-4">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
          <div className="flex items-center gap-2">
            <Server className="w-5 h-5 text-primary-500" />
            <span className="font-semibold text-white">Instance Control</span>
          </div>
          <button
            onClick={refetch}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Refresh status"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
        </div>

        {/* Content */}
        <div className="p-4">
          {/* Instance info row */}
          <div className="grid grid-cols-2 gap-4 mb-4">
            {/* This Instance */}
            <div>
              <span className="text-xs text-gray-500 uppercase tracking-wide">This Instance</span>
              <div className="flex items-center gap-2 mt-1">
                <Server className="w-4 h-4 text-gray-400" />
                <span className="text-lg font-bold text-white">{formatInstanceId(instanceId)}</span>
              </div>
            </div>

            {/* Status Badge */}
            <div>
              <span className="text-xs text-gray-500 uppercase tracking-wide">Status</span>
              <div className="mt-1">
                {isActive ? (
                  <span className="inline-flex items-center gap-1.5 px-3 py-1 bg-green-500/20 text-green-500 rounded-full text-sm font-medium">
                    <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                    ACTIVE - Trading
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 px-3 py-1 bg-gray-600/50 text-gray-300 rounded-full text-sm font-medium">
                    <span className="w-2 h-2 bg-gray-500 rounded-full" />
                    STANDBY - Monitoring
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Additional info */}
          <div className="grid grid-cols-2 gap-4 mb-4">
            {/* Active Instance */}
            {!isActive && (
              <div className="bg-gray-900 rounded-lg p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Activity className="w-4 h-4 text-green-500" />
                  <span className="text-xs text-gray-400">Active Instance</span>
                </div>
                <span className="text-sm font-medium text-white">{formatInstanceId(activeInstance)}</span>
              </div>
            )}

            {/* Other Instance Status */}
            <div className={`bg-gray-900 rounded-lg p-3 ${isActive ? 'col-span-2' : ''}`}>
              <div className="flex items-center gap-2 mb-1">
                {otherAlive ? (
                  <Wifi className="w-4 h-4 text-green-500" />
                ) : (
                  <WifiOff className="w-4 h-4 text-red-500" />
                )}
                <span className="text-xs text-gray-400">Other Instance</span>
              </div>
              <div className="flex items-center gap-2">
                <span className={`text-sm font-medium ${otherAlive ? 'text-green-500' : 'text-red-500'}`}>
                  {otherAlive ? 'Online' : 'Not Responding'}
                </span>
                {lastHeartbeat && otherAlive && (
                  <span className="text-xs text-gray-500">
                    (heartbeat: {formatTimeAgo(lastHeartbeat)})
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Action Button */}
          <div className="mt-4">
            {isActive ? (
              <button
                onClick={handleReleaseControl}
                disabled={releaseControl.isLoading}
                className="w-full flex items-center justify-center gap-2 px-4 py-3 bg-gray-700 hover:bg-gray-600 text-gray-200 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {releaseControl.isLoading ? (
                  <Loader2 className="w-5 h-5 animate-spin" />
                ) : (
                  <Power className="w-5 h-5" />
                )}
                <span className="font-medium">
                  {releaseControl.isLoading ? 'Releasing...' : 'Release Control'}
                </span>
              </button>
            ) : (
              <button
                onClick={() => setShowConfirmDialog(true)}
                disabled={!canTakeControl || takeControl.isLoading}
                className="w-full flex items-center justify-center gap-2 px-4 py-3 bg-primary-500 hover:bg-primary-600 text-white rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                title={!canTakeControl ? 'Cannot take control at this time' : undefined}
              >
                {takeControl.isLoading ? (
                  <Loader2 className="w-5 h-5 animate-spin" />
                ) : (
                  <Power className="w-5 h-5" />
                )}
                <span className="font-medium">
                  {takeControl.isLoading ? 'Taking Control...' : 'Take Control'}
                </span>
              </button>
            )}
          </div>

          {/* Takeover progress indicator */}
          {takeControl.isLoading && (
            <TakeoverProgress
              waitSeconds={takeControl.data?.wait_seconds || 0}
              message="Waiting for other instance to release control..."
            />
          )}

          {/* Error message */}
          {(takeControl.error || releaseControl.error) && (
            <div className="mt-3 p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
              <div className="flex items-center gap-2 text-red-500 text-sm">
                <AlertTriangle className="w-4 h-4 flex-shrink-0" />
                <span>{takeControl.error || releaseControl.error}</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Confirmation Dialog */}
      <ConfirmDialog
        isOpen={showConfirmDialog}
        title="Take Control?"
        description={`This will transfer trading control from ${formatInstanceId(activeInstance)} to this instance (${formatInstanceId(instanceId)}). The other instance will complete any current operations before releasing control.`}
        confirmLabel="Take Control"
        onConfirm={handleTakeControl}
        onCancel={() => setShowConfirmDialog(false)}
        isLoading={takeControl.isLoading}
      />

      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </>
  );
}
