import { BridgeProcess } from "./bridge.js";
import type { Entry, LogEntry, Query, VerifyResult } from "./types.js";

export type { Entry, LogEntry, Query, VerifyResult };
export { BridgeError } from "./bridge.js";

export class Audit {
  private bridge: BridgeProcess;

  constructor(path?: string) {
    this.bridge = new BridgeProcess("audit-bridge");
    if (path) {
      this.configure(path).catch(() => {});
    }
  }

  async configure(path: string): Promise<void> {
    await this.bridge.call("configure", { path });
  }

  async log(entry: Entry): Promise<void> {
    await this.bridge.call("log", entry);
  }

  async verify(path?: string): Promise<VerifyResult> {
    const params = path ? { path } : {};
    return (await this.bridge.call("verify", params)) as VerifyResult;
  }

  async query(path: string, query?: Query): Promise<LogEntry[]> {
    const params: Record<string, unknown> = { path };
    if (query) params.query = query;
    const result = await this.bridge.call("query", params);
    return (result as LogEntry[]) ?? [];
  }

  close(): void {
    this.bridge.close();
  }
}
