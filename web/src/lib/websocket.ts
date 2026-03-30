import { get } from 'svelte/store';
import { connected, reconnecting } from '../stores/connection';
import { currentSessionId } from '../stores/session';
import { appendToken, addToolCall, updateToolCallVerdict, completeToolCall, addArtifact, finalizeResponse, setStreaming, startNewStream, clearStreamingText } from '../stores/messages';
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
        clearStreamingText();
        addToolCall({
          id: event.message_id + '-' + Date.now(),
          toolName: event.action_started.tool_name,
          summary: event.action_started.summary,
          expanded: false,
        });
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

    case 'action_artifact':
      if (event.action_artifact) {
        addArtifact(event.action_artifact.artifact);
      }
      break;

    case 'response_complete':
      if (event.response_complete) {
        finalizeResponse(event.response_complete.content, event.response_complete.thoughts);
        setStreaming(false);
        activeStreamSessionId = null;
      }
      break;

    case 'otr_blocked':
    case 'error':
      setStreaming(false);
      activeStreamSessionId = null;
      break;

    case 'session_created':
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
