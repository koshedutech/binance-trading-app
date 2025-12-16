import { getConfluenceColor } from '../utils/patternParser';
import type { PatternData } from '../types';

interface Props {
  patternData: PatternData;
}

export default function PatternDataDisplay({ patternData }: Props) {
  return (
    <div className="flex flex-wrap gap-2 items-center">
      {/* Pattern Name Badge */}
      <span className="badge bg-purple-900 text-purple-300 border border-purple-700">
        {patternData.patternName}
      </span>

      {/* Confidence Badge */}
      <span className="badge bg-blue-900 text-blue-300">
        {patternData.confidence}% confidence
      </span>

      {/* Confluence Score Badge */}
      <span className={`badge font-semibold ${getConfluenceColor(patternData.confluenceGrade)}`}>
        Confluence: {patternData.confluenceGrade} ({patternData.confluenceScore}%)
      </span>

      {/* FVG Indicator */}
      {patternData.fvgPresent && (
        <span className="badge bg-amber-900 text-amber-300">
          FVG
        </span>
      )}

      {/* Volume Indicator */}
      {patternData.volumeMultiplier && (
        <span className="badge bg-cyan-900 text-cyan-300">
          Vol {patternData.volumeMultiplier}x
        </span>
      )}
    </div>
  );
}
