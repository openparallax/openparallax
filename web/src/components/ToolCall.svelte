<script lang="ts">
  import type { ToolCall as ToolCallType } from '../lib/types';
  import { Wrench, ShieldCheck, ShieldX, Check, X } from 'lucide-svelte';

  export let toolCall: ToolCallType;

  $: blocked = toolCall.shieldVerdict?.decision === 'BLOCK';
  $: expanded = toolCall.expanded;

  function toggle() {
    toolCall.expanded = !toolCall.expanded;
  }
</script>

<button class="tool-call" class:blocked on:click={toggle}>
  <div class="tool-call-header">
    <Wrench size={13} />
    <span class="tool-name">{toolCall.toolName}</span>
    <span class="tool-summary">{toolCall.summary}</span>
  </div>

  {#if toolCall.shieldVerdict}
    <div class="tool-call-detail">
      {#if toolCall.shieldVerdict.decision === 'ALLOW'}
        <span class="shield-allow">
          <ShieldCheck size={11} />
          Shield: ALLOW (Tier {toolCall.shieldVerdict.tier})
        </span>
      {:else}
        <span class="shield-block">
          <ShieldX size={11} />
          Shield: BLOCK (Tier {toolCall.shieldVerdict.tier})
        </span>
      {/if}
    </div>
  {/if}

  {#if toolCall.result}
    <div class="tool-call-detail">
      {#if toolCall.result.success}
        <span class="tool-result success"><Check size={11} /> {toolCall.result.summary}</span>
      {:else}
        <span class="tool-result failure"><X size={11} /> {toolCall.result.summary}</span>
      {/if}
    </div>
  {/if}

  {#if expanded && blocked && toolCall.shieldVerdict}
    <div class="tool-call-reasoning">
      {toolCall.shieldVerdict.reasoning}
    </div>
  {/if}
</button>

<style>
  .tool-call {
    max-width: 78%;
    padding: 10px 14px;
    margin: 4px 0;
    border-radius: var(--radius);
    background: rgba(0, 220, 255, 0.02);
    border: 1px solid rgba(0, 220, 255, 0.05);
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px; line-height: 1.6;
    color: var(--text-secondary);
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: border-color 200ms ease;
  }

  .tool-call:hover {
    border-color: rgba(0, 220, 255, 0.1);
  }

  .tool-call.blocked {
    border-left: 2px solid var(--error);
    animation: block-flash 600ms ease-out;
  }

  .tool-call-header {
    display: flex; align-items: center;
    gap: 6px;
  }

  .tool-name { color: var(--cyan-dim); font-weight: 500; }
  .tool-summary { color: var(--text-tertiary); }

  .tool-call-detail {
    padding-left: 20px;
    margin-top: 2px;
  }

  .shield-allow {
    color: var(--cyan-dim);
    font-size: 11px;
    display: flex; align-items: center; gap: 4px;
  }

  .shield-block {
    color: var(--error-dim);
    font-size: 11px;
    display: flex; align-items: center; gap: 4px;
  }

  .tool-result {
    font-size: 11px;
    display: flex; align-items: center; gap: 4px;
  }
  .tool-result.success { color: var(--success-dim); }
  .tool-result.failure { color: var(--error-dim); }

  .tool-call-reasoning {
    padding: 8px 0 0 20px;
    color: var(--error-dim);
    font-size: 11px;
    line-height: 1.5;
  }
</style>
