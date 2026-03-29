<script lang="ts">
  import { X } from 'lucide-svelte';
  import { activeNavItem } from '../stores/settings';
  import { artifactTabs, activeTabId, activeTab, closeArtifactTab } from '../stores/artifacts';
  import { renderMarkdown } from '../lib/format';
  import ArtifactBrowser from './ArtifactBrowser.svelte';
  import MemoryDashboard from './MemoryDashboard.svelte';
  import ConsoleViewer from './ConsoleViewer.svelte';

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
          <button
            class="canvas-tab"
            class:active={$activeTabId === tab.id}
            on:click={() => activeTabId.set(tab.id)}
          >
            <span class="tab-icon">{iconForType(tab.artifact.type)}</span>
            <span class="tab-name">{tab.artifact.title}</span>
            <button class="tab-close" on:click|stopPropagation={() => closeArtifactTab(tab.id)}>
              <X size={11} />
            </button>
          </button>
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
          {:else if $activeTab.artifact.preview_type === 'image'}
            <img
              class="artifact-image"
              src={$activeTab.artifact.path}
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
            <div class="quick-action">
              <span class="quick-action-icon">&#x1F310;</span>
              <div class="quick-action-title">Create a website</div>
              <div class="quick-action-desc">Build a stunning static site</div>
            </div>
            <div class="quick-action">
              <span class="quick-action-icon">&#x1F50D;</span>
              <div class="quick-action-title">Analyze code</div>
              <div class="quick-action-desc">Review and improve your project</div>
            </div>
            <div class="quick-action">
              <span class="quick-action-icon">&#x1F4DA;</span>
              <div class="quick-action-title">Research a topic</div>
              <div class="quick-action-desc">Deep dive into any subject</div>
            </div>
            <div class="quick-action">
              <span class="quick-action-icon">&#x1F4DD;</span>
              <div class="quick-action-title">Write a document</div>
              <div class="quick-action-desc">Reports, articles, documentation</div>
            </div>
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
    border: none; background: none;
    border-bottom: 2px solid transparent;
    transition: all 150ms ease;
    white-space: nowrap;
    display: flex; align-items: center; gap: 6px;
    font-family: inherit;
  }

  .canvas-tab:hover { color: var(--text-secondary); }
  .canvas-tab.active { color: var(--accent); border-bottom-color: var(--accent); }

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
    overflow: auto;
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
    align-items: flex-start;
    justify-content: flex-start;
  }
</style>
