<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ShieldAlert, Check, X } from 'lucide-svelte';
  import { sendTier3Decision } from '../lib/websocket';

  export let actionId: string;
  export let toolName: string;
  export let target: string;
  export let reasoning: string;
  export let timeoutSecs: number;

  let remaining = timeoutSecs;
  let timer: ReturnType<typeof setInterval>;
  let decided = false;

  onMount(() => {
    timer = setInterval(() => {
      remaining--;
      if (remaining <= 0) {
        clearInterval(timer);
        if (!decided) {
          decide(false);
        }
      }
    }, 1000);
  });

  onDestroy(() => {
    if (timer) clearInterval(timer);
  });

  function decide(approved: boolean) {
    if (decided) return;
    decided = true;
    clearInterval(timer);
    sendTier3Decision(actionId, approved ? 'approve' : 'deny');
  }

  $: minutes = Math.floor(remaining / 60);
  $: seconds = remaining % 60;
  $: timeStr = `${minutes}:${String(seconds).padStart(2, '0')}`;
</script>

<div class="tier3-approval" class:decided>
  <div class="tier3-header">
    <ShieldAlert size={16} />
    <span>Shield needs your approval</span>
  </div>

  <div class="tier3-action">
    <span class="tier3-tool">{toolName}</span>
    <span class="tier3-target">{target}</span>
  </div>

  {#if reasoning}
    <div class="tier3-reasoning">
      <span class="tier3-why">Why:</span> {reasoning}
    </div>
  {/if}

  {#if !decided}
    <div class="tier3-buttons">
      <button class="tier3-btn approve" on:click={() => decide(true)}>
        <Check size={14} /> Approve
      </button>
      <button class="tier3-btn deny" on:click={() => decide(false)}>
        <X size={14} /> Deny
      </button>
    </div>
    <div class="tier3-timer">Auto-denies in {timeStr}</div>
  {:else}
    <div class="tier3-resolved">Decision submitted</div>
  {/if}
</div>

<style>
  .tier3-approval {
    max-width: 92%;
    border-radius: 8px;
    background: rgba(12, 16, 28, 0.8);
    border: 1px solid var(--warning);
    border-left: 3px solid var(--warning);
    padding: 16px;
    margin: 8px 0;
    font-size: 13px;
    animation: tier3-pulse 2s ease-in-out infinite;
  }

  .tier3-approval.decided {
    animation: none;
    opacity: 0.6;
    border-color: var(--accent-border);
  }

  @keyframes tier3-pulse {
    0%, 100% { box-shadow: 0 0 0 0 rgba(255, 171, 0, 0); }
    50% { box-shadow: 0 0 12px 0 rgba(255, 171, 0, 0.15); }
  }

  .tier3-header {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--warning);
    font-weight: 600;
    font-family: 'Exo 2', sans-serif;
    margin-bottom: 12px;
  }

  .tier3-action {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 10px;
    padding: 10px;
    background: rgba(0, 0, 0, 0.3);
    border-radius: 4px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
  }

  .tier3-tool {
    color: var(--accent);
    font-weight: 600;
  }

  .tier3-target {
    color: var(--text-primary);
    word-break: break-all;
  }

  .tier3-reasoning {
    color: var(--text-secondary);
    font-size: 12px;
    line-height: 1.5;
    margin-bottom: 12px;
  }

  .tier3-why {
    color: var(--text-tertiary);
    font-weight: 600;
  }

  .tier3-buttons {
    display: flex;
    gap: 10px;
    margin-bottom: 8px;
  }

  .tier3-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 10px 16px;
    border-radius: 6px;
    border: none;
    font-family: 'Exo 2', sans-serif;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: all 150ms ease;
  }

  .tier3-btn.approve {
    background: rgba(0, 230, 118, 0.15);
    color: var(--success);
    border: 1px solid rgba(0, 230, 118, 0.3);
  }
  .tier3-btn.approve:hover {
    background: rgba(0, 230, 118, 0.25);
  }

  .tier3-btn.deny {
    background: rgba(255, 61, 90, 0.15);
    color: var(--error);
    border: 1px solid rgba(255, 61, 90, 0.3);
  }
  .tier3-btn.deny:hover {
    background: rgba(255, 61, 90, 0.25);
  }

  .tier3-timer {
    text-align: center;
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }

  .tier3-resolved {
    text-align: center;
    font-size: 12px;
    color: var(--text-tertiary);
  }
</style>
