import { writable, derived } from 'svelte/store';
import type { Session } from '../lib/types';

export const sessions = writable<Session[]>([]);
export const currentSessionId = writable<string | null>(null);
export const currentMode = writable<'normal' | 'otr'>('normal');
export const scrollToMessageId = writable<string | null>(null);

export const currentSession = derived(
  [sessions, currentSessionId],
  ([$sessions, $id]) => $sessions.find(s => s.id === $id) || null
);
