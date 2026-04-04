<script lang="ts">
  import { afterUpdate } from 'svelte';
  import type { Message as MessageType, Artifact } from '../lib/types';
  import { renderMarkdown } from '../lib/format';
  import { openArtifactTab } from '../stores/artifacts';
  import { activeNavItem } from '../stores/settings';

  export let message: MessageType;
  export let isStreaming = false;
  export let agentName = 'Atlas';
  export let agentAvatar = 'A';

  let bubbleEl: HTMLDivElement;

  $: isAtlas = message.role === 'assistant';
  $: isSystem = message.role === 'system';
  $: htmlContent = renderMarkdown(message.content);
  $: timestamp = new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  $: messageArtifacts = message.artifacts || [];

  let showContextMenu = false;
  let menuX = 0;
  let menuY = 0;

  function handleContextMenu(e: MouseEvent) {
    if (isSystem) return;
    const target = e.target as HTMLElement;
    if (target.tagName === 'A' || target.closest('a')) return;
    e.preventDefault();
    showContextMenu = true;
    menuX = e.clientX;
    menuY = e.clientY;
  }

  function closeMenu() {
    showContextMenu = false;
  }

  function copyText() {
    navigator.clipboard.writeText(message.content);
    closeMenu();
  }

  afterUpdate(() => {
    if (!bubbleEl) return;
    bubbleEl.querySelectorAll('pre').forEach(pre => {
      if (pre.querySelector('.copy-code-btn')) return;
      const btn = document.createElement('button');
      btn.className = 'copy-code-btn';
      btn.textContent = '\u2398';
      btn.title = 'Copy code';
      btn.addEventListener('click', () => {
        const code = pre.querySelector('code');
        const text = code ? code.textContent || '' : pre.textContent || '';
        navigator.clipboard.writeText(text);
        btn.textContent = '\u2713';
        btn.classList.add('copied');
        setTimeout(() => { btn.textContent = '\u2398'; btn.classList.remove('copied'); }, 1500);
      });
      pre.appendChild(btn);
    });
  });

  function viewArtifact(artifact: Artifact) {
    openArtifactTab(artifact);
  }

  function artifactIcon(artifact: Artifact): string {
    switch (artifact.preview_type) {
      case 'html': return '\uD83C\uDF10';
      case 'markdown': return '\uD83D\uDCDD';
      case 'image': return '\uD83D\uDDBC';
      case 'video': return '\uD83C\uDFA5';
      default: return '\uD83D\uDCC4';
    }
  }

  function artifactLang(artifact: Artifact): string {
    return artifact.language || artifact.preview_type || 'file';
  }

  function formatBytes(bytes: number): string {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / 1024 / 1024).toFixed(1) + ' MB';
  }
</script>

{#if isSystem}
  <div class="system-message">
    <div class="system-bubble markdown-content">
      {@html htmlContent}
    </div>
  </div>
{:else}
  <div class="message" class:atlas={isAtlas} class:user={!isAtlas} on:contextmenu={handleContextMenu} role="article">
    <div class="msg-header">
      <div class="msg-avatar" class:atlas-avatar={isAtlas} class:user-avatar={!isAtlas}>
        {isAtlas ? agentAvatar : 'Y'}
      </div>
      <div class="msg-name">{isAtlas ? agentName : 'You'}</div>
      <div class="msg-time">{timestamp}</div>
    </div>
    <div class="msg-bubble markdown-content" bind:this={bubbleEl}>
      {@html htmlContent}
      {#if isStreaming}
        <span class="cursor"></span>
      {/if}
      {#each messageArtifacts as artifact (artifact.id)}
        <button class="artifact-card" on:click={() => viewArtifact(artifact)}>
          <span class="artifact-card-icon">{artifactIcon(artifact)}</span>
          <div class="artifact-card-info">
            <span class="artifact-card-title">{artifact.title}</span>
            <span class="artifact-card-meta">{artifactLang(artifact)}{artifact.size_bytes ? ' \u00B7 ' + formatBytes(artifact.size_bytes) : ''}</span>
          </div>
          <span class="artifact-card-arrow">&rsaquo;</span>
        </button>
      {/each}
    </div>
  </div>

  {#if showContextMenu}
    <button class="context-backdrop" on:click={closeMenu} aria-label="Close menu"></button>
    <div class="context-menu" style="left: {menuX}px; top: {menuY}px;">
      <button class="context-item" on:click={copyText}>Copy text</button>
    </div>
  {/if}
{/if}

<style>
  .message {
    display: flex;
    flex-direction: column;
    animation: msg-in 300ms ease-out;
  }

  .message.atlas { align-items: flex-start; }
  .message.user { align-items: flex-end; }

  .msg-header {
    display: flex; align-items: center;
    gap: 6px;
    margin-bottom: 4px;
    font-size: 12px;
  }

  .message.atlas .msg-header { padding-left: 2px; }
  .message.user .msg-header { padding-right: 2px; flex-direction: row-reverse; }

  .msg-avatar {
    width: 20px; height: 20px;
    border-radius: 5px;
    display: flex; align-items: center; justify-content: center;
    font-size: 10px; font-weight: 700;
    font-family: 'JetBrains Mono', monospace;
  }

  .atlas-avatar {
    background: var(--accent-subtle);
    color: var(--accent);
    border: 1px solid var(--accent-border-active);
  }

  .user-avatar {
    background: rgba(240, 240, 245, 0.08);
    color: var(--text-secondary);
    border: 1px solid rgba(240, 240, 245, 0.1);
  }

  .msg-name { font-weight: 600; }
  .message.atlas .msg-name { color: var(--accent); }
  .message.user .msg-name { color: var(--text-secondary); }

  .msg-time { color: var(--text-tertiary); font-size: 11px; }

  .msg-bubble {
    max-width: 92%;
    padding: 12px 16px;
    border-radius: 6px;
    font-size: 14px;
    line-height: 1.65;
  }

  .message.atlas .msg-bubble {
    background: transparent;
    border: none;
    max-width: 100%;
  }

  .message.user .msg-bubble {
    background: rgba(240, 240, 245, 0.04);
    border: 1px solid rgba(240, 240, 245, 0.06);
    border-right: 2px solid rgba(240, 240, 245, 0.2);
    max-width: 85%;
  }

  .system-message {
    display: flex;
    justify-content: center;
    animation: msg-in 300ms ease-out;
  }

  .system-bubble {
    max-width: 95%;
    padding: 12px 16px;
    border-radius: 6px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--text-secondary);
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
    font-family: 'JetBrains Mono', monospace;
  }

  .cursor {
    display: inline-block;
    width: 2px; height: 14px;
    background: var(--accent);
    margin-left: 1px;
    animation: blink 1s infinite;
    vertical-align: text-bottom;
  }

  .context-backdrop {
    position: fixed;
    inset: 0;
    z-index: 99;
    background: transparent;
    border: none;
    cursor: default;
  }

  .context-menu {
    position: fixed;
    z-index: 100;
    min-width: 140px;
    background: rgba(12, 16, 28, 0.95);
    backdrop-filter: blur(16px);
    border: 1px solid var(--accent-border-active);
    border-radius: var(--radius);
    padding: 4px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
  }

  .context-item {
    width: 100%;
    padding: 6px 12px;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 12px;
    font-family: inherit;
    cursor: pointer;
    text-align: left;
    border-radius: 4px;
    transition: background 100ms ease;
  }
  .context-item:hover { background: var(--accent-ghost); }

  .artifact-card {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    margin-top: 8px;
    padding: 10px 14px;
    background: var(--bg-inset);
    border: 1px solid var(--accent-border);
    border-radius: var(--radius);
    cursor: pointer;
    font-family: inherit;
    color: inherit;
    text-align: left;
    transition: all 200ms ease;
  }
  .artifact-card:hover {
    border-color: var(--accent-border-active);
    background: var(--bg-surface-hover);
    box-shadow: var(--accent-glow);
  }

  .artifact-card-icon {
    font-size: 20px;
    flex-shrink: 0;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-ghost);
    border-radius: 6px;
  }

  .artifact-card-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .artifact-card-title {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .artifact-card-meta {
    font-size: 11px;
    font-family: 'JetBrains Mono', monospace;
    color: var(--accent-dim);
    text-transform: uppercase;
  }

  .artifact-card-arrow {
    font-size: 20px;
    color: var(--accent-dim);
    flex-shrink: 0;
    transition: transform 150ms ease;
  }
  .artifact-card:hover .artifact-card-arrow {
    transform: translateX(3px);
    color: var(--accent);
  }
</style>
