import { writable } from 'svelte/store';

export const showSettings = writable(false);
export const activeDetailTab = writable<'artifacts' | 'shield' | 'memory'>('artifacts');
export const activeNavItem = writable<'chat' | 'artifacts' | 'memory' | 'console'>('chat');
