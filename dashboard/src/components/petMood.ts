export type PetMood = 'idle' | 'busy' | 'sleeping' | 'alert'

export const MOOD_LABEL: Record<PetMood, string> = {
  idle:     'WAITING FOR JOBS',
  busy:     'JOBS RUNNING',
  sleeping: 'NO RUNNERS',
  alert:    'SCALING UP',
}

export const MOOD_SUBTEXT: Record<PetMood, string> = {
  idle:     'runners online, awaiting work',
  busy:     'one or more runners executing jobs',
  sleeping: 'all runner sets are empty',
  alert:    'new runners are being prepared',
}

export const MOOD_TOOLTIP: Record<PetMood, string> = {
  idle:     'At least one runner is online and idle, ready to pick up the next job.',
  busy:     'At least one runner is currently executing a job.',
  sleeping: 'No runners are active. The controller is watching for jobs.',
  alert:    'New runners are being provisioned to absorb pending jobs.',
}
