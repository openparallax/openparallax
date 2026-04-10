import { BridgeProcess } from "./bridge.js";
import type { SearchResult } from "./types.js";

export type { SearchResult };
export { BridgeError } from "./bridge.js";

export class Memory {
  private bridge: BridgeProcess;

  constructor(workspace?: string, dbPath?: string) {
    this.bridge = new BridgeProcess("memory-bridge");
    if (workspace && dbPath) {
      this.configure(workspace, dbPath).catch(() => {});
    }
  }

  async configure(workspace: string, dbPath: string): Promise<void> {
    await this.bridge.call("configure", { workspace, db_path: dbPath });
  }

  async search(query: string, limit = 10): Promise<SearchResult[]> {
    const result = await this.bridge.call("search", { query, limit });
    return (result as SearchResult[]) ?? [];
  }

  async read(fileType: string): Promise<string> {
    const result = (await this.bridge.call("read", { file_type: fileType })) as Record<string, string> | null;
    return result?.content ?? "";
  }

  close(): void {
    this.bridge.close();
  }
}
