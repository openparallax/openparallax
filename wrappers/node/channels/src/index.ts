import { BridgeProcess } from "./bridge.js";
import type { ChannelMessage } from "./types.js";

export type { ChannelMessage };
export { BridgeError } from "./bridge.js";

export class Channels {
  private bridge: BridgeProcess;

  constructor() {
    this.bridge = new BridgeProcess("channels-bridge");
  }

  async splitMessage(content: string, maxLength = 4096): Promise<string[]> {
    const result = await this.bridge.call("split_message", {
      content,
      max_length: maxLength,
    });
    return (result as string[]) ?? [];
  }

  async formatMessage(text: string, format = 0): Promise<ChannelMessage> {
    const result = await this.bridge.call("format_message", { text, format });
    return (result as ChannelMessage) ?? { Text: text, Format: format };
  }

  close(): void {
    this.bridge.close();
  }
}
