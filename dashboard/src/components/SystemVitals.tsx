import type { MachineVitals } from '../types'

interface VitalConfig {
  key: string
  persona: string
  unit: string
  max: number
  warn: number
  crit: number
}

const VITAL_CONFIGS: VitalConfig[] = [
  { key: 'cpu',  persona: 'PROCESSOR LOAD',   unit: '%',   max: 100, warn: 70, crit: 90 },
  { key: 'mem',  persona: 'MEMORY BANKS',     unit: '%',   max: 100, warn: 80, crit: 95 },
  { key: 'disk', persona: 'STORAGE ARRAY',    unit: '%',   max: 100, warn: 85, crit: 95 },
  { key: 'temp', persona: 'CORE TEMPERATURE', unit: '°C',  max: 100, warn: 70, crit: 85 },
]

function getValue(vitals: MachineVitals, key: string): number {
  switch (key) {
    case 'cpu':  return vitals.cpuUsagePercent
    case 'mem':  return vitals.memoryUsagePercent
    case 'disk': return vitals.diskUsagePercent
    case 'temp': return vitals.temperatureCelsius
    default:     return 0
  }
}

function VitalBar({ config, value }: { config: VitalConfig; value: number }) {
  const rounded = Math.round(value)
  const pct = Math.min(100, Math.max(0, (value / config.max) * 100))

  const color =
    rounded >= config.crit ? '#ff3b30' :
    rounded >= config.warn ? '#ff9500' :
    '#888888'

  const barColor =
    rounded >= config.crit ? '#ff3b30' :
    rounded >= config.warn ? '#ff9500' :
    '#e8e8e8'

  return (
    <div style={{ marginBottom: 14 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 5 }}>
        <span style={{ fontSize: 9, letterSpacing: '0.15em', color: '#555', textTransform: 'uppercase' }}>
          {config.persona}
        </span>
        <span style={{ fontSize: 13, fontWeight: 700, color, fontVariantNumeric: 'tabular-nums' }}>
          {rounded}{config.unit}
        </span>
      </div>
      <div style={{ height: 4, background: '#1a1a1a', position: 'relative', overflow: 'hidden' }}>
        <div
          style={{
            height: '100%',
            width: `${pct}%`,
            background: barColor,
            transition: 'width 1.5s ease, background 0.5s ease',
            position: 'relative',
          }}
        >
          <div style={{
            position: 'absolute', inset: 0,
            background: 'repeating-linear-gradient(90deg, transparent 0px, transparent 3px, rgba(0,0,0,0.4) 3px, rgba(0,0,0,0.4) 4px)',
          }} />
        </div>
      </div>
    </div>
  )
}

export function SystemVitals({ vitals }: { vitals: MachineVitals | null }) {
  if (!vitals) {
    return (
      <div style={{ fontSize: 10, color: '#444', letterSpacing: '0.1em' }}>
        LOADING...
      </div>
    )
  }

  return (
    <div>
      {VITAL_CONFIGS.map(config => (
        <VitalBar key={config.key} config={config} value={getValue(vitals, config.key)} />
      ))}
    </div>
  )
}
