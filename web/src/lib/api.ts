import type { Session, Message, StatusResponse } from './types';

const BASE = '/api';

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || resp.statusText);
  }
  if (resp.status === 204) return undefined as T;
  return resp.json();
}

export async function getStatus(): Promise<StatusResponse> {
  return fetchJSON('/status');
}

export async function listSessions(): Promise<Session[]> {
  return fetchJSON('/sessions');
}

export async function createSession(mode: string = 'normal'): Promise<Session> {
  return fetchJSON('/sessions', {
    method: 'POST',
    body: JSON.stringify({ mode }),
  });
}

export async function getSession(id: string): Promise<Session> {
  return fetchJSON(`/sessions/${id}`);
}

export async function deleteSession(id: string): Promise<void> {
  return fetchJSON(`/sessions/${id}`, { method: 'DELETE' });
}

export async function renameSession(id: string, title: string): Promise<void> {
  return fetchJSON(`/sessions/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ title }),
  });
}

export async function getMessages(sessionId: string): Promise<Message[]> {
  return fetchJSON(`/sessions/${sessionId}/messages`);
}

export async function searchMemory(query: string, limit: number = 10) {
  return fetchJSON(`/memory/search?q=${encodeURIComponent(query)}&limit=${limit}`);
}

export async function readMemory(fileType: string) {
  return fetchJSON<{ type: string; content: string }>(`/memory/${fileType}`);
}

export async function getLogs(lines = 200, level = '', event = '', offset = 0): Promise<{ entries: any[]; total_lines: number; has_more: boolean }> {
  const params = new URLSearchParams();
  params.set('lines', String(lines));
  if (level) params.set('level', level);
  if (event) params.set('event', event);
  if (offset > 0) params.set('offset', String(offset));
  return fetchJSON(`/logs?${params}`);
}

export async function searchSessions(query: string): Promise<{ results: { session_id: string; title: string; match_type: string; snippet?: string }[] }> {
  return fetchJSON(`/sessions/search?q=${encodeURIComponent(query)}`);
}

export async function getAudit(lines = 100, offset = 0): Promise<{ entries: any[]; total_entries: number; chain_valid: boolean; has_more: boolean; chain_break_at?: number }> {
  const params = new URLSearchParams();
  params.set('lines', String(lines));
  if (offset > 0) params.set('offset', String(offset));
  return fetchJSON(`/audit?${params}`);
}

export async function getSettings(): Promise<Record<string, any>> {
  return fetchJSON('/settings');
}

export async function testMCPServer(config: { name: string; command: string; args?: string[]; env?: Record<string, string> }): Promise<{ success: boolean; tools?: string[]; error?: string }> {
  return fetchJSON('/settings/test-mcp', { method: 'POST', body: JSON.stringify(config) });
}

export async function getMetrics(period: string = 'daily'): Promise<any> {
  return fetchJSON(`/metrics?period=${period}`);
}

export async function getDailyTokens(days: number = 30): Promise<any[]> {
  return fetchJSON(`/metrics/daily?days=${days}`);
}

export async function getSessionMetrics(sessionId: string): Promise<any> {
  return fetchJSON(`/metrics/session/${sessionId}`);
}
