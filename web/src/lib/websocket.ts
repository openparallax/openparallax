import { get } from 'svelte/store';
import { connected, reconnecting } from '../stores/connection';
import { currentSessionId } from '../stores/session';
import { appendToken, addToolCallWithFlush, updateToolCallVerdict, completeToolCall, finalizeResponse, setStreaming, startNewStream, clearStreamingText, addTier3Request, addSystemMessage, failStream, failPendingToolCall } from '../stores/messages';
import { addSubAgent, updateSubAgentProgress, completeSubAgent, failSubAgent, cancelSubAgent } from '../stores/subagents';
import { addLogEntry } from '../stores/console';
import type { WSEvent } from './types';

let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let activeStreamSessionId: string | null = null;

export function connect() {
  if (socket?.readyState === WebSocket.OPEN) return;

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${proto}//${location.host}/api/ws`;

  reconnecting.set(true);
  socket = new WebSocket(url);

  socket.onopen = () => {
    connected.set(true);
    reconnecting.set(false);
  };

  socket.onclose = () => {
    connected.set(false);
    socket = null;
    scheduleReconnect();
  };

  socket.onerror = () => {
    connected.set(false);
  };

  socket.onmessage = (event) => {
    try {
      const data: WSEvent = JSON.parse(event.data);
      handleEvent(data);
    } catch {
      // Ignore malformed messages.
    }
  };
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnecting.set(true);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connect();
  }, 2000);
}

function handleEvent(event: WSEvent) {
  if (event.type === 'log_entry' && (event as any).entry) {
    addLogEntry((event as any).entry);
    return;
  }

  // Sub-agent events are global (not session-filtered).
  if (event.type === 'sub_agent_spawned' && event.sub_agent_spawned) {
    addSubAgent(event.sub_agent_spawned);
    return;
  }
  if (event.type === 'sub_agent_progress' && event.sub_agent_progress) {
    updateSubAgentProgress(event.sub_agent_progress);
    return;
  }
  if (event.type === 'sub_agent_completed' && event.sub_agent_completed) {
    completeSubAgent(event.sub_agent_completed);
    return;
  }
  if (event.type === 'sub_agent_failed' && event.sub_agent_failed) {
    failSubAgent(event.sub_agent_failed);
    return;
  }
  if (event.type === 'sub_agent_cancelled' && event.sub_agent_cancelled) {
    cancelSubAgent(event.sub_agent_cancelled);
    return;
  }

  const currentSid = get(currentSessionId);
  if (event.session_id && currentSid && event.session_id !== currentSid) {
    if (event.type === 'response_complete') {
      activeStreamSessionId = null;
    }
    return;
  }

  switch (event.type) {
    case 'llm_token':
      if (event.text) {
        appendToken(event.text.text);
      }
      break;

    case 'action_started':
      if (event.action_started) {
        setStreaming(true);
        addToolCallWithFlush(event.action_started.tool_name, event.action_started.summary);
      }
      break;

    case 'shield_verdict':
      if (event.shield_verdict) {
        updateToolCallVerdict(event.shield_verdict);
      }
      break;

    case 'action_completed':
      if (event.action_completed) {
        completeToolCall(event.action_completed);
      }
      break;

    case 'response_complete':
      if (event.response_complete) {
        finalizeResponse(event.response_complete.content, event.response_complete.thoughts);
        setStreaming(false);
        activeStreamSessionId = null;
        if (document.hidden) {
          document.title = '\u25CF OpenParallax';
        }
      }
      break;

    case 'tier3_approval_required':
      if ((event as any).action_id) {
        addTier3Request({
          actionId: (event as any).action_id,
          toolName: (event as any).tool_name || '',
          target: (event as any).target || '',
          reasoning: (event as any).reasoning || '',
          timeoutSecs: (event as any).timeout_secs || 300,
        });
      }
      break;

    case 'error': {
      // Terminal pipeline error (LLM provider down, agent crash, transport
      // failure). Surface the engine's message so the user knows what
      // happened, preserve any partial assistant output, and unblock input.
      // The same error is also persisted server-side as a system message
      // for non-OTR sessions, so a refresh shows the same line.
      const err = (event as any).error || {};
      const msg = err.message || 'unknown error';
      const recoverable = err.recoverable !== false;
      failStream(msg, recoverable);
      activeStreamSessionId = null;
      break;
    }

    case 'otr_blocked': {
      // Per-tool block. The engine emits action_started + otr_blocked but
      // no action_completed, so the orphaned tool card needs to be marked
      // failed here. The session is NOT terminal — the agent receives the
      // tool error and may keep going, so we leave streaming state alone
      // and let the eventual response_complete finalize.
      const reason = event.otr_blocked?.reason || 'OTR mode does not allow this action';
      failPendingToolCall('Blocked by OTR: ' + reason);
      break;
    }

    case 'session_created':
    case 'command_result':
      if ((event as any).text) {
        addSystemMessage((event as any).text);
      }
      break;

    case 'pong':
      break;
  }
}

export function sendMessage(sessionId: string, content: string, mode: string = 'normal') {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  activeStreamSessionId = sessionId;
  socket.send(JSON.stringify({
    type: 'message',
    session_id: sessionId,
    content,
    mode,
  }));
  startNewStream();
}

export function sendPing() {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(JSON.stringify({ type: 'ping' }));
}

export function sendCommand(sessionId: string, content: string) {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(JSON.stringify({ type: 'command', session_id: sessionId, content }));
}

export function sendCancel(sessionId: string) {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(JSON.stringify({ type: 'cancel', session_id: sessionId }));
  activeStreamSessionId = null;
}

export function sendTier3Decision(actionId: string, decision: 'approve' | 'deny') {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(JSON.stringify({
    type: 'tier3_decision',
    action_id: actionId,
    decision,
  }));
}
