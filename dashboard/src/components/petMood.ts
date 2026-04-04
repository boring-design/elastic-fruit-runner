export type PetMood = 'idle' | 'busy' | 'sleeping' | 'alert'

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
