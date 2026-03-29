import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { settingsOpen, activeNavItem } from '../stores/settings';

describe('settings store', () => {
  it('settingsOpen defaults to false', () => {
    expect(get(settingsOpen)).toBe(false);
  });

  it('settingsOpen toggles', () => {
    settingsOpen.set(true);
    expect(get(settingsOpen)).toBe(true);
    settingsOpen.set(false);
    expect(get(settingsOpen)).toBe(false);
  });

  it('activeNavItem defaults to chat', () => {
    expect(get(activeNavItem)).toBe('chat');
  });

  it('activeNavItem changes to artifacts', () => {
    activeNavItem.set('artifacts');
    expect(get(activeNavItem)).toBe('artifacts');
    activeNavItem.set('chat');
  });

  it('activeNavItem changes to memory', () => {
    activeNavItem.set('memory');
    expect(get(activeNavItem)).toBe('memory');
    activeNavItem.set('chat');
  });

  it('activeNavItem changes to console', () => {
    activeNavItem.set('console');
    expect(get(activeNavItem)).toBe('console');
    activeNavItem.set('chat');
  });
});
