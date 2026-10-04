import http from "node:http";
import { spawn } from "node:child_process";
import { resolve, join } from "node:path";
import { readdir, readFile, writeFile, unlink } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const HOST = "127.0.0.1";
const PORT = Number(process.env.PI_AGENT_BRIDGE_PORT || 8787);
const PROJECT_ROOT = resolve(process.env.WEB_STUDIO_PROJECT_ROOT || process.cwd());
const PI_BIN = process.env.PI_BIN || "pi";
const EXTENSION = fileURLToPath(new URL("./policy-extension.mjs", import.meta.url));
const MAX_BODY = 128 * 1024;
const TIMEOUT_MS = Number(process.env.PI_AGENT_TIMEOUT_MS || 180000);
const MAX_REPAIR_FILES = 100;
const MAX_SNAPSHOT_BYTES = 2 * 1024 * 1024;
let repairInProgress = false;

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

const REPAIRABLE_NAME = /(docker|compose|comfy|config|workflow)/i;
const REPAIRABLE_EXT = new Set([".yml", ".yaml", ".json", ".toml"]);
function isRepairablePath(filePath) {
  const name = filePath.split(/[\\/]/).pop() || "";
  const dot = name.lastIndexOf(".");
  const ext = dot >= 0 ? name.slice(dot).toLowerCase() : "";
  return REPAIRABLE_NAME.test(name) && REPAIRABLE_EXT.has(ext);
}
async function collectRepairableFiles(dir, out = []) {
  if (out.length >= MAX_REPAIR_FILES) return out;
  let entries = [];
  try { entries = await readdir(dir, { withFileTypes: true }); } catch { return out; }
  for (const entry of entries) {
    if (out.length >= MAX_REPAIR_FILES) break;
    if (entry.name === ".git" || entry.name === "node_modules" || entry.name === ".env") continue;
    const full = join(dir, entry.name);
    if (entry.isDirectory()) await collectRepairableFiles(full, out);
    else if (entry.isFile() && isRepairablePath(full)) out.push(full);
  }
  return out;
}
async function createRepairSnapshot() {
  const files = await collectRepairableFiles(PROJECT_ROOT);
  const snapshot = new Map(); let total = 0;
  for (const file of files) {
    try { const data = await readFile(file); if (data.byteLength > MAX_SNAPSHOT_BYTES || total + data.byteLength > MAX_SNAPSHOT_BYTES * MAX_REPAIR_FILES) continue; snapshot.set(file, data); total += data.byteLength; } catch {}
  }
  return snapshot;
}
async function rollbackRepair(snapshot) {
  const current = await collectRepairableFiles(PROJECT_ROOT);
  for (const file of current) if (!snapshot.has(file)) { try { await unlink(file); } catch {} }
  for (const [file, data] of snapshot) await writeFile(file, data);
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
    if (repairInProgress) return json(res, 409, { error: "another pi.dev repair is already in progress" });
    repairInProgress = true;
    const snapshot = await createRepairSnapshot();
    try {
      const result = await runPi(incident);
      const health = await healthCheck();
      if (result.ok && health.ok) return json(res, 200, { ok: true, agent: result, health, rolled_back: false, retest: { required: true, reason: "Web Studio must run the same test_comfy_flow after bridge repair" } });
      await rollbackRepair(snapshot);
      const rollbackHealth = await healthCheck();
      return json(res, 502, { ok: false, agent: result, health, rolled_back: true, rollback_health: rollbackHealth, retest: { required: true, reason: "Repair failed; project was restored to the pre-attempt snapshot" } });
    } finally { repairInProgress = false; }
  } catch (error) {
    return json(res, 400, { error: error instanceof Error ? error.message : String(error) });
  }
});

server.listen(PORT, HOST, () => process.stdout.write("pi.dev bridge listening on http://" + HOST + ":" + PORT + "\n"));
