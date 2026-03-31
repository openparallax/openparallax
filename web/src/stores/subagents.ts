import { writable, derived } from 'svelte/store';

export interface SubAgentState {
  name: string;
  task: string;
  status: 'spawning' | 'working' | 'completed' | 'failed' | 'cancelled' | 'timed_out';
  toolGroups: string[];
  llmCalls: number;
  toolCalls: number;
  result?: string;
  error?: string;
  startedAt: string;
  completedAt?: string;
  elapsedMs?: number;
}

export const subAgents = writable<Map<string, SubAgentState>>(new Map());

export const hasActiveSubAgents = derived(subAgents, ($subAgents) => {
  for (const agent of $subAgents.values()) {
    if (agent.status === 'spawning' || agent.status === 'working') {
      return true;
    }
  }
  return false;
});

export const subAgentCount = derived(subAgents, ($subAgents) => $subAgents.size);

export function addSubAgent(event: {
  name: string;
  task: string;
  tool_groups?: string[];
}) {
  subAgents.update((agents) => {
    const updated = new Map(agents);
    updated.set(event.name, {
      name: event.name,
      task: event.task,
      status: 'spawning',
      toolGroups: event.tool_groups || [],
      llmCalls: 0,
      toolCalls: 0,
      startedAt: new Date().toISOString(),
    });
    return updated;
  });
}

export function updateSubAgentProgress(event: {
  name: string;
  llm_calls: number;
  tool_calls: number;
  elapsed_ms: number;
}) {
  subAgents.update((agents) => {
    const updated = new Map(agents);
    const agent = updated.get(event.name);
    if (agent) {
      updated.set(event.name, {
        ...agent,
        status: 'working',
        llmCalls: event.llm_calls,
        toolCalls: event.tool_calls,
        elapsedMs: event.elapsed_ms,
      });
    }
    return updated;
  });
}

export function completeSubAgent(event: {
  name: string;
  result: string;
  duration_ms: number;
}) {
  subAgents.update((agents) => {
    const updated = new Map(agents);
    const agent = updated.get(event.name);
    if (agent) {
      updated.set(event.name, {
        ...agent,
        status: 'completed',
        result: event.result,
        elapsedMs: event.duration_ms,
        completedAt: new Date().toISOString(),
      });
    }
    return updated;
  });
}

export function failSubAgent(event: { name: string; error: string }) {
  subAgents.update((agents) => {
    const updated = new Map(agents);
    const agent = updated.get(event.name);
    if (agent) {
      updated.set(event.name, {
        ...agent,
        status: 'failed',
        error: event.error,
        completedAt: new Date().toISOString(),
      });
    }
    return updated;
  });
}

export function cancelSubAgent(event: { name: string }) {
  subAgents.update((agents) => {
    const updated = new Map(agents);
    const agent = updated.get(event.name);
    if (agent) {
      updated.set(event.name, {
        ...agent,
        status: 'cancelled',
        completedAt: new Date().toISOString(),
      });
    }
    return updated;
  });
}

export function dismissSubAgent(name: string) {
  subAgents.update((agents) => {
    const updated = new Map(agents);
    updated.delete(name);
    return updated;
  });
}
