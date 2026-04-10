import { writable, derived } from 'svelte/store';
import type { Session } from '../lib/types';

export const sessions = writable<Session[]>([]);
export const currentSessionId = writable<string | null>(null);
export const currentMode = writable<'normal' | 'otr'>('normal');
export const scrollToMessageId = writable<string | null>(null);

// suppressAutoScroll is a one-shot flag set by the Sidebar before a
// search-result session switch. ChatPanel checks it on the next
// afterUpdate and skips the pinToBottom auto-scroll, allowing the
// scrollToMessageId handler to land on the target message instead.
export const suppressAutoScroll = writable(false);

export const currentSession = derived(
  [sessions, currentSessionId],
  ([$sessions, $id]) => $sessions.find(s => s.id === $id) || null
);
