import type { PatternData } from '../types';

/**
 * Parse pattern data from reason field
 * Format: "Pattern: morning_star (75% confidence) + FVG zone + High volume (2.4x) | Confluence: A (85%)"
 */
export function parsePatternData(reason?: string): PatternData | null {
  if (!reason || !reason.includes('Pattern:')) return null;

  try {
    const patternMatch = reason.match(/Pattern:\s*(\w+)\s*\((\d+)%\s*confidence\)/);
    const confluenceMatch = reason.match(/Confluence:\s*([A-F][+-]?)\s*\((\d+)%\)/);
    const volumeMatch = reason.match(/volume.*?\((\d+\.?\d*)x\)/i);

    if (!patternMatch || !confluenceMatch) return null;

    const factors = reason.split('+').slice(1).map(f => f.split('|')[0].trim());

    return {
      patternName: formatPatternName(patternMatch[1]),
      confidence: parseInt(patternMatch[2]),
      confluenceScore: parseInt(confluenceMatch[2]),
      confluenceGrade: confluenceMatch[1],
      fvgPresent: reason.toLowerCase().includes('fvg'),
      volumeMultiplier: volumeMatch ? parseFloat(volumeMatch[1]) : undefined,
      additionalFactors: factors.filter(f => f.length > 0)
    };
  } catch (e) {
    console.error('Failed to parse pattern data:', e);
    return null;
  }
}

/**
 * Convert "morning_star" to "Morning Star"
 */
export function formatPatternName(name: string): string {
  return name
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/**
 * Get Tailwind color class for confluence grade
 */
export function getConfluenceColor(grade: string): string {
  if (grade.startsWith('A')) return 'text-green-500';
  if (grade.startsWith('B')) return 'text-yellow-500';
  if (grade.startsWith('C')) return 'text-orange-500';
  return 'text-red-500';
}

/**
 * Get background color class for confidence percentage
 */
export function getConfidenceColor(confidence: number): string {
  if (confidence >= 80) return 'bg-green-900 text-green-300';
  if (confidence >= 65) return 'bg-blue-900 text-blue-300';
  if (confidence >= 50) return 'bg-yellow-900 text-yellow-300';
  return 'bg-orange-900 text-orange-300';
}
