import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Database,
  Download,
  RefreshCw,
  BarChart2,
  HardDrive,
  Calendar,
  Sparkles,
  CheckCircle,
  AlertTriangle,
  Clock,
  Play,
  Pause,
} from 'lucide-react';
import { apiService } from '../services/api';
import DataAvailabilityTable, { CoinDataInfo } from '../components/Research/DataAvailabilityTable';
import FeatureList, { FeatureCategories } from '../components/Research/FeatureList';
import DownloadDataModal, { DownloadRequest } from '../components/Research/DownloadDataModal';

// Wrapper type for API responses
interface APIWrapper<T> {
  success: boolean;
  data: T;
}

// Types for API responses
interface DataAvailabilityResponse {
  coins: CoinDataInfo[];
  total_coins: number;
  total_candles: number;
  storage_size_mb: number;
}

interface FeaturesResponse {
  features: string[];
  count: number;
  categories: FeatureCategories;
  operators: string[];
}

interface FeatureJob {
  job_id: string;
  symbol: string;
  timeframe: string;
  status: string;
  progress: number;
  total_candles: number;
  processed_candles: number;
  cached_features: number;
  error?: string;
}

interface DownloadJob {
  job_id: string;
  status: string;
  progress: number;
  candles_downloaded: number;
  candles_total: number;
  error?: string;
}

// Format large numbers
function formatNumber(num: number): string {
  if (num >= 1000000) {
    return `${(num / 1000000).toFixed(1)}M`;
  }
  if (num >= 1000) {
    return `${(num / 1000).toFixed(1)}K`;
  }
  return num.toString();
}

// Format storage size
function formatStorage(mb: number): string {
  if (mb >= 1024) {
    return `${(mb / 1024).toFixed(2)} GB`;
  }
  return `${mb.toFixed(2)} MB`;
}

export default function ResearchData() {
  // Data state
  const [dataAvailability, setDataAvailability] = useState<DataAvailabilityResponse | null>(null);
  const [features, setFeatures] = useState<FeaturesResponse | null>(null);
  const [isLoadingData, setIsLoadingData] = useState(true);
  const [isLoadingFeatures, setIsLoadingFeatures] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Modal state
  const [showDownloadModal, setShowDownloadModal] = useState(false);

  // Active jobs state
  const [downloadJobs, setDownloadJobs] = useState<DownloadJob[]>([]);
  const [featureJobs, setFeatureJobs] = useState<FeatureJob[]>([]);

  // Refs to track current jobs for stale closure prevention
  const downloadJobsRef = useRef<DownloadJob[]>([]);
  const featureJobsRef = useRef<FeatureJob[]>([]);

  // Keep refs in sync with state
  useEffect(() => {
    downloadJobsRef.current = downloadJobs;
  }, [downloadJobs]);

  useEffect(() => {
    featureJobsRef.current = featureJobs;
  }, [featureJobs]);

  // Fetch data availability
  const fetchDataAvailability = useCallback(async () => {
    setIsLoadingData(true);
    setError(null);
    try {
      const response = await apiService.get<APIWrapper<DataAvailabilityResponse>>('/research/data-availability');
      setDataAvailability(response.data.data);
    } catch (err) {
      console.error('Failed to fetch data availability:', err);
      setError('Failed to load data availability');
    } finally {
      setIsLoadingData(false);
    }
  }, []);

  // Fetch features
  const fetchFeatures = useCallback(async () => {
    setIsLoadingFeatures(true);
    try {
      const response = await apiService.get<APIWrapper<FeaturesResponse>>('/research/features');
      setFeatures(response.data.data);
    } catch (err) {
      console.error('Failed to fetch features:', err);
    } finally {
      setIsLoadingFeatures(false);
    }
  }, []);

  // Fetch active download jobs (including paused ones that can be resumed)
  const fetchDownloadJobs = useCallback(async () => {
    try {
      const response = await apiService.get<APIWrapper<{ jobs: DownloadJob[]; total: number }>>('/research/download-jobs');
      const activeJobs = response.data.data.jobs.filter(
        (job) => job.status === 'pending' || job.status === 'in_progress' || job.status === 'paused'
      );
      setDownloadJobs(activeJobs);
    } catch (err) {
      console.error('Failed to fetch download jobs:', err);
    }
  }, []);

  // Fetch active feature jobs
  const fetchFeatureJobs = useCallback(async () => {
    try {
      const response = await apiService.get<APIWrapper<{ jobs: FeatureJob[]; total: number }>>('/research/feature-jobs');
      const activeJobs = response.data.data.jobs.filter(
        (job) => job.status === 'pending' || job.status === 'running'
      );
      setFeatureJobs(activeJobs);
    } catch (err) {
      console.error('Failed to fetch feature jobs:', err);
    }
  }, []);

  // Initial data load
  useEffect(() => {
    fetchDataAvailability();
    fetchFeatures();
    fetchDownloadJobs();
    fetchFeatureJobs();
  }, [fetchDataAvailability, fetchFeatures, fetchDownloadJobs, fetchFeatureJobs]);

  // Poll for active jobs
  useEffect(() => {
    if (downloadJobs.length === 0 && featureJobs.length === 0) return;

    const interval = setInterval(() => {
      fetchDownloadJobs();
      fetchFeatureJobs();
      // Also refresh data availability to show new data - use ref to avoid stale closure
      if (downloadJobsRef.current.some((j) => j.status === 'in_progress')) {
        fetchDataAvailability();
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [downloadJobs.length, featureJobs.length, fetchDownloadJobs, fetchFeatureJobs, fetchDataAvailability]);

  // Handle download submission
  const handleDownload = async (request: DownloadRequest) => {
    try {
      await apiService.post<{ job_id: string; status: string; estimated_candles: number }>(
        '/research/download-data',
        request
      );
      // Refresh jobs list
      await fetchDownloadJobs();
    } catch (err) {
      console.error('Failed to start download:', err);
      // Re-throw so the modal can display the error
      throw err;
    }
  };

  // Handle feature calculation
  const handleCalculateFeatures = async (symbol: string, timeframe: string) => {
    try {
      await apiService.post('/research/calculate-features', { symbol, timeframe });
      await fetchFeatureJobs();
    } catch (err) {
      console.error('Failed to start feature calculation:', err);
    }
  };

  // Handle resume download job
  const handleResumeDownload = async (jobId: string) => {
    try {
      await apiService.post(`/research/download-resume/${jobId}`, {});
      await fetchDownloadJobs();
    } catch (err) {
      console.error('Failed to resume download:', err);
    }
  };

  // Calculate if all data has features
  const allFeaturesCalculated = dataAvailability
    ? dataAvailability.coins.length > 0 && featureJobs.length === 0
    : false;

  return (
    <div className="min-h-screen bg-gray-900 p-6">
      <div className="max-w-7xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-white flex items-center gap-3">
              <Database className="w-7 h-7 text-primary-400" />
              Research Data
            </h1>
            <p className="text-gray-400 mt-1">
              Manage historical candle data for backtesting and pattern discovery
            </p>
          </div>
          <button
            onClick={() => setShowDownloadModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
          >
            <Download className="w-4 h-4" />
            Download Data
          </button>
        </div>

        {/* Error alert */}
        {error && (
          <div className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500/30 rounded-lg">
            <AlertTriangle className="w-5 h-5 text-red-400" />
            <span className="text-red-400">{error}</span>
            <button
              onClick={() => setError(null)}
              className="ml-auto text-red-400 hover:text-red-300"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Active Jobs */}
        {(downloadJobs.length > 0 || featureJobs.length > 0) && (
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-4">
            <h3 className="text-sm font-medium text-gray-300 mb-3 flex items-center gap-2">
              <Clock className="w-4 h-4 text-yellow-400" />
              Active Jobs
            </h3>
            <div className="space-y-2">
              {downloadJobs.map((job) => {
                const isPaused = job.status === 'paused';
                return (
                  <div
                    key={job.job_id}
                    className={`flex items-center gap-4 p-3 rounded-lg ${
                      isPaused ? 'bg-yellow-500/10 border border-yellow-500/30' : 'bg-dark-700/50'
                    }`}
                  >
                    {isPaused ? (
                      <Pause className="w-4 h-4 text-yellow-400" />
                    ) : (
                      <Download className="w-4 h-4 text-primary-400" />
                    )}
                    <div className="flex-1">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className="text-sm text-white">
                            {isPaused ? 'Paused' : 'Downloading'}: {job.job_id}
                          </span>
                          {isPaused && job.error && (
                            <span className="text-xs text-yellow-400">{job.error}</span>
                          )}
                        </div>
                        <span className="text-xs text-gray-400">
                          {job.candles_downloaded.toLocaleString()} / {job.candles_total.toLocaleString()} candles
                        </span>
                      </div>
                      <div className="mt-1 h-1.5 bg-dark-600 rounded-full overflow-hidden">
                        <div
                          className={`h-full transition-all duration-300 ${
                            isPaused ? 'bg-yellow-500' : 'bg-primary-500'
                          }`}
                          style={{ width: `${job.progress}%` }}
                        />
                      </div>
                    </div>
                    <span className="text-xs text-gray-400">{job.progress.toFixed(1)}%</span>
                    {isPaused && (
                      <button
                        onClick={() => handleResumeDownload(job.job_id)}
                        className="p-1.5 bg-green-500/20 hover:bg-green-500/30 rounded text-green-400"
                        title="Resume download"
                      >
                        <Play className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                );
              })}
              {featureJobs.map((job) => (
                <div
                  key={job.job_id}
                  className="flex items-center gap-4 p-3 bg-dark-700/50 rounded-lg"
                >
                  <BarChart2 className="w-4 h-4 text-purple-400" />
                  <div className="flex-1">
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-white">
                        Calculating features: {job.symbol} ({job.timeframe})
                      </span>
                      <span className="text-xs text-gray-400">
                        {job.processed_candles.toLocaleString()} / {job.total_candles.toLocaleString()} candles
                      </span>
                    </div>
                    <div className="mt-1 h-1.5 bg-dark-600 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-purple-500 transition-all duration-300"
                        style={{ width: `${job.progress}%` }}
                      />
                    </div>
                  </div>
                  <span className="text-xs text-gray-400">{job.progress.toFixed(1)}%</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Data Summary Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          {/* Total Coins */}
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-4">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <Database className="w-4 h-4" />
              Total Coins
            </div>
            <div className="text-2xl font-bold text-white">
              {isLoadingData ? (
                <span className="text-gray-500">--</span>
              ) : (
                dataAvailability?.total_coins || 0
              )}
            </div>
          </div>

          {/* Total Candles */}
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-4">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <BarChart2 className="w-4 h-4" />
              Total Candles
            </div>
            <div className="text-2xl font-bold text-white">
              {isLoadingData ? (
                <span className="text-gray-500">--</span>
              ) : (
                formatNumber(dataAvailability?.total_candles || 0)
              )}
            </div>
          </div>

          {/* Storage */}
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-4">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <HardDrive className="w-4 h-4" />
              Storage
            </div>
            <div className="text-2xl font-bold text-white">
              {isLoadingData ? (
                <span className="text-gray-500">--</span>
              ) : (
                formatStorage(dataAvailability?.storage_size_mb || 0)
              )}
            </div>
          </div>

          {/* Features */}
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-4">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <Sparkles className="w-4 h-4" />
              Features
            </div>
            <div className="text-2xl font-bold text-white">
              {isLoadingFeatures ? (
                <span className="text-gray-500">--</span>
              ) : (
                `${features?.count || 0} calculated`
              )}
            </div>
          </div>

          {/* Status */}
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-4">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <Calendar className="w-4 h-4" />
              Status
            </div>
            <div className="flex items-center gap-2">
              {downloadJobs.length > 0 || featureJobs.length > 0 ? (
                <>
                  <RefreshCw className="w-4 h-4 text-yellow-400 animate-spin" />
                  <span className="text-yellow-400 font-medium">Processing</span>
                </>
              ) : allFeaturesCalculated ? (
                <>
                  <CheckCircle className="w-4 h-4 text-green-400" />
                  <span className="text-green-400 font-medium">Up to date</span>
                </>
              ) : (
                <>
                  <AlertTriangle className="w-4 h-4 text-yellow-400" />
                  <span className="text-yellow-400 font-medium">Features pending</span>
                </>
              )}
            </div>
          </div>
        </div>

        {/* Data Availability Table */}
        <DataAvailabilityTable
          coins={dataAvailability?.coins || []}
          onRefresh={fetchDataAvailability}
          onCalculateFeatures={handleCalculateFeatures}
          isLoading={isLoadingData}
        />

        {/* Feature List */}
        {features && features.categories && (
          <FeatureList
            categories={features.categories}
            totalCount={features.count}
          />
        )}

        {/* Download Modal */}
        <DownloadDataModal
          isOpen={showDownloadModal}
          onClose={() => setShowDownloadModal(false)}
          onSubmit={handleDownload}
          existingSymbols={dataAvailability?.coins.map((c) => c.symbol) || []}
        />
      </div>
    </div>
  );
}
