import { useState, useEffect } from 'react';
import { CheckCircle2, XCircle } from 'lucide-react';
import SignalCard from './SignalCard';
import { apiService } from '../services/api';
import type { EnhancedPendingSignal } from '../types';

export default function EnhancedSignalsPanel() {
  const [activeTab, setActiveTab] = useState<'confirmed' | 'rejected'>('confirmed');
  const [confirmedSignals, setConfirmedSignals] = useState<EnhancedPendingSignal[]>([]);
  const [rejectedSignals, setRejectedSignals] = useState<EnhancedPendingSignal[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchSignals = async () => {
    try {
      const [confirmed, rejected] = await Promise.all([
        apiService.getPendingSignalsByStatus('CONFIRMED', 50),
        apiService.getPendingSignalsByStatus('REJECTED', 50),
      ]);
      setConfirmedSignals(confirmed);
      setRejectedSignals(rejected);
    } catch (error) {
      console.error('Failed to fetch signals:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSignals();
    const interval = setInterval(fetchSignals, 15000); // Reduced from 5s to 15s to avoid rate limits
    return () => clearInterval(interval);
  }, []);

  const handleExecute = async (signal: EnhancedPendingSignal) => {
    try {
      // Execute the signal by calling the backend
      await apiService.confirmPendingSignal(signal.id, 'CONFIRM');
      alert('Signal execution initiated!');
      fetchSignals();
    } catch (error) {
      console.error('Failed to execute signal:', error);
      alert('Failed to execute signal');
    }
  };

  const handleDuplicate = async (signal: EnhancedPendingSignal) => {
    try {
      await apiService.duplicatePendingSignal(signal.id);
      alert('Signal duplicated and added to pending!');
      fetchSignals();
    } catch (error) {
      console.error('Failed to duplicate signal:', error);
      alert('Failed to duplicate signal');
    }
  };

  const handleArchive = async (signal: EnhancedPendingSignal) => {
    try {
      await apiService.archivePendingSignal(signal.id);
      alert('Signal archived');
      fetchSignals();
    } catch (error) {
      console.error('Failed to archive signal:', error);
      alert('Failed to archive signal');
    }
  };

  const handleDelete = async (signal: EnhancedPendingSignal) => {
    if (!confirm('Are you sure you want to permanently delete this signal?')) return;

    try {
      await apiService.deletePendingSignal(signal.id);
      alert('Signal deleted');
      fetchSignals();
    } catch (error) {
      console.error('Failed to delete signal:', error);
      alert('Failed to delete signal');
    }
  };

  const currentSignals = activeTab === 'confirmed' ? confirmedSignals : rejectedSignals;

  return (
    <div className="space-y-4">
      {/* Tabs */}
      <div className="flex gap-2 border-b border-dark-700">
        <button
          onClick={() => setActiveTab('confirmed')}
          className={`px-4 py-2 flex items-center gap-2 font-semibold transition-colors border-b-2 ${
            activeTab === 'confirmed'
              ? 'border-green-500 text-green-500'
              : 'border-transparent text-gray-400 hover:text-gray-300'
          }`}
        >
          <CheckCircle2 className="w-4 h-4" />
          Confirmed Signals ({confirmedSignals.length})
        </button>

        <button
          onClick={() => setActiveTab('rejected')}
          className={`px-4 py-2 flex items-center gap-2 font-semibold transition-colors border-b-2 ${
            activeTab === 'rejected'
              ? 'border-red-500 text-red-500'
              : 'border-transparent text-gray-400 hover:text-gray-300'
          }`}
        >
          <XCircle className="w-4 h-4" />
          Rejected Signals ({rejectedSignals.length})
        </button>
      </div>

      {/* Signals List */}
      <div className="space-y-3 max-h-[600px] overflow-y-auto scrollbar-thin">
        {loading ? (
          <div className="text-center py-8 text-gray-400">Loading signals...</div>
        ) : currentSignals.length === 0 ? (
          <div className="text-center py-8 text-gray-400">
            No {activeTab} signals
          </div>
        ) : (
          currentSignals.map((signal) => (
            <SignalCard
              key={signal.id}
              signal={signal}
              onExecute={handleExecute}
              onDuplicate={handleDuplicate}
              onArchive={handleArchive}
              onDelete={handleDelete}
            />
          ))
        )}
      </div>
    </div>
  );
}
