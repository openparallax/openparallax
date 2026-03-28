<script lang="ts">
  import { shieldLog } from '../stores/messages';
</script>

<div class="shield-log">
  {#if $shieldLog.length === 0}
    <div class="empty-state">No Shield activity yet</div>
  {:else}
    {#each $shieldLog as entry, i (i)}
      <div class="shield-log-entry">
        <div class="shield-log-badge" class:allow={entry.decision === 'ALLOW'} class:block={entry.decision === 'BLOCK'}>
          {entry.decision}
        </div>
        <div>
          <div class="shield-log-text">{entry.toolName}</div>
          <div class="shield-log-time">Tier {entry.tier} &middot; {entry.reasoning.slice(0, 60)}</div>
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .empty-state {
    color: var(--text-tertiary);
    font-size: 13px;
    text-align: center;
    padding: 40px 0;
  }

  .shield-log-entry {
    display: flex; align-items: flex-start;
    gap: 10px;
    padding: 10px 0;
    border-bottom: 1px solid rgba(0, 220, 255, 0.04);
    font-size: 12px;
  }

  .shield-log-badge {
    padding: 2px 8px;
    border-radius: 3px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px; font-weight: 600;
    letter-spacing: 0.03em;
    flex-shrink: 0;
  }

  .shield-log-badge.allow { background: rgba(0, 220, 255, 0.1); color: var(--cyan); }
  .shield-log-badge.block { background: rgba(255, 61, 90, 0.1); color: var(--error); }

  .shield-log-text { color: var(--text-secondary); line-height: 1.5; }
  .shield-log-time { color: var(--text-tertiary); font-size: 11px; margin-top: 2px; }
</style>
