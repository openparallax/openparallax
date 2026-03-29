<script lang="ts">
  import { ChevronRight, ChevronDown, Wrench, ShieldCheck, ShieldX, Check, X } from 'lucide-svelte';
  import type { ToolCall, Thought } from '../lib/types';

  export let toolCalls: ToolCall[] = [];
  export let thoughts: Thought[] = [];
  export let live = false;

  let expanded = false;

  $: items = thoughts.length > 0 ? thoughts : buildFromToolCalls(toolCalls);
  $: toolCount = items.filter(t => t.stage === 'tool_call').length;
  $: hasBlock = toolCalls.length > 0
    ? toolCalls.some(tc => tc.shieldVerdict?.decision === 'BLOCK')
    : items.some(t => t.detail?.shield === 'BLOCK');
  $: allComplete = !live && (toolCalls.length > 0
    ? toolCalls.every(tc => tc.result)
    : items.filter(t => t.stage === 'tool_call').every(t => t.detail?.success !== undefined));
  $: successCount = toolCalls.length > 0
    ? toolCalls.filter(tc => tc.result?.success).length
    : items.filter(t => t.stage === 'tool_call' && t.detail?.success === true).length;

  let expandedCalls: Record<string, boolean> = {};

  function buildFromToolCalls(tcs: ToolCall[]): Thought[] {
    return tcs.map(tc => ({
      stage: 'tool_call' as const,
      summary: `${tc.toolName} — ${tc.summary}`,
      detail: {
        tool_name: tc.toolName,
        success: tc.result?.success,
        shield: tc.shieldVerdict?.decision,
        result_summary: tc.result?.summary,
      },
    }));
  }

  function getToolCall(thought: Thought): ToolCall | undefined {
    if (thought.stage !== 'tool_call') return undefined;
    const name = thought.detail?.tool_name as string;
    const matched = toolCalls.find(tc => tc.toolName === name);
    if (matched) return matched;

    const shieldDecision = thought.detail?.shield as string | undefined;
    return {
      id: Math.random().toString(),
      toolName: name || '',
      summary: thought.summary,
      expanded: false,
      shieldVerdict: shieldDecision ? {
        toolName: name || '',
        decision: shieldDecision as 'ALLOW' | 'BLOCK' | 'ESCALATE',
        tier: (thought.detail?.shield_tier as number) || 0,
        confidence: 1,
        reasoning: (thought.detail?.shield_reasoning as string) || '',
      } : undefined,
      result: thought.detail?.success !== undefined
        ? { success: thought.detail.success as boolean, summary: (thought.detail.result_summary as string) || thought.summary }
        : undefined,
    };
  }

  function toggleCall(id: string) {
    expandedCalls[id] = !expandedCalls[id];
    expandedCalls = expandedCalls;
  }
</script>

<div class="tool-envelope" class:blocked={hasBlock} class:expanded>
  <button class="tool-summary" on:click={() => expanded = !expanded}>
    <span class="tool-chevron">
      {#if expanded}
        <ChevronDown size={10} />
      {:else}
        <ChevronRight size={10} />
      {/if}
    </span>
    <Wrench size={12} />
    <span>
      <strong>{toolCount} tool {toolCount === 1 ? 'call' : 'calls'}</strong>
      {#if allComplete && toolCount > 0}
        &mdash; {successCount}/{toolCount} succeeded
      {:else if live}
        &mdash; running
      {/if}
    </span>
  </button>

  {#if expanded}
    <div class="tool-details">
      {#each items as item, i (i)}
        {#if item.stage === 'reasoning'}
          <div class="tool-thought">{item.summary}</div>
        {:else}
          {@const tc = getToolCall(item)}
          {#if tc}
            <button class="tool-call-item" class:expanded={expandedCalls[tc.id]} on:click={() => toggleCall(tc.id)}>
              <div class="tool-call-header">
                <Wrench size={12} />
                <span class="tool-call-name">{tc.toolName}</span>
                <span class="tool-call-summary">&mdash; {tc.summary}</span>
              </div>
              {#if tc.shieldVerdict}
                <div class="tool-call-shield">
                  {#if tc.shieldVerdict.decision === 'ALLOW'}
                    <span class="shield-allow">
                      <ShieldCheck size={11} />
                      Shield: ALLOW (Tier {tc.shieldVerdict.tier}) &middot; <Check size={11} />
                    </span>
                  {:else}
                    <span class="shield-block">
                      <ShieldX size={11} />
                      Shield: BLOCK (Tier {tc.shieldVerdict.tier})
                    </span>
                  {/if}
                </div>
              {/if}
              {#if expandedCalls[tc.id]}
                <div class="tool-call-result">
                  {#if tc.shieldVerdict?.decision === 'BLOCK' && tc.shieldVerdict.reasoning}
                    <div class="tool-call-reasoning">{tc.shieldVerdict.reasoning}</div>
                  {/if}
                  {#if tc.result}
                    <div class="tool-call-detail">
                      {#if tc.result.success}
                        <Check size={11} /> {tc.result.summary}
                      {:else}
                        <X size={11} /> {tc.result.summary}
                      {/if}
                    </div>
                  {/if}
                </div>
              {/if}
            </button>
          {/if}
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .tool-envelope {
    max-width: 92%;
    border-radius: 6px;
    background: rgba(0, 220, 255, 0.03);
    border: 1px solid rgba(0, 220, 255, 0.08);
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    margin: 4px 0;
    overflow: hidden;
    transition: border-color 200ms ease;
  }

  .tool-envelope:hover {
    border-color: var(--accent-subtle);
    background: rgba(0, 220, 255, 0.05);
  }

  .tool-envelope.blocked {
    border-left: 2px solid var(--error);
    animation: block-flash 600ms ease-out;
  }

  .tool-summary {
    padding: 8px 12px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    border: none;
    background: none;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    text-align: left;
    transition: background 150ms ease;
  }
  .tool-summary:hover { background: var(--accent-ghost); }
  .tool-summary strong { color: var(--accent-dim); }

  .tool-chevron {
    color: var(--text-tertiary);
    display: flex;
    transition: transform 200ms ease;
  }
  .tool-envelope.expanded .tool-chevron { transform: rotate(90deg); }

  .tool-details {
    padding: 0 12px 10px;
    border-top: 1px solid var(--accent-border);
  }

  .tool-thought {
    font-family: 'Exo 2', sans-serif;
    font-style: italic;
    color: var(--text-tertiary);
    padding: 8px 0 4px;
    font-size: 12px;
  }

  .tool-call-item {
    display: block;
    width: 100%;
    padding: 6px 0;
    border: none;
    background: none;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    text-align: left;
    cursor: pointer;
    border-bottom: 1px solid var(--accent-border);
  }
  .tool-call-item:last-child { border-bottom: none; }

  .tool-call-header {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .tool-call-name { color: var(--accent-dim); font-weight: 500; }
  .tool-call-summary { color: var(--text-tertiary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .tool-call-shield {
    padding-left: 20px;
    margin-top: 2px;
    font-size: 11px;
  }

  .shield-allow {
    color: var(--accent-dim);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .shield-block {
    color: var(--error-dim);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .tool-call-result {
    padding-left: 20px;
    margin-top: 4px;
    font-size: 11px;
  }

  .tool-call-reasoning {
    color: var(--error-dim);
    line-height: 1.5;
    margin-bottom: 4px;
  }

  .tool-call-detail {
    color: var(--text-tertiary);
    display: flex;
    align-items: center;
    gap: 4px;
  }
</style>
