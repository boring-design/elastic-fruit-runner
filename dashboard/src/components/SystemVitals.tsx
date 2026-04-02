import { useState, useEffect } from 'react'

interface Vital {
  key: string
  label: string       // human label
  persona: string     // personified label
  unit: string
  base: number        // base value
  variance: number    // random noise range
  warn: number        // warn threshold
  crit: number        // critical threshold
}

const VITALS: Vital[] = [
  { key: 'cpu',  label: 'CPU',      persona: 'PROCESSOR LOAD',   unit: '%',   base: 34,  variance: 18, warn: 70, crit: 90 },
  { key: 'mem',  label: 'MEMORY',   persona: 'MEMORY BANKS',     unit: '%',   base: 52,  variance: 8,  warn: 80, crit: 95 },
  { key: 'disk', label: 'DISK',     persona: 'STORAGE ARRAY',    unit: '%',   base: 67,  variance: 1,  warn: 85, crit: 95 },
  { key: 'temp', label: 'TEMP',     persona: 'CORE TEMPERATURE', unit: '°C',  base: 48,  variance: 12, warn: 70, crit: 85 },
]

function clamp(v: number, min: number, max: number) {
  return Math.min(max, Math.max(min, v))
}

function useFluctuating(base: number, variance: number, interval = 2000) {
  const [value, setValue] = useState(base)
  useEffect(() => {
    const id = setInterval(() => {
      const delta = (Math.random() - 0.5) * variance
      setValue(v => clamp(Math.round(v + delta * 0.3), base - variance, base + variance))
    }, interval)
    return () => clearInterval(id)
  }, [base, variance, interval])
  return value
}

function VitalBar({ vital }: { vital: Vital }) {
  const value = useFluctuating(vital.base, vital.variance)
  const pct = clamp(value / (vital.key === 'temp' ? 100 : 100) * 100, 0, 100)

  const color =
    value >= vital.crit ? '#ff3b30' :
    value >= vital.warn ? '#ff9500' :
    '#888888'

  const barColor =
    value >= vital.crit ? '#ff3b30' :
    value >= vital.warn ? '#ff9500' :
    '#e8e8e8'

  return (
    <div style={{ marginBottom: 14 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 5 }}>
        <span style={{ fontSize: 9, letterSpacing: '0.15em', color: '#555', textTransform: 'uppercase' }}>
          {vital.persona}
        </span>
        <span style={{ fontSize: 13, fontWeight: 700, color, fontVariantNumeric: 'tabular-nums' }}>
          {value}{vital.unit}
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
          {/* barcode texture */}
          <div style={{
            position: 'absolute', inset: 0,
            background: 'repeating-linear-gradient(90deg, transparent 0px, transparent 3px, rgba(0,0,0,0.4) 3px, rgba(0,0,0,0.4) 4px)',
          }} />
        </div>
      </div>
    </div>
  )
}

export function SystemVitals() {
  return (
    <div>
      {VITALS.map(v => <VitalBar key={v.key} vital={v} />)}
    </div>
  )
}
