#!/usr/bin/env node
/**
 * Postinstall script — downloads the Go bridge binary for the current platform.
 * Follows the same pattern as install.sh: detect platform, download archive,
 * verify checksum, extract the specific binary.
 */

import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync, chmodSync, createWriteStream } from "node:fs";
import { join, dirname } from "node:path";
import { arch, platform } from "node:os";
import { execSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { get } from "node:https";

const REPO = "openparallax/openparallax";
const __dirname = dirname(fileURLToPath(import.meta.url));
const pkgJson = JSON.parse(readFileSync(join(__dirname, "..", "package.json"), "utf8"));

// Derive the bridge binary name from the package name.
// @openparallax/shield → openparallax-shield-bridge
const scope = pkgJson.name.replace("@openparallax/", "");
const BINARY_NAME = `openparallax-${scope}-bridge`;

const GOOS = { linux: "linux", darwin: "darwin", win32: "windows" };
const GOARCH = { x64: "amd64", arm64: "arm64" };

async function main() {
  const binDir = join(__dirname, "..", "bin");
  const ext = platform() === "win32" ? ".exe" : "";
  const dest = join(binDir, `${BINARY_NAME}${ext}`);

  // Already downloaded.
  if (existsSync(dest)) {
    return;
  }

  const os = GOOS[platform()] ?? platform();
  const ar = GOARCH[arch()] ?? "amd64";
  const version = resolveVersion();
  if (!version) {
    console.log(`Could not determine version — skipping bridge download.`);
    console.log(`Install OpenParallax (curl -sSL https://get.openparallax.dev | sh) or place '${BINARY_NAME}' in your PATH.`);
    return;
  }

  const versionNum = version.replace(/^v/, "");
  const archiveExt = os === "windows" ? "zip" : "tar.gz";
  const archiveName = `openparallax-bridges_${versionNum}_${os}_${ar}.${archiveExt}`;
  const baseUrl = `https://github.com/${REPO}/releases/download/${version}`;

  console.log(`Downloading ${BINARY_NAME} (${os}/${ar})...`);

  try {
    // Download archive and checksums.
    const archiveBuf = await download(`${baseUrl}/${archiveName}`);
    const checksumsTxt = await download(`${baseUrl}/checksums.txt`).then(b => b.toString());

    // Verify checksum.
    const expected = findChecksum(checksumsTxt, archiveName);
    if (expected) {
      const actual = createHash("sha256").update(archiveBuf).digest("hex");
      if (actual !== expected) {
        console.error(`Checksum mismatch for ${archiveName}`);
        return;
      }
      console.log("Checksum verified.");
    }

    // Extract the specific binary.
    const binaryBuf = await extractBinary(archiveBuf, `${BINARY_NAME}${ext}`, archiveExt);
    if (!binaryBuf) {
      console.error(`Binary ${BINARY_NAME}${ext} not found in archive.`);
      return;
    }

    // Install.
    mkdirSync(binDir, { recursive: true });
    writeFileSync(dest, binaryBuf);
    if (platform() !== "win32") {
      chmodSync(dest, 0o755);
    }
    console.log(`Installed ${BINARY_NAME} to ${dest}`);

  } catch (err) {
    console.log(`Auto-download failed: ${err.message}`);
    console.log(`Install OpenParallax (curl -sSL https://get.openparallax.dev | sh) or place '${BINARY_NAME}' in your PATH.`);
  }
}

function resolveVersion() {
  // Read from package.json.
  const v = pkgJson.version;
  if (v && v !== "0.0.0") return `v${v}`;

  // Fall back to latest release via GitHub API.
  try {
    const out = execSync(
      `curl -sf https://api.github.com/repos/${REPO}/releases/latest`,
      { encoding: "utf8", timeout: 10000 }
    );
    const data = JSON.parse(out);
    return data.tag_name || null;
  } catch {
    return null;
  }
}

function findChecksum(checksumsTxt, archiveName) {
  for (const line of checksumsTxt.split("\n")) {
    if (line.includes(archiveName)) {
      return line.split(/\s+/)[0];
    }
  }
  return null;
}

function download(url) {
  return new Promise((resolve, reject) => {
    const doGet = (u, redirects = 0) => {
      if (redirects > 5) return reject(new Error("Too many redirects"));
      get(u, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return doGet(res.headers.location, redirects + 1);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode} for ${u}`));
        }
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      }).on("error", reject);
    };
    doGet(url);
  });
}

async function extractBinary(archiveBuf, binaryName, archiveExt) {
  if (archiveExt === "zip") {
    // Minimal ZIP extraction — find the binary entry.
    // For simplicity, shell out to unzip.
    const { mkdtempSync, rmSync } = await import("node:fs");
    const { tmpdir } = await import("node:os");
    const tmp = mkdtempSync(join(tmpdir(), "op-bridge-"));
    try {
      const archivePath = join(tmp, "archive.zip");
      writeFileSync(archivePath, archiveBuf);
      execSync(`unzip -o "${archivePath}" -d "${tmp}"`, { stdio: "ignore" });
      // Find the binary.
      const { readdirSync } = await import("node:fs");
      for (const f of readdirSync(tmp, { recursive: true })) {
        if (f.endsWith(binaryName)) {
          return readFileSync(join(tmp, f));
        }
      }
      return null;
    } finally {
      rmSync(tmp, { recursive: true, force: true });
    }
  } else {
    // tar.gz — shell out to tar.
    const { mkdtempSync, rmSync } = await import("node:fs");
    const { tmpdir } = await import("node:os");
    const tmp = mkdtempSync(join(tmpdir(), "op-bridge-"));
    try {
      const archivePath = join(tmp, "archive.tar.gz");
      writeFileSync(archivePath, archiveBuf);
      execSync(`tar xzf "${archivePath}" -C "${tmp}"`, { stdio: "ignore" });
      const { readdirSync } = await import("node:fs");
      for (const f of readdirSync(tmp, { recursive: true })) {
        if (f.endsWith(binaryName)) {
          return readFileSync(join(tmp, f));
        }
      }
      return null;
    } finally {
      rmSync(tmp, { recursive: true, force: true });
    }
  }
}

main().catch((err) => {
  console.log(`Bridge download skipped: ${err.message}`);
});
