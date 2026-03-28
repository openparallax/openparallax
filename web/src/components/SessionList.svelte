<script lang="ts">
  import { sessions, currentSessionId } from '../stores/session';
  import { messages, clearMessages } from '../stores/messages';
  import { getMessages } from '../lib/api';
  import { formatRelativeTime } from '../lib/format';

  async function switchSession(id: string) {
    currentSessionId.set(id);
    clearMessages();
    try {
      const msgs = await getMessages(id);
      messages.set(msgs);
    } catch {
      // Session may not have messages yet.
    }
  }
</script>

<div class="session-list">
  {#each $sessions as session (session.id)}
    <button
      class="session-item"
      class:active={$currentSessionId === session.id}
      on:click={() => switchSession(session.id)}
    >
      <div class="session-info">
        <div class="session-name" class:otr={session.mode === 'otr'}>
          {#if session.mode === 'otr'}OTR: {/if}{session.title || 'New Session'}
        </div>
        <div class="session-meta">
          {#if session.last_msg_at}
            {formatRelativeTime(session.last_msg_at)}
          {:else}
            {formatRelativeTime(session.created_at)}
          {/if}
          {#if session.message_count > 0}
            &middot; {session.message_count} messages
          {/if}
        </div>
      </div>
      <div class="session-dot" class:active={$currentSessionId === session.id} class:otr={session.mode === 'otr'}></div>
    </button>
  {/each}
</div>

<style>
  .session-list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 10px;
  }

  .session-item {
    display: flex; align-items: center;
    justify-content: space-between;
    padding: 10px 12px;
    border-radius: var(--radius);
    cursor: pointer;
    transition: all 150ms ease;
    border: 1px solid transparent;
    margin-bottom: 2px;
    background: none;
    width: 100%;
    text-align: left;
    font-family: inherit;
  }

  .session-item:hover { background: var(--bg-surface-hover); }
  .session-item.active {
    background: var(--cyan-ghost);
    border-color: var(--cyan-border);
  }

  .session-info { min-width: 0; flex: 1; }

  .session-name {
    font-size: 13px; font-weight: 500;
    color: var(--text-primary);
    white-space: nowrap; overflow: hidden;
    text-overflow: ellipsis;
  }

  .session-name.otr { color: var(--warning); }

  .session-meta {
    font-size: 11px;
    color: var(--text-tertiary);
    margin-top: 2px;
  }

  .session-dot {
    width: 7px; height: 7px;
    border-radius: 50%;
    background: var(--text-tertiary);
    flex-shrink: 0;
    margin-left: 8px;
  }

  .session-dot.active {
    background: var(--success);
    box-shadow: 0 0 6px rgba(0, 230, 118, 0.4);
  }

  .session-dot.otr {
    background: var(--warning);
    box-shadow: 0 0 6px rgba(255, 171, 0, 0.4);
  }
</style>
