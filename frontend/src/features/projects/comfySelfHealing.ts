export type PiAgentError = {
  code?: string
  message: string
  node?: string
  input?: string
  recoverable?: boolean
}

export type PiAgentIncident = {
  protocol: 'comfyui-self-healing/v1'
  projectId: string
  container: 'comfyui'
  endpoint: 'local-comfyui'
  testId: string
  category: string
  errors: PiAgentError[]
  workflowFingerprint: string
  allowedActions: {
    safe: string[]
    repair: string[]
    dangerous: string[]
  }
  policy: {
    arbitraryShell: false
    hostFilesystemAccess: false
    volumeDeletion: false
    imageReplacement: false
    networkMutation: false
  }
}

const LOCAL_BRIDGE_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]'])

export function parsePiAgentBridgeUrl(value: string): URL {
  const parsed = new URL(value.trim())
  if (!LOCAL_BRIDGE_HOSTS.has(parsed.hostname)) {
    throw new Error('pi.dev bridge должен быть доступен только через localhost')
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('pi.dev bridge должен использовать HTTP(S)')
  }
  return parsed
}

export function buildPiAgentIncident(
  projectId: string,
  testId: string,
  category: string,
  errors: PiAgentError[],
  workflowFingerprint: string,
): PiAgentIncident {
  return {
    protocol: 'comfyui-self-healing/v1',
    projectId,
    container: 'comfyui',
    endpoint: 'local-comfyui',
    testId,
    category,
    errors: errors.map(item => ({
      code: item.code,
      message: item.message,
      node: item.node,
      input: item.input,
      recoverable: item.recoverable,
    })),
    workflowFingerprint,
    allowedActions: {
      safe: ['inspect_runtime', 'read_logs', 'read_status'],
      repair: ['edit_allowlisted_workflow', 'edit_allowlisted_config', 'restart_comfyui', 'retest'],
      dangerous: [],
    },
    policy: {
      arbitraryShell: false,
      hostFilesystemAccess: false,
      volumeDeletion: false,
      imageReplacement: false,
      networkMutation: false,
    },
  }
}
