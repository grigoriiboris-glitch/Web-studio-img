export function allWarningsOverridden(warnings: string[], selected: string[], reason: string): boolean {
  if (warnings.length === 0) return true
  if (!reason.trim()) return false
  return warnings.every(item => selected.includes(item)) && selected.length === warnings.length
}

export function approvalStatusClass(status: string): string {
  return 'check-' + status
}
