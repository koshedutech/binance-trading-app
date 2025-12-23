import React, { useState, useEffect } from 'react';
import { CreditCard, Check, TrendingUp, Calendar, Download, AlertCircle, Crown, Zap, Rocket, Star } from 'lucide-react';
import { useAuth, TIER_INFO } from '../contexts/AuthContext';
import { apiService } from '../services/api';

interface ProfitPeriod {
  id: string;
  period_start: string;
  period_end: string;
  starting_balance: number;
  ending_balance: number;
  gross_profit: number;
  net_profit: number;
  profit_share_rate: number;
  profit_share_due: number;
  settlement_status: string;
  created_at: string;
}

interface Invoice {
  id: string;
  amount: number;
  status: string;
  created_at: string;
  pdf_url?: string;
}

const tiers = [
  {
    id: 'free',
    name: 'Free',
    icon: Star,
    price: 0,
    profitShare: 30,
    maxPositions: 3,
    color: 'gray',
    features: [
      'Up to 3 open positions',
      'Spot trading only',
      'Basic strategies',
      'Community support',
    ],
  },
  {
    id: 'trader',
    name: 'Trader',
    icon: Zap,
    price: 49,
    profitShare: 20,
    maxPositions: 10,
    color: 'blue',
    features: [
      'Up to 10 open positions',
      'Spot + Futures trading',
      'Advanced strategies',
      'Email support',
      'AI signal recommendations',
    ],
  },
  {
    id: 'pro',
    name: 'Pro',
    icon: Rocket,
    price: 149,
    profitShare: 12,
    maxPositions: 25,
    color: 'purple',
    popular: true,
    features: [
      'Up to 25 open positions',
      'All trading modes',
      'Custom strategies',
      'Priority support',
      'AI autopilot mode',
      'Advanced analytics',
    ],
  },
  {
    id: 'whale',
    name: 'Whale',
    icon: Crown,
    price: 499,
    profitShare: 5,
    maxPositions: -1, // unlimited
    color: 'yellow',
    features: [
      'Unlimited positions',
      'All features included',
      'Dedicated account manager',
      '24/7 priority support',
      'Custom integrations',
      'Early access to features',
    ],
  },
];

const Billing: React.FC = () => {
  const { user } = useAuth();
  const [profitHistory, setProfitHistory] = useState<ProfitPeriod[]>([]);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'plans' | 'history' | 'invoices'>('plans');
  const [isUpgrading, setIsUpgrading] = useState(false);

  const currentTier = user?.subscription_tier || 'free';
  const tierInfo = TIER_INFO[currentTier as keyof typeof TIER_INFO] || TIER_INFO.free;

  useEffect(() => {
    fetchBillingData();
  }, []);

  const fetchBillingData = async () => {
    try {
      setIsLoading(true);
      const [history, invs] = await Promise.all([
        apiService.getProfitHistory(),
        apiService.getInvoices(),
      ]);
      setProfitHistory(history);
      setInvoices(invs);
    } catch (error) {
      console.error('Failed to load billing data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleUpgrade = async (tierId: string) => {
    if (tierId === currentTier) return;

    setIsUpgrading(true);
    try {
      const { checkout_url } = await apiService.createCheckoutSession(tierId);
      window.location.href = checkout_url;
    } catch (error) {
      console.error('Failed to create checkout session:', error);
      alert('Failed to start upgrade process. Please try again.');
    } finally {
      setIsUpgrading(false);
    }
  };

  const handleManageSubscription = async () => {
    try {
      const { portal_url } = await apiService.createCustomerPortal();
      window.location.href = portal_url;
    } catch (error) {
      console.error('Failed to open customer portal:', error);
      alert('Failed to open billing portal. Please try again.');
    }
  };

  const getTierIndex = (tierId: string) => tiers.findIndex(t => t.id === tierId);

  return (
    <div className="max-w-6xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">Billing & Subscription</h1>
        <p className="text-gray-400">Manage your subscription and view billing history</p>
      </div>

      {/* Current Plan Summary */}
      <div className="bg-dark-800 rounded-lg border border-dark-700 p-6 mb-6">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-gray-400 text-sm">Current Plan</p>
            <div className="flex items-center gap-3 mt-1">
              <h2 className="text-2xl font-bold text-white">{currentTier.toUpperCase()}</h2>
              <span className={`px-3 py-1 rounded-full text-sm ${
                currentTier === 'whale' ? 'bg-yellow-600/20 text-yellow-400' :
                currentTier === 'pro' ? 'bg-purple-600/20 text-purple-400' :
                currentTier === 'trader' ? 'bg-blue-600/20 text-blue-400' :
                'bg-gray-600/20 text-gray-400'
              }`}>
                {tierInfo.profitShare}% profit share
              </span>
            </div>
          </div>
          {currentTier !== 'free' && (
            <button
              onClick={handleManageSubscription}
              className="px-4 py-2 bg-dark-700 text-white rounded-lg hover:bg-dark-600 transition-colors"
            >
              Manage Subscription
            </button>
          )}
        </div>
        <div className="mt-4 grid grid-cols-3 gap-4">
          <div className="bg-dark-700 rounded-lg p-4">
            <p className="text-gray-400 text-sm">Max Positions</p>
            <p className="text-xl font-bold text-white">
              {tierInfo.maxPositions === -1 ? 'Unlimited' : tierInfo.maxPositions}
            </p>
          </div>
          <div className="bg-dark-700 rounded-lg p-4">
            <p className="text-gray-400 text-sm">Monthly Fee</p>
            <p className="text-xl font-bold text-white">${tierInfo.monthlyFee}/mo</p>
          </div>
          <div className="bg-dark-700 rounded-lg p-4">
            <p className="text-gray-400 text-sm">Features</p>
            <p className="text-xl font-bold text-white">{tierInfo.features.length} Active</p>
          </div>
        </div>
      </div>

      {/* Tab Navigation */}
      <div className="flex border-b border-dark-700 mb-6">
        <button
          onClick={() => setActiveTab('plans')}
          className={`px-4 py-3 font-medium transition-colors relative ${
            activeTab === 'plans' ? 'text-primary-400' : 'text-gray-400 hover:text-white'
          }`}
        >
          Plans & Pricing
          {activeTab === 'plans' && <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-500" />}
        </button>
        <button
          onClick={() => setActiveTab('history')}
          className={`px-4 py-3 font-medium transition-colors relative ${
            activeTab === 'history' ? 'text-primary-400' : 'text-gray-400 hover:text-white'
          }`}
        >
          Profit History
          {activeTab === 'history' && <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-500" />}
        </button>
        <button
          onClick={() => setActiveTab('invoices')}
          className={`px-4 py-3 font-medium transition-colors relative ${
            activeTab === 'invoices' ? 'text-primary-400' : 'text-gray-400 hover:text-white'
          }`}
        >
          Invoices
          {activeTab === 'invoices' && <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-500" />}
        </button>
      </div>

      {/* Plans Tab */}
      {activeTab === 'plans' && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {tiers.map((tier) => {
            const TierIcon = tier.icon;
            const isCurrentTier = tier.id === currentTier;
            const canUpgrade = getTierIndex(tier.id) > getTierIndex(currentTier);
            const canDowngrade = getTierIndex(tier.id) < getTierIndex(currentTier);

            return (
              <div
                key={tier.id}
                className={`relative bg-dark-800 rounded-lg border p-6 ${
                  tier.popular ? 'border-purple-500' : isCurrentTier ? 'border-primary-500' : 'border-dark-700'
                }`}
              >
                {tier.popular && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-1 bg-purple-600 text-white text-xs font-medium rounded-full">
                    Most Popular
                  </div>
                )}
                {isCurrentTier && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-1 bg-primary-600 text-white text-xs font-medium rounded-full">
                    Current Plan
                  </div>
                )}

                <div className={`w-12 h-12 rounded-lg flex items-center justify-center mb-4 bg-${tier.color}-600/20`}>
                  <TierIcon className={`w-6 h-6 text-${tier.color}-400`} />
                </div>

                <h3 className="text-xl font-bold text-white">{tier.name}</h3>
                <div className="mt-2">
                  <span className="text-3xl font-bold text-white">${tier.price}</span>
                  <span className="text-gray-400">/month</span>
                </div>
                <p className="text-sm text-gray-400 mt-1">+ {tier.profitShare}% profit share</p>

                <ul className="mt-4 space-y-2">
                  {tier.features.map((feature, idx) => (
                    <li key={idx} className="flex items-center gap-2 text-sm text-gray-300">
                      <Check className="w-4 h-4 text-green-400" />
                      {feature}
                    </li>
                  ))}
                </ul>

                <button
                  onClick={() => handleUpgrade(tier.id)}
                  disabled={isCurrentTier || isUpgrading}
                  className={`w-full mt-6 py-2 rounded-lg font-medium transition-colors ${
                    isCurrentTier
                      ? 'bg-dark-700 text-gray-400 cursor-not-allowed'
                      : canUpgrade
                      ? 'bg-primary-600 text-white hover:bg-primary-700'
                      : canDowngrade
                      ? 'bg-dark-700 text-gray-300 hover:bg-dark-600'
                      : 'bg-dark-700 text-gray-400'
                  }`}
                >
                  {isCurrentTier ? 'Current Plan' : canUpgrade ? 'Upgrade' : 'Downgrade'}
                </button>
              </div>
            );
          })}
        </div>
      )}

      {/* Profit History Tab */}
      {activeTab === 'history' && (
        <div className="bg-dark-800 rounded-lg border border-dark-700">
          {isLoading ? (
            <div className="p-8 text-center">
              <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto" />
              <p className="text-gray-400 mt-4">Loading profit history...</p>
            </div>
          ) : profitHistory.length === 0 ? (
            <div className="p-8 text-center">
              <TrendingUp className="w-12 h-12 text-gray-600 mx-auto mb-4" />
              <h3 className="text-lg font-medium text-white mb-2">No Profit History</h3>
              <p className="text-gray-400">Your weekly profit settlements will appear here.</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-dark-700">
                    <th className="text-left py-3 px-4 text-gray-400 font-medium">Period</th>
                    <th className="text-right py-3 px-4 text-gray-400 font-medium">Starting</th>
                    <th className="text-right py-3 px-4 text-gray-400 font-medium">Ending</th>
                    <th className="text-right py-3 px-4 text-gray-400 font-medium">Net Profit</th>
                    <th className="text-right py-3 px-4 text-gray-400 font-medium">Share Rate</th>
                    <th className="text-right py-3 px-4 text-gray-400 font-medium">Share Due</th>
                    <th className="text-center py-3 px-4 text-gray-400 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {profitHistory.map((period) => (
                    <tr key={period.id} className="border-b border-dark-700 hover:bg-dark-700/50">
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-2">
                          <Calendar className="w-4 h-4 text-gray-500" />
                          <span className="text-white">
                            {new Date(period.period_start).toLocaleDateString()} - {new Date(period.period_end).toLocaleDateString()}
                          </span>
                        </div>
                      </td>
                      <td className="py-3 px-4 text-right text-gray-300">
                        ${period.starting_balance.toFixed(2)}
                      </td>
                      <td className="py-3 px-4 text-right text-gray-300">
                        ${period.ending_balance?.toFixed(2) || '-'}
                      </td>
                      <td className={`py-3 px-4 text-right font-medium ${
                        period.net_profit >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}>
                        {period.net_profit >= 0 ? '+' : ''}${period.net_profit.toFixed(2)}
                      </td>
                      <td className="py-3 px-4 text-right text-gray-300">
                        {(period.profit_share_rate * 100).toFixed(0)}%
                      </td>
                      <td className="py-3 px-4 text-right text-white font-medium">
                        ${period.profit_share_due.toFixed(2)}
                      </td>
                      <td className="py-3 px-4 text-center">
                        <span className={`px-2 py-1 rounded text-xs ${
                          period.settlement_status === 'paid' ? 'bg-green-600/20 text-green-400' :
                          period.settlement_status === 'invoiced' ? 'bg-yellow-600/20 text-yellow-400' :
                          'bg-gray-600/20 text-gray-400'
                        }`}>
                          {period.settlement_status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Invoices Tab */}
      {activeTab === 'invoices' && (
        <div className="bg-dark-800 rounded-lg border border-dark-700">
          {isLoading ? (
            <div className="p-8 text-center">
              <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto" />
              <p className="text-gray-400 mt-4">Loading invoices...</p>
            </div>
          ) : invoices.length === 0 ? (
            <div className="p-8 text-center">
              <CreditCard className="w-12 h-12 text-gray-600 mx-auto mb-4" />
              <h3 className="text-lg font-medium text-white mb-2">No Invoices</h3>
              <p className="text-gray-400">Your invoices will appear here once you have billing activity.</p>
            </div>
          ) : (
            <div className="divide-y divide-dark-700">
              {invoices.map((invoice) => (
                <div key={invoice.id} className="flex items-center justify-between p-4 hover:bg-dark-700/50">
                  <div className="flex items-center gap-4">
                    <div className="w-10 h-10 rounded-lg bg-dark-700 flex items-center justify-center">
                      <CreditCard className="w-5 h-5 text-gray-400" />
                    </div>
                    <div>
                      <p className="text-white font-medium">${invoice.amount.toFixed(2)}</p>
                      <p className="text-gray-400 text-sm">
                        {new Date(invoice.created_at).toLocaleDateString()}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className={`px-2 py-1 rounded text-xs ${
                      invoice.status === 'paid' ? 'bg-green-600/20 text-green-400' :
                      invoice.status === 'pending' ? 'bg-yellow-600/20 text-yellow-400' :
                      'bg-red-600/20 text-red-400'
                    }`}>
                      {invoice.status}
                    </span>
                    {invoice.pdf_url && (
                      <a
                        href={invoice.pdf_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="p-2 text-gray-400 hover:text-white hover:bg-dark-600 rounded-lg transition-colors"
                      >
                        <Download className="w-5 h-5" />
                      </a>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Payment Methods Info */}
      <div className="mt-6 bg-dark-800 rounded-lg border border-dark-700 p-6">
        <div className="flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-blue-400 mt-0.5" />
          <div>
            <h3 className="text-white font-medium">Payment Information</h3>
            <p className="text-gray-400 text-sm mt-1">
              We accept credit cards, debit cards, and crypto payments (USDT/USDC).
              Profit share is calculated weekly and invoiced automatically.
              You can manage your payment methods in the Stripe customer portal.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Billing;
