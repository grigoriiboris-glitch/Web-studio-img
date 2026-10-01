export const REJECT_REASONS = [
  'composition', 'subject', 'pose', 'lighting', 'color', 'material',
  'background', 'object', 'style', 'prompt', 'quality', 'other',
] as const

export type RejectSeverity = '' | 'low' | 'medium' | 'high'

export function canSubmitReject(reasons: string[], skipReason: boolean): boolean {
  return skipReason || reasons.length > 0
}

export type RejectAnalyticsItem = { reason: string; count: number; percent: number }

export function buildAvoidConstraints(reasons: RejectAnalyticsItem[]): string[] {
  return reasons.slice(0, 5).map(item => 'avoid ' + item.reason)
}

export function buildRejectConstraints(reasons: RejectAnalyticsItem[]) {
  return reasons.slice(0, 5).map(({ reason, count, percent }) => ({ reason, count, percent }))
}
