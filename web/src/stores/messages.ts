import { writable, get } from 'svelte/store';
import type { Message, ToolCall, ShieldVerdict, Artifact } from '../lib/types';

export const messages = writable<Message[]>([]);
export const toolCalls = writable<ToolCall[]>([]);
export const artifacts = writable<Artifact[]>([]);
export const shieldLog = writable<ShieldVerdict[]>([]);
export const streaming = writable(false);
export const streamingText = writable('');

export function setStreaming(value: boolean) {
  streaming.set(value);
  if (value) streamingText.set('');
}

export function appendToken(text: string) {
  streamingText.update(t => t + text);
}

export function addToolCall(tc: ToolCall) {
  toolCalls.update(calls => [...calls, tc]);
}

export function updateToolCallVerdict(verdict: ShieldVerdict) {
  shieldLog.update(log => [verdict, ...log]);
  toolCalls.update(calls => {
    const updated = [...calls];
    // Update the most recent tool call that matches.
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
  toolCalls.update(calls => {
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
}

export function finalizeResponse(content: string) {
  const currentText = get(streamingText);
  const finalContent = content || currentText;

  messages.update(msgs => [...msgs, {
    id: 'msg-' + Date.now(),
    session_id: '',
    role: 'assistant' as const,
    content: finalContent,
    timestamp: new Date().toISOString(),
  }]);

  streamingText.set('');
}

export function addUserMessage(content: string) {
  messages.update(msgs => [...msgs, {
    id: 'msg-' + Date.now(),
    session_id: '',
    role: 'user' as const,
    content,
    timestamp: new Date().toISOString(),
  }]);
  // Clear tool calls for the new turn.
  toolCalls.set([]);
}

export function clearMessages() {
  messages.set([]);
  toolCalls.set([]);
  artifacts.set([]);
  shieldLog.set([]);
  streamingText.set('');
  streaming.set(false);
}
