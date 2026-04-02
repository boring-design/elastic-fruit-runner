import { useRef, useEffect, useState } from 'react'

export type PetMood = 'idle' | 'busy' | 'sleeping' | 'alert'

// ─── Pixel frame definitions (16×16) ─────────────────────────────────────────
// 0 = transparent  1 = body  2 = eye/bright  3 = mouth  4 = dim/shadow

type P = 0 | 1 | 2 | 3 | 4
type Frame = P[][]

const _ = 0, B = 1, E = 2, M = 3, D = 4

// Eyes OPEN, smile
const FRAME_IDLE_OPEN: Frame = [
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,B,B,D,D,D,D,D,D,B,B,_,_,_],
  [_,_,_,B,D,_,_,_,_,_,_,D,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,M,_,_,_,_,M,_,B,_,_,_],
  [_,_,_,B,_,_,M,M,M,M,_,_,B,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,_,B,B,_,_,_,_,_,_,B,B,_,_,_],
]

// Eyes half-closed (blink frame 1)
const FRAME_BLINK_HALF: Frame = [
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,B,B,D,D,D,D,D,D,B,B,_,_,_],
  [_,_,_,B,D,_,_,_,_,_,_,D,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,M,_,_,_,_,M,_,B,_,_,_],
  [_,_,_,B,_,_,M,M,M,M,_,_,B,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,_,B,B,_,_,_,_,_,_,B,B,_,_,_],
]

// Eyes closed (blink frame 2)
const FRAME_BLINK_CLOSED: Frame = [
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,B,B,D,D,D,D,D,D,B,B,_,_,_],
  [_,_,_,B,D,_,_,_,_,_,_,D,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,M,_,_,_,_,M,_,B,_,_,_],
  [_,_,_,B,_,_,M,M,M,M,_,_,B,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,_,B,B,_,_,_,_,_,_,B,B,_,_,_],
]

// BUSY — eyes wide open, focused (larger bright eyes)
const FRAME_BUSY: Frame = [
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,B,B,D,D,D,D,D,D,B,B,_,_,_],
  [_,_,_,B,_,E,E,E,E,E,E,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,E,E,E,E,E,E,_,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,M,M,M,M,M,M,_,B,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,B,_,_,B,B,E,E,B,B,_,_,B,_,_],
  [_,_,B,_,_,B,B,E,E,B,B,_,_,B,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,_,B,B,_,_,_,_,_,_,B,B,_,_,_],
]

// SLEEPING — eyes closed, zzz mouth
const FRAME_SLEEP: Frame = [
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,B,B,D,D,D,D,D,D,B,B,_,_,_],
  [_,_,_,B,D,_,_,_,_,_,_,D,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,_,_,_,_,_,_,_,B,_,_,_],
  [_,_,_,B,_,_,M,M,M,_,_,_,B,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,_,B,B,_,_,_,_,_,_,B,B,_,_,_],
]

// ALERT / PREPARING — eyes open wide + exclamation
const FRAME_ALERT: Frame = [
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,B,B,D,D,D,D,D,D,B,B,_,_,_],
  [_,_,_,B,E,_,_,_,_,_,_,E,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,_,E,E,_,_,E,E,_,B,_,_,_],
  [_,_,_,B,E,_,_,_,_,_,_,E,B,_,_,_],
  [_,_,_,B,_,_,M,_,_,M,_,_,B,_,_,_],
  [_,_,_,B,_,_,_,M,M,_,_,_,B,_,_,_],
  [_,_,_,_,B,B,B,B,B,B,B,B,_,_,_,_],
  [_,_,_,_,_,B,B,B,B,B,B,_,_,_,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,_,_,B,B,B,B,B,B,_,_,B,_,_],
  [_,_,B,B,B,B,B,B,B,B,B,B,B,B,_,_],
  [_,_,_,B,B,_,_,_,_,_,_,B,B,_,_,_],
]

// ─── Color palette per mood ────────────────────────────────────────────────────
const PALETTE: Record<PetMood, Record<number, string>> = {
  idle:     { 1: '#b0b0b0', 2: '#ffffff', 3: '#ffffff', 4: '#555' },
  busy:     { 1: '#c8c8c8', 2: '#ffffff', 3: '#ffffff', 4: '#666' },
  sleeping: { 1: '#707070', 2: '#888888', 3: '#888888', 4: '#444' },
  alert:    { 1: '#d0d0d0', 2: '#ffffff', 3: '#ffffff', 4: '#555' },
}

// ─── Animation sequences ───────────────────────────────────────────────────────
const SEQUENCES: Record<PetMood, { frame: Frame; duration: number }[]> = {
  idle: [
    { frame: FRAME_IDLE_OPEN,   duration: 3000 },
    { frame: FRAME_BLINK_HALF,  duration: 80  },
    { frame: FRAME_BLINK_CLOSED,duration: 100 },
    { frame: FRAME_BLINK_HALF,  duration: 80  },
    { frame: FRAME_IDLE_OPEN,   duration: 2500 },
    { frame: FRAME_BLINK_HALF,  duration: 80  },
    { frame: FRAME_BLINK_CLOSED,duration: 100 },
    { frame: FRAME_BLINK_HALF,  duration: 80  },
  ],
  busy: [
    { frame: FRAME_BUSY, duration: 400 },
    { frame: FRAME_IDLE_OPEN, duration: 150 },
    { frame: FRAME_BUSY, duration: 400 },
    { frame: FRAME_IDLE_OPEN, duration: 150 },
  ],
  sleeping: [
    { frame: FRAME_SLEEP, duration: 1200 },
    { frame: FRAME_BLINK_CLOSED, duration: 1200 },
  ],
  alert: [
    { frame: FRAME_ALERT, duration: 300 },
    { frame: FRAME_IDLE_OPEN, duration: 200 },
    { frame: FRAME_ALERT, duration: 300 },
    { frame: FRAME_IDLE_OPEN, duration: 200 },
    { frame: FRAME_ALERT, duration: 300 },
    { frame: FRAME_IDLE_OPEN, duration: 800 },
  ],
}

// ─── Canvas renderer ──────────────────────────────────────────────────────────
const GRID = 16
const SCALE = 4  // each logical pixel = 4 CSS px → 64×64 display

function drawFrame(ctx: CanvasRenderingContext2D, frame: Frame, palette: Record<number, string>) {
  ctx.clearRect(0, 0, GRID * SCALE, GRID * SCALE)
  for (let row = 0; row < GRID; row++) {
    for (let col = 0; col < GRID; col++) {
      const val = frame[row]?.[col] ?? 0
      if (val === 0) continue
      ctx.fillStyle = palette[val] ?? '#fff'
      ctx.fillRect(col * SCALE, row * SCALE, SCALE, SCALE)
    }
  }
}

// ─── Component ────────────────────────────────────────────────────────────────
interface PixelPetProps {
  mood: PetMood
}

export function PixelPet({ mood }: PixelPetProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [seqIdx, setSeqIdx] = useState(0)

  // Reset sequence index when mood changes
  useEffect(() => { setSeqIdx(0) }, [mood])

  // Advance animation frames
  useEffect(() => {
    const seq = SEQUENCES[mood]
    const { duration } = seq[seqIdx % seq.length]
    const id = setTimeout(() => setSeqIdx(i => (i + 1) % seq.length), duration)
    return () => clearTimeout(id)
  }, [mood, seqIdx])

  // Draw current frame
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.imageSmoothingEnabled = false
    const seq = SEQUENCES[mood]
    drawFrame(ctx, seq[seqIdx % seq.length].frame, PALETTE[mood])
  }, [mood, seqIdx])

  return (
    <canvas
      ref={canvasRef}
      width={GRID * SCALE}
      height={GRID * SCALE}
      style={{ imageRendering: 'pixelated', display: 'block' }}
    />
  )
}

// ─── Status text per mood ─────────────────────────────────────────────────────
export const MOOD_LABEL: Record<PetMood, string> = {
  idle:     'STANDING BY',
  busy:     'PROCESSING',
  sleeping: 'RESTING',
  alert:    'SPINNING UP',
}

export const MOOD_SUBTEXT: Record<PetMood, string> = {
  idle:     'runners waiting for jobs',
  busy:     'jobs in execution',
  sleeping: 'all runners idle',
  alert:    'preparing new runners',
}
