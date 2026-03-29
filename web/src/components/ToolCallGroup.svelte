<script lang="ts">
  import { ChevronRight, ChevronDown, Wrench } from 'lucide-svelte';
  import type { ToolCall as ToolCallType } from '../lib/types';
  import ToolCall from './ToolCall.svelte';

  export let toolCalls: ToolCallType[];

  let expanded = false;

  $: hasBlock = toolCalls.some(tc => tc.shieldVerdict?.decision === 'BLOCK');
  $: allComplete = toolCalls.every(tc => tc.result);
  $: successCount = toolCalls.filter(tc => tc.result?.success).length;
</script>

<div class="tool-group" class:blocked={hasBlock}>
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
      {toolCalls.length} tool {toolCalls.length === 1 ? 'call' : 'calls'}
      {#if allComplete}
        <span class="tool-group-result">
          &mdash; {successCount}/{toolCalls.length} succeeded
        </span>
      {/if}
    </span>
  </button>

  {#if expanded}
    <div class="tool-group-body">
      {#each toolCalls as tc (tc.id)}
        <ToolCall toolCall={tc} />
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

  .tool-group-body {
    padding: 0 8px 8px;
    display: flex; flex-direction: column;
    gap: 2px;
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
