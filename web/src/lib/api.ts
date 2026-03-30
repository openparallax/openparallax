import type { Session, Message, StatusResponse, Artifact } from './types';

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

export async function listArtifacts(): Promise<Artifact[]> {
  return fetchJSON('/artifacts');
}

export async function getArtifactContent(path: string): Promise<{ content: string }> {
  return fetchJSON(`/artifacts/${encodeURIComponent(path)}`);
}

export async function getLogs(lines = 200, level = '', event = ''): Promise<{ entries: any[]; total_lines: number; has_more: boolean }> {
  const params = new URLSearchParams();
  params.set('lines', String(lines));
  if (level) params.set('level', level);
  if (event) params.set('event', event);
  return fetchJSON(`/logs?${params}`);
}

export async function getAudit(lines = 100): Promise<{ entries: any[]; total_entries: number; chain_valid: boolean; has_more: boolean; chain_break_at?: number }> {
  return fetchJSON(`/audit?lines=${lines}`);
}
