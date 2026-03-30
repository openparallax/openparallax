<script lang="ts">
  import { onMount } from 'svelte';
  import { X } from 'lucide-svelte';
  import { activeNavItem } from '../stores/settings';
  import { artifactTabs, activeTabId, activeTab, closeArtifactTab, clearArtifactTabs, togglePinTab } from '../stores/artifacts';
  import { sessions, currentSessionId, currentMode } from '../stores/session';
  import { addUserMessage, clearMessages, loadMessages } from '../stores/messages';
  import { sendMessage } from '../lib/websocket';
  import { connected } from '../stores/connection';
  import { createSession, listSessions, getMessages } from '../lib/api';
  import { renderMarkdown } from '../lib/format';
  import ArtifactBrowser from './ArtifactBrowser.svelte';
  import MemoryDashboard from './MemoryDashboard.svelte';
  import ConsoleViewer from './ConsoleViewer.svelte';
  import type { Session } from '../lib/types';

  interface QuickAction {
    icon: string;
    title: string;
    desc: string;
    prompt: string;
    sessionId?: string;
  }

  const defaultActions: QuickAction[] = [
    { icon: '\uD83C\uDF10', title: 'Create a website', desc: 'Build a stunning static site', prompt: 'Create a website for me' },
    { icon: '\uD83D\uDD0D', title: 'Analyze code', desc: 'Review and improve your project', prompt: 'Analyze the code in my workspace' },
    { icon: '\uD83D\uDCDA', title: 'Research a topic', desc: 'Deep dive into any subject', prompt: 'Research a topic for me' },
    { icon: '\uD83D\uDCDD', title: 'Write a document', desc: 'Reports, articles, documentation', prompt: 'Write a document for me' },
  ];

  let quickActions: QuickAction[] = defaultActions;

  onMount(async () => {
    try {
      const sessionList = await listSessions();
      if (sessionList && sessionList.length > 0) {
        const recent = sessionList.slice(0, 4);
        const personalized: QuickAction[] = recent.map((s: Session) => {
          const title = s.title || 'Untitled session';
          return {
            icon: '\uD83D\uDD04',
            title: `Continue: ${title.slice(0, 30)}`,
            desc: 'Pick up where you left off',
            prompt: '',
            sessionId: s.id,
          };
        });
        if (personalized.length >= 2) {
          quickActions = personalized;
        }
      }
    } catch {
      quickActions = defaultActions;
    }
  });

  async function handleQuickAction(action: QuickAction) {
    if (!$connected) return;

    if (action.sessionId) {
      const prevOTR = $currentMode === 'otr' ? $currentSessionId : null;
      const target = $sessions.find(s => s.id === action.sessionId);
      const targetMode = target?.mode === 'otr' ? 'otr' : 'normal';

      currentSessionId.set(action.sessionId);
      currentMode.set(targetMode);
      clearMessages();
      clearArtifactTabs();

      if (prevOTR && prevOTR !== action.sessionId) {
        sessions.update(s => s.filter(sess => sess.id !== prevOTR));
      }

      try {
        const msgs = await getMessages(action.sessionId);
        if (msgs && msgs.length > 0) {
          loadMessages(msgs);
        }
      } catch {
        // Session may have no messages.
      }
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

    addUserMessage(action.prompt);
    sendMessage(sid, action.prompt, $currentMode);
  }

  function iconForType(type: string): string {
    switch (type) {
      case 'file': return '\uD83D\uDCC4';
      case 'command_output': return '\uD83D\uDCBB';
      default: return '\uD83D\uDCC1';
    }
  }

  function renderArtifactContent(content: string): string {
    return renderMarkdown(content);
  }

  $: showCanvas = $activeNavItem === 'chat';
</script>

<div class="canvas glass">
  {#if showCanvas}
    {#if $artifactTabs.length > 0}
      <div class="canvas-tabs">
        {#each $artifactTabs as tab (tab.id)}
          <div
            class="canvas-tab"
            class:active={$activeTabId === tab.id}
            class:pinned={tab.pinned}
            role="tab"
            tabindex="0"
            on:click={() => activeTabId.set(tab.id)}
            on:keydown={(e) => e.key === 'Enter' && activeTabId.set(tab.id)}
            on:contextmenu|preventDefault={() => togglePinTab(tab.id)}
          >
            {#if tab.pinned}
              <span class="tab-pin" title="Pinned">&#x1F4CC;</span>
            {:else}
              <span class="tab-icon">{iconForType(tab.artifact.type)}</span>
            {/if}
            <span class="tab-name">{tab.artifact.title}</span>
            {#if !tab.pinned}
              <button class="tab-close" on:click|stopPropagation={() => closeArtifactTab(tab.id)}>
                <X size={11} />
              </button>
            {/if}
          </div>
        {/each}
      </div>

      <div class="canvas-content">
        {#if $activeTab}
          {#if $activeTab.artifact.preview_type === 'html'}
            <iframe
              class="artifact-iframe"
              sandbox="allow-scripts"
              scrolling="no"
              srcdoc={$activeTab.artifact.content}
              title={$activeTab.artifact.title}
            ></iframe>
          {:else if $activeTab.artifact.preview_type === 'markdown'}
            <div class="artifact-markdown markdown-content">
              {@html renderArtifactContent($activeTab.artifact.content)}
            </div>
          {:else if $activeTab.artifact.preview_type === 'image' && $activeTab.artifact.language === 'svg'}
            <div class="artifact-svg">
              {@html $activeTab.artifact.content}
            </div>
          {:else if $activeTab.artifact.preview_type === 'image'}
            <img
              class="artifact-image"
              src={`/api/artifacts/${encodeURIComponent($activeTab.artifact.path)}`}
              alt={$activeTab.artifact.title}
            />
          {:else}
            <pre class="artifact-code"><code>{$activeTab.artifact.content}</code></pre>
          {/if}
        {/if}
      </div>
    {:else}
      <div class="canvas-content">
        <div class="empty-state">
          <div class="empty-agent-name">Atlas</div>
          <div class="empty-tagline">What would you like to create?</div>
          <div class="quick-actions">
            {#each quickActions as action (action.title)}
              <button class="quick-action" on:click={() => handleQuickAction(action)}>
                <span class="quick-action-icon">{action.icon}</span>
                <div class="quick-action-title">{action.title}</div>
                <div class="quick-action-desc">{action.desc}</div>
              </button>
            {/each}
          </div>
        </div>
      </div>
    {/if}
  {:else}
    <div class="canvas-content alt-view">
      {#if $activeNavItem === 'artifacts'}
        <ArtifactBrowser />
      {:else if $activeNavItem === 'memory'}
        <MemoryDashboard />
      {:else if $activeNavItem === 'console'}
        <ConsoleViewer />
      {/if}
    </div>
  {/if}
</div>

<style>
  .canvas {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    overflow: hidden;
  }

  .canvas-tabs {
    display: flex;
    padding: 0 16px;
    border-bottom: 1px solid var(--accent-border);
    gap: 0;
    overflow-x: auto;
  }

  .canvas-tab {
    padding: 10px 16px;
    font-size: 13px; font-weight: 500;
    color: var(--text-tertiary);
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 150ms ease;
    white-space: nowrap;
    display: flex; align-items: center; gap: 6px;
    font-family: inherit;
  }

  .canvas-tab:hover { color: var(--text-secondary); }
  .canvas-tab.active { color: var(--accent); border-bottom-color: var(--accent); }

  .canvas-tab.pinned { border-bottom-color: var(--accent-border); }
  .tab-pin { font-size: 10px; }
  .tab-icon { font-size: 12px; }
  .tab-name { max-width: 120px; overflow: hidden; text-overflow: ellipsis; }

  .tab-close {
    display: flex; align-items: center; justify-content: center;
    width: 16px; height: 16px;
    border: none; background: none;
    color: var(--text-tertiary);
    cursor: pointer;
    border-radius: 3px;
    opacity: 0;
    transition: all 150ms ease;
    padding: 0;
  }
  .canvas-tab:hover .tab-close { opacity: 0.5; }
  .tab-close:hover { opacity: 1; background: var(--accent-ghost); }

  .canvas-content {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 24px;
  }

  .empty-state {
    text-align: center;
    max-width: 480px;
  }

  .empty-agent-name {
    font-family: 'Exo 2', sans-serif;
    font-weight: 800;
    font-size: 42px;
    color: var(--text-primary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    margin-bottom: 8px;
    opacity: 0.8;
  }

  .empty-tagline {
    font-size: 15px;
    color: var(--text-tertiary);
    margin-bottom: 32px;
  }

  .quick-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }

  .quick-action {
    padding: 16px;
    background: var(--bg-inset);
    backdrop-filter: blur(12px);
    border: 1px solid var(--accent-border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: all 200ms ease;
    text-align: left;
    font-family: inherit;
    color: inherit;
  }

  .quick-action:hover {
    border-color: var(--accent-border-active);
    box-shadow: var(--accent-glow);
    transform: translateY(-1px);
  }

  .quick-action-icon { font-size: 18px; margin-bottom: 8px; display: block; }
  .quick-action-title { font-size: 13px; font-weight: 600; color: var(--text-primary); margin-bottom: 3px; }
  .quick-action-desc { font-size: 12px; color: var(--text-tertiary); line-height: 1.4; }

  .artifact-iframe {
    flex: 1;
    border: none;
    border-radius: 4px;
    background: #0d0d14;
    width: 100%;
    height: 100%;
    overflow: hidden;
  }

  .artifact-markdown {
    max-width: 720px;
    width: 100%;
    padding: 24px;
  }

  .artifact-svg {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
  }
  .artifact-svg :global(svg) {
    max-width: 100%;
    max-height: 100%;
  }

  .artifact-image {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }

  .artifact-code {
    width: 100%;
    padding: 24px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-primary);
    background: var(--bg-inset);
    border-radius: var(--radius);
    overflow: auto;
    white-space: pre;
  }

  .alt-view {
    align-items: stretch;
    justify-content: stretch;
    padding: 20px;
  }
</style>
