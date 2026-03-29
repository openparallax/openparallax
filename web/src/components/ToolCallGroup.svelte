<script lang="ts">
  import { ChevronRight, ChevronDown, Wrench } from 'lucide-svelte';
  import type { ToolCall as ToolCallType, Thought } from '../lib/types';
  import ToolCall from './ToolCall.svelte';

  export let toolCalls: ToolCallType[] = [];
  export let thoughts: Thought[] = [];
  export let live = false;

  let expanded = false;

  $: items = thoughts.length > 0 ? thoughts : buildFromToolCalls(toolCalls);
  $: toolCount = items.filter(t => t.stage === 'tool_call').length;
  $: hasBlock = toolCalls.some(tc => tc.shieldVerdict?.decision === 'BLOCK');
  $: allComplete = !live && toolCalls.every(tc => tc.result);
  $: successCount = toolCalls.filter(tc => tc.result?.success).length;

  function buildFromToolCalls(tcs: ToolCallType[]): Thought[] {
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

  function toolCallForThought(thought: Thought): ToolCallType | undefined {
    if (thought.stage !== 'tool_call') return undefined;
    const name = thought.detail?.tool_name as string;
    return toolCalls.find(tc => tc.toolName === name) || {
      id: Math.random().toString(),
      toolName: name || '',
      summary: thought.summary,
      expanded: false,
      result: thought.detail?.success !== undefined
        ? { success: thought.detail.success as boolean, summary: thought.detail.result_summary as string || thought.summary }
        : undefined,
    };
  }
</script>

<div class="tool-group" class:blocked={hasBlock} class:live>
  <button class="tool-group-header" on:click={() => expanded = !expanded}>
    <span class="tool-group-chevron">
      {#if expanded}
        <ChevronDown size={13} />
      {:else}
        <ChevronRight size={13} />
      {/if}
    </span>
    <Wrench size={13} />
    <span class="tool-group-summary">
      {toolCount} tool {toolCount === 1 ? 'call' : 'calls'}
      {#if allComplete && toolCount > 0}
        <span class="tool-group-result">
          &mdash; {successCount}/{toolCount} succeeded
        </span>
      {:else if live}
        <span class="tool-group-live">&mdash; running</span>
      {/if}
    </span>
  </button>

  {#if expanded}
    <div class="tool-group-body">
      {#each items as item, i (i)}
        {#if item.stage === 'reasoning'}
          <div class="reasoning-block">{item.summary}</div>
        {:else}
          {@const tc = toolCallForThought(item)}
          {#if tc}
            <ToolCall toolCall={tc} />
          {/if}
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .tool-group {
    max-width: 85%;
    border-radius: var(--radius);
    background: rgba(0, 220, 255, 0.02);
    border: 1px solid rgba(0, 220, 255, 0.05);
    margin: 2px 0;
    transition: border-color 200ms ease;
  }

  .tool-group:hover {
    border-color: rgba(0, 220, 255, 0.12);
  }

  .tool-group.blocked {
    border-left: 2px solid var(--error);
  }

  .tool-group.live {
    border-color: rgba(0, 220, 255, 0.15);
  }

  .tool-group-header {
    display: flex; align-items: center;
    gap: 6px;
    padding: 8px 12px;
    width: 100%;
    border: none; background: none;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
    text-align: left;
  }

  .tool-group-header:hover { color: var(--text-primary); }

  .tool-group-chevron { color: var(--cyan-dim); display: flex; }

  .tool-group-summary { color: var(--cyan-dim); font-weight: 500; }

  .tool-group-result { color: var(--text-tertiary); font-weight: 400; }

  .tool-group-live { color: var(--cyan-dim); font-weight: 400; }

  .tool-group-body {
    padding: 0 8px 8px;
    display: flex; flex-direction: column;
    gap: 4px;
  }

  .reasoning-block {
    padding: 6px 12px;
    font-size: 12px;
    line-height: 1.5;
    color: var(--text-tertiary);
    font-style: italic;
    border-left: 2px solid rgba(0, 220, 255, 0.08);
    margin: 2px 0;
  }

  .tool-group-body :global(.tool-call) {
    max-width: 100%;
    margin: 0;
    border: none;
    border-radius: 4px;
    background: rgba(0, 220, 255, 0.01);
  }

  .tool-group-body :global(.tool-call:hover) {
    background: rgba(0, 220, 255, 0.03);
  }
</style>
