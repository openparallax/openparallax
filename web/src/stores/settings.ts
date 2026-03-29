import { writable } from 'svelte/store';

export const settingsOpen = writable(false);
export const activeNavItem = writable<'chat' | 'artifacts' | 'memory' | 'console'>('chat');
