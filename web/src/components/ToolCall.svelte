<script lang="ts">
  import type { ToolCall as ToolCallType } from '../lib/types';
  import { Wrench, ShieldCheck, ShieldX, Check, X } from 'lucide-svelte';

  export let toolCall: ToolCallType;

  let expanded = false;

  $: blocked = toolCall.shieldVerdict?.decision === 'BLOCK';

  function toggle() {
    expanded = !expanded;
  }
</script>

<button class="tool-call" class:blocked on:click={toggle}>
  <div class="tool-call-header">
    <Wrench size={13} />
    <span class="tool-name">{toolCall.toolName}</span>
    <span class="tool-divider">&mdash;</span>
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

      {#if toolCall.result}
        <span class="tool-result-inline" class:success={toolCall.result.success} class:failure={!toolCall.result.success}>
          {#if toolCall.result.success}
            <Check size={11} /> {toolCall.result.summary}
          {:else}
            <X size={11} /> {toolCall.result.summary}
          {/if}
        </span>
      {/if}
    </div>
  {/if}

  {#if expanded}
    <div class="tool-call-expanded">
      {#if blocked && toolCall.shieldVerdict}
        <div class="tool-call-reasoning">{toolCall.shieldVerdict.reasoning}</div>
      {/if}
      {#if toolCall.result}
        <div class="tool-call-result-detail">
          Result: {toolCall.result.summary}
        </div>
      {/if}
    </div>
  {/if}
</button>

<style>
  .tool-call {
    max-width: 85%;
    padding: 8px 14px;
    margin: 2px 0;
    border-radius: var(--radius);
    background: rgba(0, 220, 255, 0.02);
    border: 1px solid rgba(0, 220, 255, 0.05);
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px; line-height: 1.5;
    color: var(--text-secondary);
    cursor: pointer;
    text-align: left;
    width: auto;
    display: inline-block;
    transition: border-color 200ms ease;
  }

  .tool-call:hover {
    border-color: rgba(0, 220, 255, 0.12);
  }

  .tool-call.blocked {
    border-left: 2px solid var(--error);
    animation: block-flash 600ms ease-out;
  }

  .tool-call-header {
    display: flex; align-items: center;
    gap: 6px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tool-name { color: var(--cyan-dim); font-weight: 500; }
  .tool-divider { color: var(--text-tertiary); }
  .tool-summary { color: var(--text-tertiary); overflow: hidden; text-overflow: ellipsis; }

  .tool-call-detail {
    display: flex; align-items: center; gap: 16px;
    padding-left: 20px;
    margin-top: 3px;
    flex-wrap: wrap;
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

  .tool-result-inline {
    font-size: 11px;
    display: flex; align-items: center; gap: 4px;
  }
  .tool-result-inline.success { color: var(--success-dim); }
  .tool-result-inline.failure { color: var(--error-dim); }

  .tool-call-expanded {
    margin-top: 6px;
    padding-top: 6px;
    border-top: 1px solid rgba(0, 220, 255, 0.04);
  }

  .tool-call-reasoning {
    color: var(--error-dim);
    font-size: 11px;
    line-height: 1.5;
    padding-left: 20px;
  }

  .tool-call-result-detail {
    color: var(--text-tertiary);
    font-size: 11px;
    padding-left: 20px;
  }
</style>
