import { BridgeProcess } from "./bridge.js";

export { BridgeError } from "./bridge.js";

export class Sandbox {
  private bridge: BridgeProcess;

  constructor() {
    this.bridge = new BridgeProcess("sandbox-bridge");
  }

  async verifyCanary(): Promise<unknown> {
    return this.bridge.call("verify_canary");
  }

  close(): void {
    this.bridge.close();
  }
}
