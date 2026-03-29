import { writable, get } from 'svelte/store';
import type { Message, ToolCall, ShieldVerdict, Artifact, Thought } from '../lib/types';
import { openArtifactTab } from './artifacts';

export const messages = writable<Message[]>([]);
export const pendingToolCalls = writable<ToolCall[]>([]);
export const artifacts = writable<Artifact[]>([]);
export const shieldLog = writable<ShieldVerdict[]>([]);
export const streaming = writable(false);
export const streamingText = writable('');

export function setStreaming(value: boolean) {
  streaming.set(value);
}

export function startNewStream() {
  streaming.set(true);
  streamingText.set('');
}

export function appendToken(text: string) {
  streamingText.update(t => t + text);
}

export function clearStreamingText() {
  streamingText.set('');
}

export function addToolCall(tc: ToolCall) {
  pendingToolCalls.update(calls => [...calls, tc]);
}

export function updateToolCallVerdict(verdict: ShieldVerdict) {
  shieldLog.update(log => [verdict, ...log]);
  pendingToolCalls.update(calls => {
    const updated = [...calls];
    for (let i = updated.length - 1; i >= 0; i--) {
      if (updated[i].toolName === verdict.toolName && !updated[i].shieldVerdict) {
        updated[i] = { ...updated[i], shieldVerdict: verdict };
        break;
      }
    }
    return updated;
  });
}

export function completeToolCall(result: { tool_name: string; success: boolean; summary: string }) {
  pendingToolCalls.update(calls => {
    const updated = [...calls];
    for (let i = updated.length - 1; i >= 0; i--) {
      if (updated[i].toolName === result.tool_name && !updated[i].result) {
        updated[i] = { ...updated[i], result: { success: result.success, summary: result.summary } };
        break;
      }
    }
    return updated;
  });
}

export function addArtifact(artifact: Artifact) {
  artifacts.update(a => [...a, artifact]);
  openArtifactTab(artifact);
}

export function finalizeResponse(content: string, thoughts?: Thought[]) {
  const currentText = get(streamingText);
  const finalContent = content || currentText;

  const pending = get(pendingToolCalls);
  let finalThoughts: Thought[] | undefined;

  if (pending.length > 0) {
    finalThoughts = pending.map(tc => ({
      stage: 'tool_call' as const,
      summary: `${tc.toolName} — ${tc.summary}`,
      detail: {
        tool_name: tc.toolName,
        success: tc.result?.success,
        shield: tc.shieldVerdict?.decision,
        shield_tier: tc.shieldVerdict?.tier,
        shield_reasoning: tc.shieldVerdict?.reasoning,
        result_summary: tc.result?.summary,
      },
    }));
  } else if (thoughts && thoughts.length > 0) {
    finalThoughts = thoughts;
  }

  messages.update(msgs => [...msgs, {
    id: 'msg-' + Date.now(),
    session_id: '',
    role: 'assistant' as const,
    content: finalContent,
    timestamp: new Date().toISOString(),
    thoughts: finalThoughts,
  }]);

  streamingText.set('');
  pendingToolCalls.set([]);
}

export function loadMessages(msgs: Message[]) {
  messages.set(msgs);
  const restored: Artifact[] = [];
  for (const msg of msgs) {
    if (msg.artifacts && msg.artifacts.length > 0) {
      for (const a of msg.artifacts) {
        restored.push(a);
        openArtifactTab(a);
      }
    }
  }
  if (restored.length > 0) {
    artifacts.set(restored);
  }
}

export function addUserMessage(content: string) {
  messages.update(msgs => [...msgs, {
    id: 'msg-' + Date.now(),
    session_id: '',
    role: 'user' as const,
    content,
    timestamp: new Date().toISOString(),
  }]);
  pendingToolCalls.set([]);
}

export function clearMessages() {
  messages.set([]);
  pendingToolCalls.set([]);
  artifacts.set([]);
  shieldLog.set([]);
  streamingText.set('');
  streaming.set(false);
}
