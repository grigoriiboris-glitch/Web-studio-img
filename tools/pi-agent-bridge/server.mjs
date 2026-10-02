import http from "node:http";
import { spawn } from "node:child_process";
import { resolve } from "node:path";

const HOST = "127.0.0.1";
const PORT = Number(process.env.PI_AGENT_BRIDGE_PORT || 8787);
const PROJECT_ROOT = resolve(process.env.WEB_STUDIO_PROJECT_ROOT || process.cwd());
const PI_BIN = process.env.PI_BIN || "pi";
const EXTENSION = resolve(new URL("./policy-extension.mjs", import.meta.url).pathname);
const MAX_BODY = 128 * 1024;
const TIMEOUT_MS = Number(process.env.PI_AGENT_TIMEOUT_MS || 180000);

const json = (res, status, body) => {
  res.writeHead(status, { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" });
  res.end(JSON.stringify(body));
};

function readBody(req) {
  return new Promise((resolveBody, reject) => {
    let body = "";
    req.on("data", chunk => {
      body += chunk;
      if (Buffer.byteLength(body) > MAX_BODY) { reject(new Error("incident payload too large")); req.destroy(); }
    });
    req.on("end", () => { try { resolveBody(JSON.parse(body || "{}")); } catch { reject(new Error("invalid JSON")); } });
    req.on("error", reject);
  });
}

function validateIncident(incident) {
  if (!incident || incident.protocol !== "comfyui-self-healing/v1") throw new Error("unsupported incident protocol");
  if (typeof incident.projectId !== "string" || !incident.projectId) throw new Error("projectId is required");
  if (incident.container !== "comfyui") throw new Error("only the comfyui container is supported");
  if (!["connection", "container", "runtime", "model"].includes(incident.category)) throw new Error("unsupported repair category");
  if (!Array.isArray(incident.errors) || incident.errors.length > 20) throw new Error("invalid errors");
  if (typeof incident.workflowFingerprint !== "string" || incident.workflowFingerprint.length > 256) throw new Error("invalid workflowFingerprint");
}

function promptFor(incident) {
  return [
    "You are the local repair agent for Web Studio Img.",
    "Work only in the current project and on the local ComfyUI runtime.",
    "Diagnose the supplied incident, then repair only the technical cause.",
    "Never change artist intent, workflow semantics, Creative Brief, Artist Flow order, input assets, or color-reference contract.",
    "Do not access .env, secrets, credentials, private keys, .git, or files outside the project.",
    "Use only the tools allowed by the bridge policy extension.",
    "First inspect runtime status/logs and relevant compose/config/workflow files.",
    "For repair, make the smallest necessary allowlisted config/workflow change, restart ComfyUI only when needed, and verify /system_stats.",
    "Do not delete containers, volumes, models, images, repositories, or files.",
    "Do not install system packages or change host networking.",
    "End with exactly these headings: DIAGNOSIS, ACTIONS, HEALTH_CHECK, REMAINING.",
    "",
    "Structured incident:",
    JSON.stringify(incident, null, 2),
  ].join("\n");
}

function runPi(incident) {
  return new Promise((resolveRun) => {
    const child = spawn(PI_BIN, [
      "--mode", "rpc", "--no-session",
      "--tools", "read,bash,edit,write,grep,find,ls",
      "--extension", EXTENSION,
    ], {
      cwd: PROJECT_ROOT,
      env: { ...process.env, PI_AGENT_BRIDGE: "1", PI_AGENT_PROJECT_ROOT: PROJECT_ROOT },
      stdio: ["pipe", "pipe", "pipe"],
    });
    let stdout = "", stderr = "", settled = false;
    let timer;

    const finish = (result) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      try { child.kill("SIGTERM"); } catch {}
      resolveRun(result);
    };

    timer = setTimeout(() => finish({
      ok: false, status: "timeout", output: stdout.slice(-12000),
      error: "pi.dev agent timed out",
    }), TIMEOUT_MS);

    child.stdout.on("data", chunk => {
      stdout += chunk.toString();
      for (const line of chunk.toString().split("\n")) {
        if (!line.trim()) continue;
        try {
          const event = JSON.parse(line);
          if (event.type === "agent_settled") {
            finish({
              ok: !stdout.includes('"stopReason":"error"') && !stdout.includes('"errorMessage"'),
              status: "settled", output: stdout.slice(-12000), stderr: stderr.slice(-4000),
            });
          }
        } catch {}
      }
    });
    child.stderr.on("data", chunk => { stderr += chunk.toString(); });
    child.on("error", error => finish({ ok: false, status: "spawn_error", error: error.message, stderr }));
    child.on("exit", (code, signal) => {
      if (!settled && code !== null) finish({
        ok: code === 0, status: code === 0 ? "exited" : "failed",
        output: stdout.slice(-12000), stderr: stderr.slice(-4000), exitCode: code, signal,
      });
    });

    child.stdin.write(JSON.stringify({ id: "repair-1", type: "set_auto_retry", enabled: false }) + "\n");
    child.stdin.write(JSON.stringify({ id: "repair-2", type: "prompt", message: promptFor(incident) }) + "\n");
  });
}

async function healthCheck() {
  try {
    const response = await fetch("http://127.0.0.1:8188/system_stats", { signal: AbortSignal.timeout(5000) });
    return { ok: response.ok, status: response.status };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
}

const server = http.createServer(async (req, res) => {
  if (req.method === "GET" && req.url === "/health") return json(res, 200, { ok: true, host: HOST, port: PORT, projectRoot: PROJECT_ROOT });
  if (req.method !== "POST" || req.url !== "/repair") return json(res, 404, { error: "not_found" });
  try {
    const incident = await readBody(req);
    validateIncident(incident);
    if (incident.confirmed !== true) return json(res, 400, { error: "explicit repair confirmation is required" });
    const result = await runPi(incident);
    const health = await healthCheck();
    return json(res, result.ok && health.ok ? 200 : 502, {
      ok: result.ok && health.ok, agent: result, health,
      retest: { required: true, reason: "Web Studio must run the same test_comfy_flow after bridge repair" },
    });
  } catch (error) {
    return json(res, 400, { error: error instanceof Error ? error.message : String(error) });
  }
});

server.listen(PORT, HOST, () => process.stdout.write("pi.dev bridge listening on http://" + HOST + ":" + PORT + "\n"));
