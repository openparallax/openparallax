import { fileURLToPath } from "node:url";
import { BridgeProcess } from "./bridge.js";
import type { ActionRequest, ShieldConfig, Verdict } from "./types.js";

export type { ActionRequest, ShieldConfig, Verdict, EvaluatorConfig } from "./types.js";

const DEFAULT_POLICY = fileURLToPath(
  new URL("../policies/default.yaml", import.meta.url),
);

/**
 * 4-tier AI security pipeline for evaluating agent actions.
 *
 * Communicates with the Go shield-bridge binary over JSON-RPC.
 *
 * @example
 * ```typescript
 * import { Shield } from '@openparallax/shield';
 *
 * // Uses the bundled default policy automatically:
 * const shield = new Shield();
 * const verdict = await shield.evaluate({ Type: 'file_write', Payload: { path: '/etc/passwd' } });
 * console.log(verdict.decision); // "BLOCK"
 * shield.close();
 * ```
 */
export class Shield {
  private bridge: BridgeProcess;

  constructor(config?: ShieldConfig) {
    this.bridge = new BridgeProcess("shield-bridge");
    const resolved: ShieldConfig = {
      PolicyFile: DEFAULT_POLICY,
      ...config,
    };
    this.configure(resolved).catch(() => {});
  }

  /** Initialize the Shield pipeline with the given configuration. */
  async configure(config: ShieldConfig): Promise<void> {
    await this.bridge.call("configure", config);
  }

  /** Evaluate an action through the 4-tier security pipeline. */
  async evaluate(action: ActionRequest): Promise<Verdict> {
    const result = (await this.bridge.call("evaluate", action)) as Verdict;
    return result;
  }

  /** Shut down the bridge process. */
  close(): void {
    this.bridge.close();
  }
}
