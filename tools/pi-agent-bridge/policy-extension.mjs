import path from "node:path";

const root = path.resolve(process.env.PI_AGENT_PROJECT_ROOT || process.cwd());

function insideProject(file) {
  const resolved = path.resolve(root, file || ".");
  return resolved === root || resolved.startsWith(root + path.sep);
}

function protectedPath(file) {
  const normalized = String(file || "").replaceAll("\\", "/");
  return /(^|\/)\.env(?:\.|$)/i.test(normalized)
    || /(^|\/)\.git(?:\/|$)/i.test(normalized)
    || /(^|\/)(?:id_rsa|id_ed25519|credentials|secrets?)(?:\.|$)/i.test(normalized)
    || /(^|\/)node_modules(?:\/|$)/i.test(normalized);
}

function allowedConfigPath(file) {
  const normalized = String(file || "").replaceAll("\\", "/").toLowerCase();
  if (!insideProject(file) || protectedPath(file)) return false;
  return /(?:^|\/)(?:docker-compose|compose|comfy|docker|config|workflow)[^/]*\.(?:ya?ml|json|toml)$/.test(normalized);
}

function allowedBash(command) {
  const c = String(command || "").trim();
  if (!c || c.length > 1200) return false;
  if (/(?:\brm\s|\brmdir\b|\bmkfs\b|\bshutdown\b|\breboot\b|\bsudo\b|\bsu\s|\bchmod\b|\bchown\b|\biptables\b|\bufw\b|curl[^\n]*\|[^\n]*sh|wget[^\n]*\|[^\n]*sh|\bdocker\s+rm\b|\bdocker\s+volume\s+rm\b|\bdocker\s+system\s+prune\b|\bdocker\s+image\s+rm\b|\bgit\s+(?:push|reset|clean)\b)/i.test(c)) return false;
  return /^(?:pwd|ls(?:\s|$)|cat\s+[^;|&]+|grep\s+[^;|&]+|find\s+[^;|&]+|git\s+(?:status|diff)(?:\s|$)|docker\s+(?:ps|inspect|logs)(?:\s|$)|docker\s+(?:restart)\s+comfyui$|docker\s+compose\s+(?:ps|logs|restart\s+comfyui|up\s+-d\s+comfyui)$|curl\s+(?:-fsS\s+)?http:\/\/127\.0\.0\.1:8188\/[^;&|]+)$/.test(c);
}

export default function (pi) {
  pi.on("tool_call", async event => {
    if (event.toolName === "read" || event.toolName === "grep" || event.toolName === "find") {
      const file = event.input.path || "";
      if (file && (!insideProject(file) || protectedPath(file))) {
        return { block: true, reason: "Bridge policy: access outside the project or to protected data is blocked." };
      }
    }
    if (event.toolName === "write" || event.toolName === "edit") {
      if (!allowedConfigPath(event.input.path)) {
        return { block: true, reason: "Bridge policy: only allowlisted ComfyUI/Docker config files can be modified." };
      }
    }
    if (event.toolName === "bash" && !allowedBash(event.input.command)) {
      return { block: true, reason: "Bridge policy: shell command is not allowlisted." };
    }
  });
}
