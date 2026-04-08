import { writable, get } from 'svelte/store';
import type { Message, ShieldVerdict, Thought } from '../lib/types';

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
