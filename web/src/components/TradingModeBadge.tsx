interface TradingModeBadgeProps {
  mode?: string;
}

export default function TradingModeBadge({ mode }: TradingModeBadgeProps) {
  if (!mode) return null;

  const getModeStyles = (mode: string) => {
    const styles = {
      ultra_fast: 'bg-red-500',
      scalp: 'bg-orange-500',
      swing: 'bg-blue-500',
      position: 'bg-green-500',
      scalp_reentry: 'bg-yellow-500',
    };
    return styles[mode as keyof typeof styles] || 'bg-gray-500';
  };

  const getModeLabel = (mode: string) => {
    const labels = {
      ultra_fast: 'ULTRA',
      scalp: 'SCALP',
      swing: 'SWING',
      position: 'POS',
      scalp_reentry: 'REENTRY',
    };
    return labels[mode as keyof typeof labels] || mode.toUpperCase();
  };

  return (
    <span
      className={`px-2 py-1 rounded text-xs font-bold text-white ${getModeStyles(mode)}`}
      title={`Trading Mode: ${mode.replace('_', ' ')}`}
    >
      {getModeLabel(mode)}
    </span>
  );
}
