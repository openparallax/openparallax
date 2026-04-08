import { writable, get } from 'svelte/store';
import type { Message, ShieldVerdict, Thought } from '../lib/types';

// A single step in the streaming timeline. Reasoning text lives in the
// streaming bubble itself (`streamingText`), not here — the dropdown
// holds tool calls only.
export interface PipelineStep {
  type: 'tool_call';
  toolName: string;
  summary?: string;
  shieldVerdict?: ShieldVerdict;
  result?: { success: boolean; summary: string };
}

export const messages = writable<Message[]>([]);
export const pendingSteps = writable<PipelineStep[]>([]);
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

// addToolCall registers a new tool call in the pending steps and inserts
// a blank-line separator into the streaming bubble so the next reasoning
// fragment renders below the current one. The streaming text is NOT
// cleared — reasoning fragments accumulate visibly in the bubble for the
// duration of the message and are persisted as part of the final assistant
// content (the engine's loop builds the same separated content for the
// SQLite row, so a refresh shows the same thing).
export function addToolCall(toolName: string, summary: string) {
  streamingText.update(t => {
    if (t === '') return t;
    if (t.endsWith('\n\n')) return t;
    if (t.endsWith('\n')) return t + '\n';
    return t + '\n\n';
  });
  pendingSteps.update(steps => [...steps, { type: 'tool_call', toolName, summary }]);
}

export function updateToolCallVerdict(verdict: ShieldVerdict) {
  shieldLog.update(log => [verdict, ...log]);
  pendingSteps.update(steps => {
    const updated = [...steps];
    for (let i = updated.length - 1; i >= 0; i--) {
      const s = updated[i];
      if (s.toolName === verdict.toolName && !s.shieldVerdict) {
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
      if (s.toolName === result.tool_name && !s.result) {
        updated[i] = { ...s, result: { success: result.success, summary: result.summary } };
        break;
      }
    }
    return updated;
  });
}

export function finalizeResponse(content: string, thoughts?: Thought[]) {
  // The engine builds `content` as the merged reasoning + final answer
  // (see internal/agent/loop.go presentation buffer), so we use it
  // verbatim. The streaming bubble was already showing the same thing
  // live; nothing gets stripped.
  const finalContent = (content || get(streamingText)).trim();

  const steps = get(pendingSteps);
  let finalThoughts: Thought[] | undefined;

  // The dropdown holds tool calls only. Reasoning entries from the
  // engine's thoughts list are filtered out at render time (and we
  // also drop them here so the persisted message stays lean).
  if (thoughts && thoughts.length > 0) {
    const toolThoughts = thoughts.filter(t => t.stage === 'tool_call');
    finalThoughts = toolThoughts.map(t => {
      if (!t.summary) return t;
      const step = steps.find(s => s.toolName && t.summary.includes(s.toolName));
      if (!step) return t;
      const merged: Record<string, any> = { ...t.detail };
      if (!merged.tool_name) merged.tool_name = step.toolName;
      if (merged.success === undefined && step.result) merged.success = step.result.success;
      if (!merged.shield && step.shieldVerdict) merged.shield = step.shieldVerdict.decision;
      if (!merged.shield_tier && step.shieldVerdict) merged.shield_tier = step.shieldVerdict.tier;
      if (!merged.result_summary && step.result) merged.result_summary = step.result.summary;
      return { ...t, detail: merged };
    });
    if (finalThoughts.length === 0) finalThoughts = undefined;
  } else if (steps.length > 0) {
    finalThoughts = steps.map(s => ({
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
    }));
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
  pendingSteps.set([]);
}

export function loadMessages(msgs: Message[]) {
  messages.set(msgs);
}

// failPendingToolCall marks the most recent in-progress tool call as a
// failure. Used when the engine emits a terminal event for a tool (such as
// otr_blocked) without a matching action_completed, leaving an orphaned
// "started" card. The match is implicit-by-recency because action processing
// is sequential per session, so the last unfinished tool_call is guaranteed
// to be the one the terminal event refers to.
export function failPendingToolCall(summary: string) {
  pendingSteps.update(steps => {
    const updated = [...steps];
    for (let i = updated.length - 1; i >= 0; i--) {
      const s = updated[i];
      if (s.type === 'tool_call' && !s.result) {
        updated[i] = { ...s, result: { success: false, summary } };
        break;
      }
    }
    return updated;
  });
}

// failStream is called when the pipeline aborts mid-message with an error
// (LLM provider down, agent crash, transport failure). It preserves any
// partial content the agent already produced, appends a system error line
// so the user can see what happened, and clears stream state so the input
// is unblocked. For non-OTR sessions the engine also writes a matching
// system message to SQLite so the line survives a refresh; for OTR
// sessions the line lives only in memory.
export function failStream(errorMessage: string, recoverable: boolean = true) {
  const partial = get(streamingText).trim();
  const steps = get(pendingSteps);
  if (partial || steps.length > 0) {
    finalizeResponse(partial);
  }
  let line = '⚠ error: ' + errorMessage;
  if (!recoverable) {
    line += ' (unrecoverable — the agent may need /restart)';
  }
  addSystemMessage(line);
  streaming.set(false);
  streamingText.set('');
  pendingSteps.set([]);
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
}

export function clearMessages() {
  messages.set([]);
  pendingSteps.set([]);
  shieldLog.set([]);
  streamingText.set('');
  streaming.set(false);
}
