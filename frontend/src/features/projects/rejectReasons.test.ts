import { describe, expect, it } from 'vitest'

import {
  REJECT_REASONS,
  buildAvoidConstraints,
  buildRejectConstraints,
  canSubmitReject,
} from './rejectReasons'

describe('reject reason helpers', () => {
  it('contains the complete MVP reason set', () => {
    expect(REJECT_REASONS).toHaveLength(12)
    expect(REJECT_REASONS).toContain('composition')
    expect(REJECT_REASONS).toContain('quality')
    expect(REJECT_REASONS).toContain('other')
  })

  it('requires a reason unless skip is explicit', () => {
    expect(canSubmitReject([], false)).toBe(false)
    expect(canSubmitReject(['lighting'], false)).toBe(true)
    expect(canSubmitReject([], true)).toBe(true)
  })

  it('builds explicit avoid constraints without changing the prompt', () => {
    expect(buildAvoidConstraints([
      { reason: 'composition', count: 3, percent: 50 },
      { reason: 'lighting', count: 2, percent: 33.3 },
      { reason: 'color', count: 1, percent: 16.7 },
    ])).toEqual(['avoid composition', 'avoid lighting', 'avoid color'])
  })

  it('builds a bounded analytics snapshot for the next iteration', () => {
    expect(buildRejectConstraints([
      { reason: 'composition', count: 4, percent: 40 },
      { reason: 'lighting', count: 3, percent: 30 },
      { reason: 'color', count: 2, percent: 20 },
      { reason: 'subject', count: 1, percent: 10 },
      { reason: 'pose', count: 1, percent: 10 },
      { reason: 'style', count: 1, percent: 10 },
    ])).toEqual([
      { reason: 'composition', count: 4, percent: 40 },
      { reason: 'lighting', count: 3, percent: 30 },
      { reason: 'color', count: 2, percent: 20 },
      { reason: 'subject', count: 1, percent: 10 },
      { reason: 'pose', count: 1, percent: 10 },
    ])
  })
})
