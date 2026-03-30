<script lang="ts">
  import { Send, Square } from 'lucide-svelte';
  import { currentSessionId, currentMode, sessions } from '../stores/session';
  import { streaming, addUserMessage, addSystemMessage, clearMessages, messages } from '../stores/messages';
  import { connected } from '../stores/connection';
  import { activeNavItem, sidebarOpen } from '../stores/settings';
  import { clearArtifactTabs } from '../stores/artifacts';
  import { sendMessage } from '../lib/websocket';
  import { createSession, getStatus, deleteSession } from '../lib/api';

  let text = '';
  let textarea: HTMLTextAreaElement;

  const HELP_TEXT = `**Available commands:**
- \`/new\` — Start a new session
- \`/otr\` — Start a new Off The Record session
- \`/quit\` — Close current session, start new one
- \`/restart\` — Restart the engine
- \`/clear\` — Clear the chat view (history preserved)
- \`/status\` — Show system health and session stats
- \`/delete\` — Delete the current session
- \`/sessions\` — Focus session list
- \`/help\` — Show this help message`;

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  async function handleSend() {
    const content = text.trim();
    if (!content || !$connected) return;

    if (content.startsWith('/')) {
      text = '';
      if (textarea) textarea.style.height = 'auto';
      await handleSlashCommand(content);
      return;
    }

    let sid = $currentSessionId;
    if (!sid) {
      try {
        const sess = await createSession($currentMode);
        sessions.update(s => [sess, ...s]);
        currentSessionId.set(sess.id);
        sid = sess.id;
      } catch {
        return;
      }
    }

    addUserMessage(content);
    sendMessage(sid, content, $currentMode);
    text = '';

    if (textarea) {
      textarea.style.height = 'auto';
    }
  }

  async function handleSlashCommand(cmd: string) {
    const parts = cmd.split(/\s+/);
    const command = parts[0].toLowerCase();

    switch (command) {
      case '/help':
        addSystemMessage(HELP_TEXT);
        break;

      case '/new':
        await createNewSession('normal');
        break;

      case '/otr':
        await createNewSession('otr');
        break;

      case '/clear':
        messages.set([]);
        addSystemMessage('Chat cleared. History is preserved.');
        break;

      case '/status':
        await showStatus();
        break;

      case '/restart':
        addSystemMessage('Restarting engine...');
        try {
          await fetch('/api/restart', { method: 'POST' });
        } catch {
          /* WS will drop and reconnect */
        }
        break;

      case '/quit':
        clearMessages();
        clearArtifactTabs();
        currentSessionId.set(null);
        currentMode.set('normal');
        await createNewSession('normal');
        break;

      case '/delete':
        await handleDelete();
        break;

      case '/sessions':
        sidebarOpen.set(true);
        break;

      default:
        addSystemMessage(`Unknown command: \`${command}\`\nType \`/help\` for available commands.`);
        break;
    }
  }

  async function showStatus() {
    try {
      const s = await getStatus();
      const shield = s.shield;
      const shieldLine = shield
        ? `Shield: ${shield.active ? 'Active' : 'Down'} · Tier 2: ${shield.tier2_used}/${shield.tier2_budget} calls today`
        : 'Shield: Unknown';
      const msgCount = $messages.length;

      addSystemMessage(`**System Status**
- **Session:** ${msgCount} messages
- **Model:** ${s.model || 'Not configured'}
- **${shieldLine}**
- **Workspace:** ${s.workspace || 'Unknown'}`);
    } catch {
      addSystemMessage('Failed to fetch system status. Engine may be unreachable.');
    }
  }

  async function handleDelete() {
    const sid = $currentSessionId;
    if (!sid) {
      addSystemMessage('No active session to delete.');
      return;
    }
    addSystemMessage('Delete this session and all its messages? This cannot be undone.\nType `/delete confirm` to proceed.');
    pendingDelete = sid;
  }

  let pendingDelete: string | null = null;

  $: if (text.trim() === '/delete confirm' && pendingDelete) {
    (async () => {
      const sid = pendingDelete;
      pendingDelete = null;
      text = '';
      try {
        await deleteSession(sid);
        sessions.update(s => s.filter(sess => sess.id !== sid));
        clearMessages();
        clearArtifactTabs();
        currentSessionId.set(null);
        currentMode.set('normal');
        addSystemMessage('Session deleted.');
        await createNewSession('normal');
      } catch {
        addSystemMessage('Failed to delete session.');
      }
    })();
  }

  async function createNewSession(mode: 'normal' | 'otr') {
    try {
      if ($currentMode === 'otr' && $currentSessionId) {
        const otrId = $currentSessionId;
        sessions.update(s => s.filter(sess => sess.id !== otrId));
      }
      const sess = await createSession(mode);
      sessions.update(s => [sess, ...s]);
      currentSessionId.set(sess.id);
      currentMode.set(mode);
      clearMessages();
      clearArtifactTabs();
      activeNavItem.set('chat');
    } catch {
      /* ignore */
    }
  }

  function autoResize() {
    if (!textarea) return;
    textarea.style.height = 'auto';
    textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px';
  }

  $: isOTR = $currentMode === 'otr';
</script>

<div class="input-area">
  <div class="input-container">
    <textarea
      bind:this={textarea}
      bind:value={text}
      on:keydown={handleKeydown}
      on:input={autoResize}
      class="input-field"
      class:otr={isOTR}
      placeholder={isOTR ? 'Off the record...' : 'Type a message...'}
      rows="1"
      disabled={!$connected}
    ></textarea>
    <button class="send-btn" on:click={handleSend} disabled={!$connected || (!text.trim() && !$streaming)}>
      {#if $streaming}
        <Square size={16} />
      {:else}
        <Send size={16} />
      {/if}
    </button>
  </div>
  <div class="input-footer">
    <span>Shift+Enter for multiline</span>
    <span class="encrypted-badge">&#x1F512; ENCRYPTED</span>
  </div>
</div>

<style>
  .input-area {
    padding: 12px 14px;
    border-top: 1px solid var(--accent-border);
  }

  .input-container {
    display: flex; gap: 8px;
    align-items: flex-end;
  }

  .input-field {
    flex: 1;
    background: rgba(12, 16, 28, 0.5);
    backdrop-filter: blur(12px);
    border: 1px solid var(--accent-border);
    border-radius: 6px;
    padding: 10px 14px;
    color: var(--text-primary);
    font-family: 'Exo 2', sans-serif;
    font-size: 14px; line-height: 1.5;
    resize: none; outline: none;
    min-height: 40px; max-height: 100px;
    transition: border-color 200ms ease, box-shadow 200ms ease;
  }

  .input-field::placeholder { color: var(--text-tertiary); }

  .input-field:focus {
    border-color: var(--accent-border-active);
    box-shadow: var(--accent-glow);
  }

  .input-field.otr {
    border-color: var(--warning-dim);
  }
  .input-field.otr:focus {
    border-color: var(--warning);
    box-shadow: 0 0 20px rgba(255, 171, 0, 0.12);
  }

  .send-btn {
    width: 40px; height: 40px;
    border-radius: 6px;
    border: none;
    background: linear-gradient(135deg, var(--accent), rgba(0, 180, 220, 1));
    color: #06060c;
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 200ms ease;
    box-shadow: 0 0 15px rgba(0, 220, 255, 0.2);
    flex-shrink: 0;
  }

  .send-btn:hover:not(:disabled) {
    box-shadow: var(--accent-glow-strong);
    transform: translateY(-1px);
  }

  .send-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .input-footer {
    display: flex; justify-content: space-between;
    align-items: center;
    padding: 6px 4px 0;
    font-size: 10px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }

  .encrypted-badge {
    display: flex; align-items: center; gap: 4px;
  }
</style>
