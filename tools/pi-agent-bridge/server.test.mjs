import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("bridge is localhost-only and requires confirmation", async () => {
  const source = await readFile(new URL("./server.mjs", import.meta.url), "utf8");
  assert.match(source, /127\\.0\\.0\\.1/);
  assert.match(source, /incident\\.confirmed !== true/);
});

test("policy blocks dangerous and secret access", async () => {
  const source = await readFile(new URL("./policy-extension.mjs", import.meta.url), "utf8");
  assert.match(source, /\\.env/);
  assert.match(source, /docker\\s\\+rm/);
  assert.match(source, /sudo/);
});
