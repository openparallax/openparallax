<script lang="ts">
  import { connected, reconnecting } from '../stores/connection';
  import { onMount } from 'svelte';
  import { getStatus } from '../lib/api';

  let model = '';

  onMount(async () => {
    try {
      const s = await getStatus();
      model = s.model;
    } catch {
      model = 'unknown';
    }
  });
</script>

<div class="status-bar">
  <span class="status-model">{model}</span>
  <span class="status-connection" class:online={$connected} class:reconnecting={$reconnecting}>
    {#if $connected}
      connected
    {:else if $reconnecting}
      reconnecting...
    {:else}
      disconnected
    {/if}
  </span>
</div>

<style>
  .status-bar {
    position: fixed;
    bottom: 0; left: 0; right: 0;
    padding: 4px 16px;
    display: flex; justify-content: space-between;
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: var(--text-tertiary);
    background: rgba(6, 6, 12, 0.8);
    z-index: 10;
  }

  .status-connection.online { color: var(--success-dim); }
  .status-connection.reconnecting { color: var(--warning-dim); }
</style>
