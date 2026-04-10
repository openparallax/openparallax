import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { sessions, currentSessionId, currentMode, currentSession } from '../stores/session';

describe('session store', () => {
  it('sessions starts empty', () => {
    expect(get(sessions)).toEqual([]);
  });

  it('currentSessionId defaults to null', () => {
    expect(get(currentSessionId)).toBeNull();
  });

  it('currentMode defaults to normal', () => {
    expect(get(currentMode)).toBe('normal');
  });

  it('currentMode toggles to otr', () => {
    currentMode.set('otr');
    expect(get(currentMode)).toBe('otr');
    currentMode.set('normal');
  });

  it('currentSession derived store resolves', () => {
    sessions.set([
      { id: 's1', mode: 'normal' as const, created_at: '2026-01-01T00:00:00Z' },
      { id: 's2', mode: 'otr' as const, created_at: '2026-01-02T00:00:00Z' },
    ]);
    currentSessionId.set('s2');

    const sess = get(currentSession);
    expect(sess).not.toBeNull();
    expect(sess!.id).toBe('s2');
    expect(sess!.mode).toBe('otr');

    sessions.set([]);
    currentSessionId.set(null);
  });

  it('currentSession returns null when no match', () => {
    sessions.set([]);
    currentSessionId.set('nonexistent');
    expect(get(currentSession)).toBeNull();
    currentSessionId.set(null);
  });
});
