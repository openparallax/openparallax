import { spawn, execSync, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { arch, platform } from "node:os";
import { fileURLToPath } from "node:url";

interface JsonRpcRequest {
  jsonrpc: "2.0";
  id: number;
  method: string;
  params?: unknown;
}

interface JsonRpcResponse {
  jsonrpc: "2.0";
  id: number;
  result?: unknown;
  error?: { code: number; message: string };
}

/** Manages a Go bridge binary subprocess communicating via JSON-RPC over stdio. */
export class BridgeProcess {
  private proc: ChildProcess | null = null;
  private nextId = 0;
  private pending = new Map<
    number,
    { resolve: (v: unknown) => void; reject: (e: Error) => void }
  >();
  private binaryPath: string;

  constructor(binaryName: string) {
    this.binaryPath = findBinary(binaryName);
    this.start();
  }

  /** Send a JSON-RPC request and return the result. */
  async call(method: string, params?: unknown): Promise<unknown> {
    if (!this.proc || this.proc.killed) {
      this.start();
    }

    const id = ++this.nextId;
    const request: JsonRpcRequest = { jsonrpc: "2.0", id, method };
    if (params !== undefined) {
      request.params = params;
    }

    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.proc!.stdin!.write(JSON.stringify(request) + "\n");
    });
  }

  /** Terminate the bridge process. */
  close(): void {
    if (this.proc && !this.proc.killed) {
      this.proc.stdin?.end();
      this.proc.kill();
      this.proc = null;
    }
    for (const [, { reject }] of this.pending) {
      reject(new Error("bridge closed"));
    }
    this.pending.clear();
  }

  private start(): void {
    this.proc = spawn(this.binaryPath, [], {
      stdio: ["pipe", "pipe", "pipe"],
    });

    const rl = createInterface({ input: this.proc.stdout! });
    rl.on("line", (line: string) => {
      try {
        const response: JsonRpcResponse = JSON.parse(line);
        const handler = this.pending.get(response.id);
        if (!handler) return;
        this.pending.delete(response.id);

        if (response.error) {
          handler.reject(
            new BridgeError(response.error.message, response.error.code)
          );
        } else {
          handler.resolve(response.result);
        }
      } catch {
        // Ignore non-JSON lines.
      }
    });

    this.proc.on("exit", () => {
      for (const [, { reject }] of this.pending) {
        reject(new Error("bridge process exited"));
      }
      this.pending.clear();
    });
  }
}

/** Error returned by the bridge binary. */
export class BridgeError extends Error {
  code: number;
  constructor(message: string, code: number = -1) {
    super(message);
    this.name = "BridgeError";
    this.code = code;
  }
}

function findBinary(name: string): string {
  const ext = process.platform === "win32" ? ".exe" : "";
  const dir = fileURLToPath(new URL("../bin", import.meta.url));

  // Check package bin/ directory.
  const pkgBinary = join(dir, `${name}${ext}`);
  if (existsSync(pkgBinary)) return pkgBinary;

  // Check platform-specific name.
  const goos: Record<string, string> = {
    linux: "linux",
    darwin: "darwin",
    win32: "windows",
  };
  const goarch: Record<string, string> = { x64: "amd64", arm64: "arm64" };
  const os = goos[process.platform] ?? process.platform;
  const ar = goarch[arch()] ?? "amd64";
  const platformBinary = join(dir, `${name}-${os}-${ar}${ext}`);
  if (existsSync(platformBinary)) return platformBinary;

  // Fall back to PATH.
  try {
    const which = execSync(`which ${name}`, { encoding: "utf8" }).trim();
    if (which) return which;
  } catch {
    // Not in PATH.
  }

  throw new Error(
    `Bridge binary '${name}' not found. ` +
      `Install it or place '${name}' in your PATH.`
  );
}
