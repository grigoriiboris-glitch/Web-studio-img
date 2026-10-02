import { describe, expect, it } from 'vitest'
import { buildPiAgentIncident, parsePiAgentBridgeUrl } from './comfySelfHealing'

describe('ComfyUI pi.dev bridge contract', () => {
  it('accepts localhost bridge URLs only', () => {
    expect(parsePiAgentBridgeUrl('http://localhost:8787/repair').hostname).toBe('localhost')
    expect(parsePiAgentBridgeUrl('http://127.0.0.1:8787/repair').hostname).toBe('127.0.0.1')
    expect(parsePiAgentBridgeUrl('http://[::1]:8787/repair').hostname).toBe('[::1]')
  })

  it('rejects remote bridge URLs', () => {
    expect(() => parsePiAgentBridgeUrl('https://example.com/repair')).toThrow()
    expect(() => parsePiAgentBridgeUrl('ftp://localhost/repair')).toThrow()
  })

  it('contains no dangerous or executable repair action', () => {
    const incident = buildPiAgentIncident(
      'project-1',
      'test-1',
      'runtime',
      [{ code: 'comfy_runtime', message: 'runtime failed', recoverable: true }],
      'fingerprint',
    )
    expect(incident.allowedActions.dangerous).toEqual([])
    expect(incident.allowedActions.repair).toEqual([
      'edit_allowlisted_workflow',
      'edit_allowlisted_config',
      'restart_comfyui',
      'retest',
    ])
    expect(incident.policy).toEqual({
      arbitraryShell: false,
      hostFilesystemAccess: false,
      volumeDeletion: false,
      imageReplacement: false,
      networkMutation: false,
    })
    expect(JSON.stringify(incident)).not.toMatch(/(bash|sh -c|rm -rf|docker exec|powershell)/i)
  })
})
