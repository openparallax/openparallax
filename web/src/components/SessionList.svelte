<script lang="ts">
  import { Trash2 } from 'lucide-svelte';
  import { sessions, currentSessionId, currentMode } from '../stores/session';
  import { clearMessages, loadMessages } from '../stores/messages';
  import { activeNavItem, sidebarOpen } from '../stores/settings';
  import { getMessages, deleteSession } from '../lib/api';
  import { formatRelativeTime } from '../lib/format';
  import type { Session } from '../lib/types';

  async function switchSession(id: string) {
    if (id === $currentSessionId) return;

    const prevOTR = $currentMode === 'otr' ? $currentSessionId : null;

    const target = $sessions.find(s => s.id === id);
    const targetMode = target?.mode === 'otr' ? 'otr' : 'normal';

    currentSessionId.set(id);
    currentMode.set(targetMode);
    activeNavItem.set('chat');
    sidebarOpen.set(false);
    clearMessages();

    if (prevOTR && prevOTR !== id) {
      sessions.update(s => s.filter(sess => sess.id !== prevOTR));
    }

    try {
      const msgs = await getMessages(id);
      if (msgs && msgs.length > 0) {
        loadMessages(msgs);
      }
    } catch {
      // Session may have no messages.
    }
  }

  let confirmingDeleteId: string | null = null;
  let confirmTimer: ReturnType<typeof setTimeout>;

  function requestDelete(e: Event, id: string) {
    e.stopPropagation();
    clearTimeout(confirmTimer);
    confirmingDeleteId = id;
    confirmTimer = setTimeout(() => { confirmingDeleteId = null; }, 5000);
  }

  async function confirmDelete(e: Event, id: string) {
    e.stopPropagation();
    clearTimeout(confirmTimer);
    confirmingDeleteId = null;
    try {
      await deleteSession(id);
      sessions.update(s => s.filter(sess => sess.id !== id));
      if ($currentSessionId === id) {
        currentSessionId.set(null);
        clearMessages();
      }
    } catch {
      // Ignore delete errors.
    }
  }

  function cancelDelete(e: Event) {
    e.stopPropagation();
    clearTimeout(confirmTimer);
    confirmingDeleteId = null;
  }

  function sessionLabel(session: Session): string {
    if (session.title) return session.title;
    if (session.preview) {
      const text = session.preview.slice(0, 40);
      return text.length < session.preview.length ? text + '...' : text;
    }
    return 'New Session';
  }

  function sessionTime(session: Session): string {
    return formatRelativeTime(session.last_message_at || session.created_at);
  }
</script>

<div class="session-list">
  {#each $sessions as session (session.id)}
    <div
      class="session-item"
      class:active={$currentSessionId === session.id}
      role="button"
      tabindex="0"
      on:click={() => switchSession(session.id)}
      on:keydown={(e) => e.key === 'Enter' && switchSession(session.id)}
    >
      <div class="session-info">
        <div class="session-name" class:otr={session.mode === 'otr'}>
          {#if session.mode === 'otr'}OTR: {/if}{sessionLabel(session)}
        </div>
        <div class="session-meta">
          {sessionTime(session)}
        </div>
      </div>
      <div class="session-actions">
        {#if confirmingDeleteId === session.id}
          <div class="delete-confirm">
            <span>Delete?</span>
            <button class="confirm-yes" on:click={(e) => confirmDelete(e, session.id)}>Yes</button>
            <button class="confirm-no" on:click={cancelDelete}>No</button>
          </div>
        {:else}
          <button class="delete-btn" on:click={(e) => requestDelete(e, session.id)} title="Delete session">
            <Trash2 size={12} />
          </button>
        {/if}
        <div class="session-dot" class:active={$currentSessionId === session.id} class:otr={session.mode === 'otr'}></div>
      </div>
    </div>
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
  }

  .session-item:hover { background: var(--bg-surface-hover); }
  .session-item.active {
    background: var(--accent-ghost);
    border-color: var(--accent-border);
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

  .session-actions {
    display: flex; align-items: center; gap: 6px;
    flex-shrink: 0;
  }

  .delete-btn {
    width: 22px; height: 22px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--text-tertiary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    opacity: 0;
    transition: all 150ms ease;
  }

  .session-item:hover .delete-btn { opacity: 1; }
  .delete-btn:hover {
    color: var(--error);
    background: rgba(255, 61, 90, 0.1);
  }

  .session-dot {
    width: 7px; height: 7px;
    border-radius: 50%;
    background: var(--text-tertiary);
    flex-shrink: 0;
  }

  .session-dot.active {
    background: var(--success);
    box-shadow: 0 0 6px rgba(0, 230, 118, 0.4);
  }

  .session-dot.otr {
    background: var(--warning);
    box-shadow: 0 0 6px rgba(255, 171, 0, 0.4);
  }

  .delete-confirm {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    font-family: 'JetBrains Mono', monospace;
    color: var(--error);
    animation: fade-in 150ms ease-out;
  }

  .confirm-yes, .confirm-no {
    padding: 1px 6px;
    border-radius: 3px;
    border: none;
    font-size: 10px;
    font-family: 'JetBrains Mono', monospace;
    cursor: pointer;
    transition: all 100ms ease;
  }

  .confirm-yes {
    background: var(--error);
    color: var(--bg-void);
  }
  .confirm-yes:hover {
    box-shadow: 0 0 8px rgba(255, 61, 90, 0.4);
  }

  .confirm-no {
    background: var(--accent-ghost);
    color: var(--text-secondary);
  }
  .confirm-no:hover {
    background: var(--bg-surface-hover);
  }

  @keyframes fade-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>
