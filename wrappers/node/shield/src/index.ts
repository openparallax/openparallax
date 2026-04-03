import { BridgeProcess } from "./bridge.js";
import type { ActionRequest, ShieldConfig, Verdict } from "./types.js";

export type { ActionRequest, ShieldConfig, Verdict, EvaluatorConfig } from "./types.js";

/**
 * 3-tier AI security pipeline for evaluating agent actions.
 *
 * Communicates with the Go shield-bridge binary over JSON-RPC.
 *
 * @example
 * ```typescript
 * import { Shield } from '@openparallax/shield';
 *
 * const shield = new Shield({ PolicyFile: 'policy.yaml' });
 * const verdict = await shield.evaluate({ Type: 'file_write', Payload: { path: '/etc/passwd' } });
 * console.log(verdict.decision); // "BLOCK"
 * shield.close();
 * ```
 */
export class Shield {
  private bridge: BridgeProcess;

  constructor(config?: ShieldConfig) {
    this.bridge = new BridgeProcess("shield-bridge");
    if (config) {
      this.configure(config).catch(() => {});
    }
  }

  /** Initialize the Shield pipeline with the given configuration. */
  async configure(config: ShieldConfig): Promise<void> {
    await this.bridge.call("configure", config);
  }

  /** Evaluate an action through the 3-tier security pipeline. */
  async evaluate(action: ActionRequest): Promise<Verdict> {
    const result = (await this.bridge.call("evaluate", action)) as Verdict;
    return result;
  }

  /** Shut down the bridge process. */
  close(): void {
    this.bridge.close();
  }
}
