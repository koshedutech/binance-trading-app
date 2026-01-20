// Epic 10: Exit Optimization Engine - Story 10.2: Position Analytics Dashboard
// Export Functionality Component

import React, { useState } from 'react';
import { Download, FileJson, FileSpreadsheet, Loader2 } from 'lucide-react';
import { TimeRange, ExportFormat, generateExportFilename } from '../../types/positionAnalytics';

interface ExportButtonProps {
  timeRange: TimeRange;
  modeFilter?: string;
  onExport: (format: ExportFormat, range: TimeRange, mode?: string) => Promise<void>;
}

export function ExportButton({ timeRange, modeFilter, onExport }: ExportButtonProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportFormat, setExportFormat] = useState<ExportFormat | null>(null);

  const handleExport = async (format: ExportFormat) => {
    setIsExporting(true);
    setExportFormat(format);
    try {
      await onExport(format, timeRange, modeFilter);
    } catch (error) {
      console.error('Export failed:', error);
    } finally {
      setIsExporting(false);
      setExportFormat(null);
      setIsOpen(false);
    }
  };

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        disabled={isExporting}
        className="flex items-center gap-2 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-700/50 text-white px-4 py-2 rounded-lg transition-colors"
      >
        {isExporting ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : (
          <Download className="w-4 h-4" />
        )}
        <span>{isExporting ? 'Exporting...' : 'Export'}</span>
      </button>

      {isOpen && !isExporting && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-10"
            onClick={() => setIsOpen(false)}
          />

          {/* Dropdown */}
          <div className="absolute right-0 mt-2 w-48 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-20">
            <div className="p-2">
              <p className="text-xs text-gray-400 px-3 py-1 mb-1">Export Format</p>
              <button
                onClick={() => handleExport('csv')}
                className="flex items-center gap-3 w-full px-3 py-2 text-sm text-gray-200 hover:bg-gray-700 rounded-md transition-colors"
              >
                <FileSpreadsheet className="w-4 h-4 text-green-400" />
                <div className="text-left">
                  <p className="font-medium">CSV</p>
                  <p className="text-xs text-gray-500">Spreadsheet format</p>
                </div>
              </button>
              <button
                onClick={() => handleExport('json')}
                className="flex items-center gap-3 w-full px-3 py-2 text-sm text-gray-200 hover:bg-gray-700 rounded-md transition-colors"
              >
                <FileJson className="w-4 h-4 text-yellow-400" />
                <div className="text-left">
                  <p className="font-medium">JSON</p>
                  <p className="text-xs text-gray-500">Developer format</p>
                </div>
              </button>
            </div>
            <div className="border-t border-gray-700 p-2">
              <p className="text-xs text-gray-500 px-3 py-1">
                Filename: {generateExportFilename(timeRange, 'csv').replace('.csv', '')}
              </p>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

// Standalone export helper function
export async function downloadExport(
  apiUrl: string,
  format: ExportFormat,
  range: TimeRange,
  mode?: string
): Promise<void> {
  const params = new URLSearchParams({
    format,
    range,
  });
  if (mode) {
    params.append('mode', mode);
  }

  const url = `${apiUrl}?${params.toString()}`;

  try {
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${localStorage.getItem('access_token')}`,
      },
    });

    if (!response.ok) {
      throw new Error(`Export failed: ${response.statusText}`);
    }

    // Get filename from Content-Disposition header or generate one
    const contentDisposition = response.headers.get('Content-Disposition');
    let filename = generateExportFilename(range, format);
    if (contentDisposition) {
      const filenameMatch = contentDisposition.match(/filename=(.+)/);
      if (filenameMatch) {
        filename = filenameMatch[1].replace(/"/g, '');
      }
    }

    // Download the file
    const blob = await response.blob();
    const downloadUrl = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = downloadUrl;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(downloadUrl);
  } catch (error) {
    console.error('Download failed:', error);
    throw error;
  }
}

export default ExportButton;
