import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';

interface NavigationControlsProps {
  currentPage: number;
  totalPages: number;
  pageLabel: string;
  onFirst: () => void;
  onPrev: () => void;
  onNext: () => void;
  onLast: () => void;
}

export default function NavigationControls({
  currentPage,
  totalPages,
  pageLabel,
  onFirst,
  onPrev,
  onNext,
  onLast,
}: NavigationControlsProps) {
  const isFirst = currentPage <= 1;
  const isLast = currentPage >= totalPages;

  return (
    <div className="flex items-center gap-1">
      {/* First button */}
      <button
        onClick={onFirst}
        disabled={isFirst}
        className={`
          p-1.5 rounded transition-all
          ${isFirst
            ? 'text-gray-600 cursor-not-allowed'
            : 'text-gray-400 hover:text-white hover:bg-gray-700'
          }
        `}
        title="First"
      >
        <ChevronsLeft className="w-4 h-4" />
      </button>

      {/* Previous button */}
      <button
        onClick={onPrev}
        disabled={isFirst}
        className={`
          p-1.5 rounded transition-all
          ${isFirst
            ? 'text-gray-600 cursor-not-allowed'
            : 'text-gray-400 hover:text-white hover:bg-gray-700'
          }
        `}
        title="Previous"
      >
        <ChevronLeft className="w-4 h-4" />
      </button>

      {/* Page indicator */}
      <div className="px-3 py-1 text-xs text-gray-400 min-w-[100px] text-center">
        {pageLabel}
      </div>

      {/* Next button */}
      <button
        onClick={onNext}
        disabled={isLast}
        className={`
          p-1.5 rounded transition-all
          ${isLast
            ? 'text-gray-600 cursor-not-allowed'
            : 'text-gray-400 hover:text-white hover:bg-gray-700'
          }
        `}
        title="Next"
      >
        <ChevronRight className="w-4 h-4" />
      </button>

      {/* Last button */}
      <button
        onClick={onLast}
        disabled={isLast}
        className={`
          p-1.5 rounded transition-all
          ${isLast
            ? 'text-gray-600 cursor-not-allowed'
            : 'text-gray-400 hover:text-white hover:bg-gray-700'
          }
        `}
        title="Last"
      >
        <ChevronsRight className="w-4 h-4" />
      </button>
    </div>
  );
}
