import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { connected, reconnecting } from '../stores/connection';

describe('connection store', () => {
  it('connected defaults to false', () => {
    expect(get(connected)).toBe(false);
  });

  it('reconnecting defaults to false', () => {
    expect(get(reconnecting)).toBe(false);
  });

  it('connected can be set to true', () => {
    connected.set(true);
    expect(get(connected)).toBe(true);
    connected.set(false);
  });

  it('reconnecting can be set to true', () => {
    reconnecting.set(true);
    expect(get(reconnecting)).toBe(true);
    reconnecting.set(false);
  });
});
