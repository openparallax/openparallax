<script lang="ts">
  import { get } from 'svelte/store';
  import { onMount } from 'svelte';
  import { Send, Square, ShieldCheck, AlertTriangle } from 'lucide-svelte';
  import { currentSessionId, currentMode, sessions } from '../stores/session';
  import { streaming, addUserMessage, addSystemMessage, clearMessages, messages } from '../stores/messages';
  import { connected } from '../stores/connection';
  import { activeNavItem, sidebarOpen } from '../stores/settings';
  import { clearArtifactTabs } from '../stores/artifacts';
  import { sendMessage } from '../lib/websocket';
  import { createSession, getStatus, deleteSession } from '../lib/api';
  import type { SandboxStatusData } from '../lib/types';

  let text = '';
  let textarea: HTMLTextAreaElement;

  const COMMANDS = [
    { cmd: '/help', desc: 'Show available commands' },
    { cmd: '/new', desc: 'Start a new session' },
    { cmd: '/otr', desc: 'Start a new OTR session' },
    { cmd: '/quit', desc: 'Close session, start new one' },
    { cmd: '/restart', desc: 'Restart the engine' },
    { cmd: '/clear', desc: 'Clear chat view' },
    { cmd: '/status', desc: 'Show system health' },
    { cmd: '/export', desc: 'Export session as markdown' },
    { cmd: '/delete', desc: 'Delete current session' },
    { cmd: '/sessions', desc: 'Focus session list' },
  ];

  let showAutocomplete = false;
  let autocompleteItems: typeof COMMANDS = [];
  let selectedIndex = 0;

  $: {
    if (text.startsWith('/') && !text.includes(' ')) {
      const prefix = text.toLowerCase();
      autocompleteItems = COMMANDS.filter(c => c.cmd.startsWith(prefix));
      showAutocomplete = autocompleteItems.length > 0 && text !== autocompleteItems[0]?.cmd;
      selectedIndex = 0;
    } else {
      showAutocomplete = false;
      autocompleteItems = [];
    }
  }

  function selectAutocomplete(idx: number) {
    text = autocompleteItems[idx].cmd;
    showAutocomplete = false;
    textarea?.focus();
  }

  const HELP_TEXT = `**Available commands:**
- \`/new\` — Start a new session
- \`/otr\` — Start a new Off The Record session
- \`/quit\` — Close current session, start new one
- \`/restart\` — Restart the engine
- \`/clear\` — Clear the chat view (history preserved)
- \`/status\` — Show system health and session stats
- \`/export\` — Export session as markdown file
- \`/delete\` — Delete the current session
- \`/sessions\` — Focus session list
- \`/help\` — Show this help message`;

  function handleKeydown(e: KeyboardEvent) {
    if (showAutocomplete) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedIndex = (selectedIndex + 1) % autocompleteItems.length;
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedIndex = (selectedIndex - 1 + autocompleteItems.length) % autocompleteItems.length;
        return;
      }
      if (e.key === 'Tab') {
        e.preventDefault();
        selectAutocomplete(selectedIndex);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        showAutocomplete = false;
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        selectAutocomplete(selectedIndex);
        return;
      }
    }
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

      case '/export':
        exportSession();
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
      const sb = s.sandbox;
      const sandboxLine = sb?.active
        ? `Sandbox: ${sb.mode}${sb.version ? ' v' + sb.version : ''} · Filesystem: ${sb.filesystem ? 'restricted' : 'open'} · Network: ${sb.network ? 'restricted' : 'open'}`
        : `Sandbox: Inactive${sb?.reason ? ' (' + sb.reason + ')' : ''}`;

      addSystemMessage(`**System Status**
- **Session:** ${msgCount} messages
- **Model:** ${s.model || 'Not configured'}
- **${shieldLine}**
- **${sandboxLine}**
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

  function exportSession() {
    const msgs = get(messages);
    if (msgs.length === 0) {
      addSystemMessage('No messages to export.');
      return;
    }

    const now = new Date();
    const date = now.toISOString().slice(0, 10);
    const time = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    let md = `# Session Export\n*Exported on ${date} at ${time}*\n\n---\n\n`;

    for (const msg of msgs) {
      if (msg.role === 'system') continue;
      const who = msg.role === 'user' ? '**You**' : '**Atlas**';
      const ts = new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      md += `${who} (${ts}):\n${msg.content}\n`;

      if (msg.thoughts && msg.thoughts.length > 0) {
        const toolCalls = msg.thoughts.filter(t => t.stage === 'tool_call');
        if (toolCalls.length > 0) {
          md += `\n> ${toolCalls.length} tool calls\n`;
          for (const tc of toolCalls) {
            const d = tc.detail || {};
            const status = d.success ? '\u2713' : '\u2717';
            md += `> - ${d.tool_name || tc.summary} ${status}\n`;
          }
        }
      }
      md += '\n---\n\n';
    }

    const blob = new Blob([md], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `session-export-${date}.md`;
    a.click();
    URL.revokeObjectURL(url);

    addSystemMessage(`Session exported as \`session-export-${date}.md\``);
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

  let sandboxStatus: SandboxStatusData | null = null;

  onMount(async () => {
    try {
      const s = await getStatus();
      if (s.sandbox) sandboxStatus = s.sandbox;
    } catch {
      /* engine not ready */
    }
  });
</script>

<div class="input-area">
  {#if showAutocomplete}
    <div class="autocomplete-dropdown">
      {#each autocompleteItems as item, i (item.cmd)}
        <button
          class="autocomplete-item"
          class:selected={i === selectedIndex}
          on:mousedown|preventDefault={() => selectAutocomplete(i)}
        >
          <span class="ac-cmd">{item.cmd}</span>
          <span class="ac-desc">{item.desc}</span>
        </button>
      {/each}
    </div>
  {/if}
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
    {#if sandboxStatus?.active}
      <span class="sandbox-badge sandboxed" title="{sandboxStatus.mode}{sandboxStatus.version ? ' v' + sandboxStatus.version : ''} — FS: {sandboxStatus.filesystem ? 'restricted' : 'open'}, Net: {sandboxStatus.network ? 'restricted' : 'open'}">
        <ShieldCheck size={11} />
        {sandboxStatus.mode}{sandboxStatus.version ? ' v' + sandboxStatus.version : ''}{#if sandboxStatus.filesystem} · FS{/if}{#if sandboxStatus.network} · Net{/if}
      </span>
    {:else}
      <span class="sandbox-badge unsandboxed" title={sandboxStatus?.reason || 'Sandbox unavailable'}>
        <AlertTriangle size={11} />
        {sandboxStatus?.reason || 'Unsandboxed'}
      </span>
    {/if}
  </div>
</div>

<style>
  .input-area {
    padding: 12px 14px;
    border-top: 1px solid var(--accent-border);
    position: relative;
  }

  .autocomplete-dropdown {
    position: absolute;
    bottom: 100%;
    left: 14px; right: 14px;
    margin-bottom: 4px;
    background: rgba(12, 16, 28, 0.95);
    backdrop-filter: blur(16px);
    border: 1px solid var(--accent-border-active);
    border-radius: var(--radius);
    padding: 4px;
    z-index: 20;
    max-height: 200px;
    overflow-y: auto;
  }

  .autocomplete-item {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 13px;
    font-family: 'JetBrains Mono', monospace;
    cursor: pointer;
    border-radius: 4px;
    transition: background 100ms ease;
    text-align: left;
  }
  .autocomplete-item:hover,
  .autocomplete-item.selected {
    background: var(--accent-ghost);
  }

  .ac-cmd { color: var(--accent); font-weight: 500; }
  .ac-desc { color: var(--text-tertiary); font-size: 11px; }

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

  .sandbox-badge {
    display: flex; align-items: center; gap: 4px;
  }
  .sandbox-badge.sandboxed {
    color: var(--success);
  }
  .sandbox-badge.unsandboxed {
    color: var(--text-tertiary);
  }
</style>
