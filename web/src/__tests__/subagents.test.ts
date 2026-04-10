import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  subAgents,
  hasActiveSubAgents,
  subAgentCount,
  addSubAgent,
  updateSubAgentProgress,
  completeSubAgent,
  failSubAgent,
  cancelSubAgent,
  dismissSubAgent,
} from '../stores/subagents';

describe('subagents store', () => {
  beforeEach(() => {
    subAgents.set(new Map());
  });

  it('starts empty', () => {
    expect(get(subAgents).size).toBe(0);
    expect(get(hasActiveSubAgents)).toBe(false);
    expect(get(subAgentCount)).toBe(0);
  });

  it('adds a sub-agent', () => {
    addSubAgent({ name: 'phoenix', task: 'Research pricing' });
    const agents = get(subAgents);
    expect(agents.size).toBe(1);
    expect(agents.get('phoenix')?.task).toBe('Research pricing');
    expect(agents.get('phoenix')?.status).toBe('spawning');
    expect(get(hasActiveSubAgents)).toBe(true);
    expect(get(subAgentCount)).toBe(1);
  });

  it('adds a sub-agent with tool groups', () => {
    addSubAgent({ name: 'cortex', task: 'Refactor code', tool_groups: ['files', 'shell'] });
    const agent = get(subAgents).get('cortex');
    expect(agent?.toolGroups).toEqual(['files', 'shell']);
  });

  it('updates progress', () => {
    addSubAgent({ name: 'phoenix', task: 'Research' });
    updateSubAgentProgress({ name: 'phoenix', llm_calls: 3, tool_calls: 7, elapsed_ms: 12000 });
    const agent = get(subAgents).get('phoenix');
    expect(agent?.status).toBe('working');
    expect(agent?.llmCalls).toBe(3);
    expect(agent?.toolCalls).toBe(7);
    expect(agent?.elapsedMs).toBe(12000);
  });

  it('completes a sub-agent', () => {
    addSubAgent({ name: 'phoenix', task: 'Research' });
    completeSubAgent({ name: 'phoenix', result: 'Found 5 competitors', duration_ms: 45000 });
    const agent = get(subAgents).get('phoenix');
    expect(agent?.status).toBe('completed');
    expect(agent?.result).toBe('Found 5 competitors');
    expect(agent?.elapsedMs).toBe(45000);
    expect(agent?.completedAt).toBeDefined();
    expect(get(hasActiveSubAgents)).toBe(false);
  });

  it('fails a sub-agent', () => {
    addSubAgent({ name: 'phoenix', task: 'Research' });
    failSubAgent({ name: 'phoenix', error: 'timeout after 300s' });
    const agent = get(subAgents).get('phoenix');
    expect(agent?.status).toBe('failed');
    expect(agent?.error).toBe('timeout after 300s');
    expect(get(hasActiveSubAgents)).toBe(false);
  });

  it('cancels a sub-agent', () => {
    addSubAgent({ name: 'phoenix', task: 'Research' });
    cancelSubAgent({ name: 'phoenix' });
    const agent = get(subAgents).get('phoenix');
    expect(agent?.status).toBe('cancelled');
    expect(get(hasActiveSubAgents)).toBe(false);
  });

  it('dismisses a sub-agent', () => {
    addSubAgent({ name: 'phoenix', task: 'Research' });
    completeSubAgent({ name: 'phoenix', result: 'done', duration_ms: 1000 });
    dismissSubAgent('phoenix');
    expect(get(subAgents).size).toBe(0);
    expect(get(subAgentCount)).toBe(0);
  });

  it('tracks multiple agents independently', () => {
    addSubAgent({ name: 'phoenix', task: 'Research' });
    addSubAgent({ name: 'cortex', task: 'Refactor' });
    expect(get(subAgentCount)).toBe(2);
    expect(get(hasActiveSubAgents)).toBe(true);

    completeSubAgent({ name: 'phoenix', result: 'done', duration_ms: 1000 });
    expect(get(hasActiveSubAgents)).toBe(true); // cortex still active

    completeSubAgent({ name: 'cortex', result: 'done', duration_ms: 2000 });
    expect(get(hasActiveSubAgents)).toBe(false);
  });

  it('ignores progress for unknown agent', () => {
    updateSubAgentProgress({ name: 'unknown', llm_calls: 1, tool_calls: 0, elapsed_ms: 100 });
    expect(get(subAgents).size).toBe(0);
  });

  it('ignores complete for unknown agent', () => {
    completeSubAgent({ name: 'unknown', result: 'x', duration_ms: 100 });
    expect(get(subAgents).size).toBe(0);
  });
});
