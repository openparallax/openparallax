import { writable } from 'svelte/store';

export const settingsOpen = writable(false);
export const sidebarOpen = writable(false);
export const activeNavItem = writable<'chat' | 'console'>('chat');
