import { describe, expect, it } from 'vitest'

import { REJECT_REASONS, buildAvoidConstraints, canSubmitReject } from './rejectReasons'

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
      { reason: 'composition', count: 3 },
      { reason: 'lighting', count: 2 },
      { reason: 'color', count: 1 },
    ])).toEqual(['avoid composition', 'avoid lighting', 'avoid color'])
  })
})
