import { writable, get } from 'svelte/store';
import type { Message, ShieldVerdict, Artifact, Thought } from '../lib/types';
import { openArtifactTab } from './artifacts';

// A single step in the streaming timeline: either reasoning text or a tool call.
export interface PipelineStep {
  type: 'reasoning' | 'tool_call';
  // Reasoning: the text content. Tool call: tool name.
  toolName?: string;
  summary?: string;
  content?: string;
  shieldVerdict?: ShieldVerdict;
  result?: { success: boolean; summary: string };
}

export const messages = writable<Message[]>([]);
export const pendingSteps = writable<PipelineStep[]>([]);
export const pendingArtifacts = writable<Artifact[]>([]);
export const artifacts = writable<Artifact[]>([]);
export const shieldLog = writable<ShieldVerdict[]>([]);
export const streaming = writable(false);
export const streamingText = writable('');

export interface Tier3Request {
  actionId: string;
  toolName: string;
  target: string;
  reasoning: string;
  timeoutSecs: number;
}
export const pendingApprovals = writable<Tier3Request[]>([]);

export function addTier3Request(req: Tier3Request) {
  pendingApprovals.update(list => [...list, req]);
}

export function removeTier3Request(actionId: string) {
  pendingApprovals.update(list => list.filter(r => r.actionId !== actionId));
}

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

// Flush accumulated streaming text into the timeline as a reasoning step,
// then add the tool call — preserving chronological order.
export function addToolCallWithFlush(toolName: string, summary: string) {
  const text = get(streamingText).trim();
  pendingSteps.update(steps => {
    const next = [...steps];
    if (text) {
      next.push({ type: 'reasoning', content: text });
    }
    next.push({ type: 'tool_call', toolName, summary });
    return next;
  });
  streamingText.set('');
}

export function updateToolCallVerdict(verdict: ShieldVerdict) {
  shieldLog.update(log => [verdict, ...log]);
  pendingSteps.update(steps => {
    const updated = [...steps];
    for (let i = updated.length - 1; i >= 0; i--) {
      const s = updated[i];
      if (s.type === 'tool_call' && s.toolName === verdict.toolName && !s.shieldVerdict) {
        updated[i] = { ...s, shieldVerdict: verdict };
        break;
      }
    }
    return updated;
  });
}

export function completeToolCall(result: { tool_name: string; success: boolean; summary: string }) {
  pendingSteps.update(steps => {
    const updated = [...steps];
    for (let i = updated.length - 1; i >= 0; i--) {
      const s = updated[i];
      if (s.type === 'tool_call' && s.toolName === result.tool_name && !s.result) {
        updated[i] = { ...s, result: { success: result.success, summary: result.summary } };
        break;
      }
    }
    return updated;
  });
}

export function addArtifact(artifact: Artifact, showPanel = true) {
  artifacts.update(a => [...a, artifact]);
  if (artifact.type === 'file') {
    pendingArtifacts.update(a => [...a, artifact]);
  }
  if (showPanel) openArtifactTab(artifact, true);
}

export function finalizeResponse(content: string, thoughts?: Thought[]) {
  const currentText = get(streamingText);
  let finalContent = content || currentText;

  const steps = get(pendingSteps);

  // Strip reasoning text from the final message content so it only
  // appears in the thinking dropdown, not duplicated in the body.
  for (const s of steps) {
    if (s.type === 'reasoning' && s.content) {
      finalContent = finalContent.replace(s.content, '').trim();
    }
  }
  if (thoughts) {
    for (const t of thoughts) {
      if (t.stage === 'reasoning' && t.summary) {
        finalContent = finalContent.replace(t.summary, '').trim();
      }
    }
  }

  let finalThoughts: Thought[] | undefined;

  if (thoughts && thoughts.length > 0) {
    // Server-side thoughts exist. Enrich tool_call entries with live
    // shield/result data from pending steps, but DO NOT overwrite
    // fields already set by the server (e.g. detail.shield = "BLOCK").
    finalThoughts = thoughts.map(t => {
      if (t.stage !== 'tool_call' || !t.summary) return t;
      const step = steps.find(s => s.type === 'tool_call' && s.toolName && t.summary.includes(s.toolName));
      if (!step) return t;
      const merged: Record<string, any> = { ...t.detail };
      if (!merged.tool_name) merged.tool_name = step.toolName;
      if (merged.success === undefined && step.result) merged.success = step.result.success;
      if (!merged.shield && step.shieldVerdict) merged.shield = step.shieldVerdict.decision;
      if (!merged.shield_tier && step.shieldVerdict) merged.shield_tier = step.shieldVerdict.tier;
      if (!merged.result_summary && step.result) merged.result_summary = step.result.summary;
      return { ...t, detail: merged };
    });
  } else if (steps.length > 0) {
    // No server thoughts — build from pipeline steps.
    finalThoughts = steps.map(s => {
      if (s.type === 'reasoning') {
        return { stage: 'reasoning' as const, summary: s.content || '' };
      }
      return {
        stage: 'tool_call' as const,
        summary: `${s.toolName} — ${s.summary || ''}`,
        detail: {
          tool_name: s.toolName,
          success: s.result?.success,
          shield: s.shieldVerdict?.decision,
          shield_tier: s.shieldVerdict?.tier,
          shield_reasoning: s.shieldVerdict?.reasoning,
          result_summary: s.result?.summary,
        },
      };
    });
  }

  const msgArtifacts = get(pendingArtifacts);

  messages.update(msgs => [...msgs, {
    id: 'msg-' + Date.now(),
    session_id: '',
    role: 'assistant' as const,
    content: finalContent,
    timestamp: new Date().toISOString(),
    thoughts: finalThoughts,
    artifacts: msgArtifacts.length > 0 ? msgArtifacts : undefined,
  }]);

  streamingText.set('');
  pendingSteps.set([]);
  pendingArtifacts.set([]);
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

export function addSystemMessage(content: string) {
  messages.update(msgs => [...msgs, {
    id: 'sys-' + Date.now(),
    session_id: '',
    role: 'system' as const,
    content,
    timestamp: new Date().toISOString(),
  }]);
}

export function addUserMessage(content: string) {
  messages.update(msgs => [...msgs, {
    id: 'msg-' + Date.now(),
    session_id: '',
    role: 'user' as const,
    content,
    timestamp: new Date().toISOString(),
  }]);
  pendingSteps.set([]);
  pendingArtifacts.set([]);
}

export function clearMessages() {
  messages.set([]);
  pendingSteps.set([]);
  pendingArtifacts.set([]);
  artifacts.set([]);
  shieldLog.set([]);
  streamingText.set('');
  streaming.set(false);
}
