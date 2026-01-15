import { useState, ReactNode } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';

export interface CollapsibleCardProps {
  title: string;
  defaultExpanded?: boolean;
  badge?: string;
  badgeColor?: 'green' | 'red' | 'yellow' | 'blue' | 'purple' | 'cyan' | 'gray';
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
  headerClassName?: string;
  contentClassName?: string;
}

const badgeColors = {
  green: 'bg-green-500/20 text-green-400',
  red: 'bg-red-500/20 text-red-400',
  yellow: 'bg-yellow-500/20 text-yellow-400',
  blue: 'bg-blue-500/20 text-blue-400',
  purple: 'bg-purple-500/20 text-purple-400',
  cyan: 'bg-cyan-500/20 text-cyan-400',
  gray: 'bg-gray-500/20 text-gray-400',
};

export default function CollapsibleCard({
  title,
  defaultExpanded = false,
  badge,
  badgeColor = 'gray',
  icon,
  children,
  className = '',
  headerClassName = '',
  contentClassName = '',
}: CollapsibleCardProps) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  return (
    <div className={`bg-gray-700/30 rounded-lg overflow-hidden ${className}`}>
      {/* Header - clickable */}
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className={`w-full flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors ${headerClassName}`}
      >
        <div className="flex items-center gap-2">
          {icon && <span className="text-gray-400">{icon}</span>}
          <h3 className="text-sm font-medium text-gray-300">{title}</h3>
          {badge && (
            <span className={`px-2 py-0.5 rounded text-xs font-medium ${badgeColors[badgeColor]}`}>
              {badge}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {isExpanded ? (
            <ChevronUp className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          )}
        </div>
      </button>

      {/* Content - animated collapse/expand */}
      <div
        className={`transition-all duration-200 ease-in-out overflow-hidden ${
          isExpanded ? 'max-h-[2000px] opacity-100' : 'max-h-0 opacity-0'
        }`}
      >
        <div className={`px-3 pb-3 ${contentClassName}`}>
          {children}
        </div>
      </div>
    </div>
  );
}

// Compact variant for diagnostics sections
export function CollapsibleSection({
  title,
  defaultExpanded = false,
  badge,
  badgeColor = 'gray',
  icon,
  children,
  className = '',
}: Omit<CollapsibleCardProps, 'headerClassName' | 'contentClassName'>) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  return (
    <div className={`border-t border-gray-700 ${className}`}>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full flex items-center justify-between py-2 cursor-pointer hover:bg-gray-700/30 transition-colors"
      >
        <div className="flex items-center gap-1.5 text-xs text-gray-400">
          {icon}
          <span>{title}</span>
          {badge && (
            <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${badgeColors[badgeColor]}`}>
              {badge}
            </span>
          )}
        </div>
        {isExpanded ? (
          <ChevronUp className="w-3.5 h-3.5 text-gray-400" />
        ) : (
          <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
        )}
      </button>

      <div
        className={`transition-all duration-200 ease-in-out overflow-hidden ${
          isExpanded ? 'max-h-[2000px] opacity-100' : 'max-h-0 opacity-0'
        }`}
      >
        <div className="pb-2">
          {children}
        </div>
      </div>
    </div>
  );
}
