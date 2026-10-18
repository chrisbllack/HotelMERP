#!/usr/bin/env node

const args = process.argv.slice(2);
const valueAfter = (name, fallback) => {
  const index = args.indexOf(name);
  return index >= 0 && args[index + 1] ? args[index + 1] : fallback;
};

const url = valueAfter("--url", process.env.INFINEON_HEALTH_URL || "http://127.0.0.1:8080/api/v1/health");
const timeoutMs = Number(valueAfter("--timeout-ms", process.env.INFINEON_HEALTH_TIMEOUT_MS || "5000"));

if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
  console.error("Health check timeout must be a positive number of milliseconds.");
  process.exit(2);
}

try {
  const response = await fetch(url, { signal: AbortSignal.timeout(timeoutMs) });
  const body = await response.text();
  if (!response.ok) {
    console.error(`Health check failed: ${url} returned HTTP ${response.status}.`);
    process.exit(1);
  }
  try {
    JSON.parse(body);
  } catch {
    console.error(`Health check failed: ${url} returned non-JSON data.`);
    process.exit(1);
  }
  console.log(`Health check passed: ${url} returned HTTP ${response.status}.`);
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Health check failed: could not reach ${url} within ${timeoutMs}ms (${message}).`);
  process.exit(1);
}
