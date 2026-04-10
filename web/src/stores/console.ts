import { writable, get } from 'svelte/store';

export interface LogEntry {
  timestamp: string;
  level: string;
  event: string;
  data?: Record<string, any>;
}

const MAX_ENTRIES = 2000;

export const logEntries = writable<LogEntry[]>([]);
export const consoleLive = writable(true);

export function addLogEntry(entry: LogEntry) {
  logEntries.update(entries => {
    const next = [...entries, entry];
    if (next.length > MAX_ENTRIES) {
      return next.slice(next.length - MAX_ENTRIES);
    }
    return next;
  });
}

export function setLogEntries(entries: LogEntry[]) {
  logEntries.set(entries);
}

export function clearLogEntries() {
  logEntries.set([]);
}
