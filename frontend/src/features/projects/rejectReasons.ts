export const REJECT_REASONS = [
  'composition', 'subject', 'pose', 'lighting', 'color', 'material',
  'background', 'object', 'style', 'prompt', 'quality', 'other',
] as const

export type RejectSeverity = '' | 'low' | 'medium' | 'high'

export function canSubmitReject(reasons: string[], skipReason: boolean): boolean {
  return skipReason || reasons.length > 0
}

export function buildAvoidConstraints(reasons: Array<{ reason: string; count: number }>): string[] {
  return reasons.slice(0, 5).map(item => 'avoid ' + item.reason)
}
