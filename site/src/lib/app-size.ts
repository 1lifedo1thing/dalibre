import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

// Size of the Dalibre release binary in MB, measured by building it the way
// releases are built (stripped, trimpath) when the site builds. Astro runs
// from site/, so the Go module is one level up.
function measure(): number {
  const dir = mkdtempSync(join(tmpdir(), "dalibre-size-"));
  try {
    const out = join(dir, "dalibre");
    execFileSync("go", ["build", "-trimpath", "-ldflags=-s -w", "-o", out, "./cmd/dalibre"], {
      cwd: resolve(process.cwd(), ".."),
      stdio: ["ignore", "ignore", "inherit"],
    });
    return Math.round(statSync(out).size / (1024 * 1024));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

export const dalibreMB = measure();
