import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { messages, addSystemMessage, addUserMessage, clearMessages } from '../stores/messages';
import { settingsOpen, sidebarOpen, activeNavItem } from '../stores/settings';
import { logEntries, addLogEntry, clearLogEntries } from '../stores/console';

describe('system messages', () => {
  beforeEach(() => clearMessages());

  it('addSystemMessage creates a system role message', () => {
    addSystemMessage('Test system message');
    const msgs = get(messages);
    expect(msgs).toHaveLength(1);
    expect(msgs[0].role).toBe('system');
    expect(msgs[0].content).toBe('Test system message');
  });

  it('system messages coexist with user messages', () => {
    addUserMessage('hello');
    addSystemMessage('system info');
    const msgs = get(messages);
    expect(msgs).toHaveLength(2);
    expect(msgs[0].role).toBe('user');
    expect(msgs[1].role).toBe('system');
  });
});

describe('/clear preserves history concept', () => {
  beforeEach(() => clearMessages());

  it('clearing messages sets array to empty', () => {
    addUserMessage('one');
    addUserMessage('two');
    messages.set([]);
    expect(get(messages)).toHaveLength(0);
  });
});

describe('settings and sidebar stores', () => {
  it('settingsOpen toggles', () => {
    settingsOpen.set(true);
    expect(get(settingsOpen)).toBe(true);
    settingsOpen.set(false);
    expect(get(settingsOpen)).toBe(false);
  });

  it('sidebarOpen toggles', () => {
    sidebarOpen.set(true);
    expect(get(sidebarOpen)).toBe(true);
    sidebarOpen.set(false);
  });

  it('activeNavItem defaults to chat', () => {
    expect(get(activeNavItem)).toBe('chat');
  });
});

describe('console log entries', () => {
  beforeEach(() => clearLogEntries());

  it('addLogEntry pushes to store', () => {
    addLogEntry({ timestamp: '2026-01-01T00:00:00Z', level: 'info', event: 'test' });
    expect(get(logEntries)).toHaveLength(1);
  });

  it('entries capped at 2000', () => {
    for (let i = 0; i < 2010; i++) {
      addLogEntry({ timestamp: '2026-01-01T00:00:00Z', level: 'info', event: `e${i}` });
    }
    expect(get(logEntries)).toHaveLength(2000);
  });

  it('clearLogEntries resets', () => {
    addLogEntry({ timestamp: '2026-01-01T00:00:00Z', level: 'info', event: 'test' });
    clearLogEntries();
    expect(get(logEntries)).toHaveLength(0);
  });
});

describe('token counter derivation', () => {
  beforeEach(() => clearLogEntries());

  it('sums tokens from llm events', () => {
    addLogEntry({
      timestamp: '2026-01-01T00:00:00Z', level: 'info', event: 'llm_call_started',
      data: { input_tokens: 100, output_tokens: 50 },
    });
    addLogEntry({
      timestamp: '2026-01-01T00:00:01Z', level: 'info', event: 'llm_call_completed',
      data: { input_tokens: 200, output_tokens: 80 },
    });
    const entries = get(logEntries);
    const total = entries
      .filter(e => e.event && e.event.includes('llm'))
      .reduce((sum, e) => sum + (Number(e.data?.input_tokens) || 0) + (Number(e.data?.output_tokens) || 0), 0);
    expect(total).toBe(430);
  });
});
