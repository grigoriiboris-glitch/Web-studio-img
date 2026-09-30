import { describe, expect, it } from 'vitest'

import { allWarningsOverridden, approvalStatusClass } from './workflowApproval'

describe('approval gate helpers', () => {
  it('requires every warning and a reason', () => {
    expect(allWarningsOverridden([], [], '')).toBe(true)
    expect(allWarningsOverridden(['similarity'], ['similarity'], 'skip documented')).toBe(true)
    expect(allWarningsOverridden(['similarity'], [], 'skip documented')).toBe(false)
    expect(allWarningsOverridden(['similarity'], ['similarity'], '')).toBe(false)
    expect(allWarningsOverridden(['similarity'], ['similarity', 'extra'], 'reason')).toBe(false)
  })

  it('maps checklist status to a stable class', () => {
    expect(approvalStatusClass('passed')).toBe('check-passed')
    expect(approvalStatusClass('warning')).toBe('check-warning')
    expect(approvalStatusClass('blocked')).toBe('check-blocked')
  })
})
