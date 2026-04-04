<script lang="ts">
  import { afterUpdate, onMount } from 'svelte';
  import { ChevronDown, X } from 'lucide-svelte';
  import { messages, pendingToolCalls, streaming, streamingText, pendingApprovals, removeTier3Request } from '../stores/messages';
  import { logEntries } from '../stores/console';
  import { artifactPanelOpen, activeTab, artifactPanelView } from '../stores/artifacts';
  import { getStatus } from '../lib/api';
  import { renderMarkdown } from '../lib/format';
  import Message from './Message.svelte';
  import ToolCallEnvelope from './ToolCallEnvelope.svelte';
  import Tier3Approval from './Tier3Approval.svelte';
  import InputArea from './InputArea.svelte';

  let agentName = 'Atlas';
  let agentAvatar = 'A';

  onMount(async () => {
    try {
      const status = await getStatus();
      if (status.agent_name) agentName = status.agent_name;
      if (status.agent_avatar) {
        agentAvatar = status.agent_avatar;
      } else if (status.agent_name) {
        agentAvatar = status.agent_name.charAt(0).toUpperCase();
      }
    } catch { /* engine not ready */ }
  });

  let messagesEl: HTMLDivElement;
  let prevMessageCount = 0;
  let showScrollBtn = false;

  let panelWidth = parseInt(localStorage.getItem('op_panel_w') || '460');
  let resizingPanel = false;

  let wrapperEl: HTMLDivElement;

  function startPanelResize(e: MouseEvent) {
    e.preventDefault();
    resizingPanel = true;
    const startX = e.clientX;
    const startW = panelWidth;
    const totalW = wrapperEl ? wrapperEl.clientWidth : window.innerWidth;
    const minSide = Math.round(totalW * 0.25);
    const maxPanel = totalW - minSide;

    function onMove(ev: MouseEvent) {
      panelWidth = Math.max(minSide, Math.min(maxPanel, startW - (ev.clientX - startX)));
    }

    function onUp() {
      resizingPanel = false;
      localStorage.setItem('op_panel_w', String(panelWidth));
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    }

    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  function closeRender() {
    artifactPanelOpen.set(false);
  }

  afterUpdate(() => {
    if (!messagesEl) return;
    const count = $messages.length;
    const delta = count - prevMessageCount;
    if (delta > 2) {
      messagesEl.scrollTop = messagesEl.scrollHeight;
    } else if (delta > 0 || $streaming) {
      messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
    }
    prevMessageCount = count;
  });

  function handleScroll() {
    if (!messagesEl) return;
    const distFromBottom = messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight;
    showScrollBtn = distFromBottom > 200;
  }

  function scrollToBottom() {
    if (!messagesEl) return;
    messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && $artifactPanelOpen) {
      artifactPanelOpen.set(false);
    }
  }

  $: messageCount = $messages.length;

  $: totalTokens = $logEntries
    .filter(e => e.event && e.event.includes('llm'))
    .reduce((sum, e) => {
      const d = e.data || {};
      return sum + (Number(d.input_tokens) || 0) + (Number(d.output_tokens) || 0);
    }, 0);

  function formatTokens(n: number): string {
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return String(n);
  }

  $: renderArtifact = $artifactPanelOpen && $activeTab ? $activeTab.artifact : null;
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="chat-wrapper glass" class:streaming={$streaming} class:resizing={resizingPanel} bind:this={wrapperEl}>
  <div class="chat-side">
    <div class="chat-header">
      <span>CHAT</span>
      <span class="msg-count">{messageCount} messages{#if totalTokens > 0} &middot; {formatTokens(totalTokens)} tokens{/if}</span>
    </div>

    <div class="messages-container">
      <div class="messages" class:compact={$artifactPanelOpen} bind:this={messagesEl} on:scroll={handleScroll}>
        {#if $messages.length === 0 && !$streaming}
          <div class="empty-state">
            <div class="empty-orb"></div>
            <span class="empty-text">Ready when you are.</span>
          </div>
        {/if}
        {#each $messages as msg (msg.id)}
          <div class="msg-group">
            {#if msg.role === 'assistant' && msg.thoughts && msg.thoughts.length > 0}
              <ToolCallEnvelope thoughts={msg.thoughts} />
            {/if}
            <Message message={msg} {agentName} {agentAvatar} />
          </div>
        {/each}

        {#if $pendingToolCalls.length > 0}
          <ToolCallEnvelope toolCalls={$pendingToolCalls} live={true} />
        {/if}

        {#each $pendingApprovals as approval (approval.actionId)}
          <Tier3Approval
            actionId={approval.actionId}
            toolName={approval.toolName}
            target={approval.target}
            reasoning={approval.reasoning}
            timeoutSecs={approval.timeoutSecs}
          />
        {/each}

        {#if $streaming && $streamingText}
          <Message message={{ id: 'streaming', session_id: '', role: 'assistant', content: $streamingText, timestamp: new Date().toISOString() }} isStreaming={true} {agentName} {agentAvatar} />
        {/if}

        {#if $streaming && !$streamingText && $pendingToolCalls.length === 0}
          <div class="thinking">
            <div class="thinking-dot"></div>
            <div class="thinking-dot"></div>
            <div class="thinking-dot"></div>
          </div>
        {/if}
      </div>

      {#if showScrollBtn}
        <button class="scroll-to-bottom" on:click={scrollToBottom} aria-label="Scroll to bottom">
          <ChevronDown size={16} />
        </button>
      {/if}
    </div>

    <div class="input-wrap" class:compact={$artifactPanelOpen}>
      <InputArea />
    </div>
  </div>

  {#if renderArtifact}
    <div class="render-divider" on:mousedown={startPanelResize} role="separator" aria-label="Resize"></div>
    <div class="render-side" style="--pw:{panelWidth}px">
      <button class="render-close" on:click={closeRender} aria-label="Close">
        <X size={14} />
      </button>
      {#if renderArtifact.preview_type === 'html'}
        <iframe
          class="render-frame"
          sandbox="allow-scripts allow-same-origin"
          srcdoc={renderArtifact.content}
          title={renderArtifact.title}
        ></iframe>
      {:else if renderArtifact.preview_type === 'markdown'}
        <div class="render-markdown markdown-content">
          {@html renderMarkdown(renderArtifact.content)}
        </div>
      {:else if renderArtifact.language === 'svg'}
        <div class="render-svg">
          {@html renderArtifact.content}
        </div>
      {:else if renderArtifact.preview_type === 'image'}
        <img class="render-image" src={`/api/artifacts/${encodeURIComponent(renderArtifact.path)}`} alt={renderArtifact.title} />
      {:else}
        <pre class="render-code"><code>{renderArtifact.content}</code></pre>
      {/if}
    </div>
  {/if}
</div>

<style>
  .chat-wrapper {
    width: 100%;
    height: 100%;
    display: flex;
    overflow: hidden;
    transition: border-color 300ms ease, box-shadow 300ms ease;
  }

  .chat-wrapper.streaming {
    animation: breathe 2.5s ease-in-out infinite;
  }

  .chat-wrapper.resizing {
    cursor: col-resize;
    user-select: none;
  }

  .chat-wrapper.resizing .render-side {
    pointer-events: none;
  }

  .chat-side {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .chat-header {
    padding: 12px 16px;
    border-bottom: 1px solid var(--accent-border);
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: var(--text-tertiary);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-shrink: 0;
  }

  .msg-count {
    color: var(--accent-dim);
  }

  .messages-container {
    flex: 1;
    position: relative;
    overflow: hidden;
  }

  .messages {
    position: absolute;
    inset: 0;
    overflow-y: auto;
    padding: 24px clamp(20px, 4vw, 60px);
    display: flex; flex-direction: column;
    gap: 14px;
    transition: padding 300ms ease;
  }

  .messages.compact {
    padding: 24px 20px;
  }

  .input-wrap {
    transition: padding 300ms ease;
    padding: 0 clamp(6px, 3vw, 46px);
  }

  .input-wrap.compact {
    padding: 0 6px;
  }

  /* --- Render split --- */

  .render-divider {
    width: 6px;
    cursor: col-resize;
    flex-shrink: 0;
    position: relative;
    transition: background 200ms ease;
  }

  .render-divider::after {
    content: '';
    position: absolute;
    top: 0; bottom: 0;
    left: 50%;
    width: 1px;
    background: var(--accent-border);
    transition: background 200ms ease;
  }

  .render-divider:hover::after {
    background: var(--accent-border-active);
  }

  .render-divider:hover {
    background: var(--accent-ghost);
  }

  .render-side {
    width: var(--pw, 460px);
    min-width: 25%;
    flex-shrink: 0;
    position: relative;
    overflow: hidden;
    border-left: 1px solid var(--accent-border);
    animation: render-fade 200ms ease-out;
  }

  @keyframes render-fade {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .render-close {
    position: absolute;
    top: 8px; right: 8px;
    z-index: 5;
    width: 24px; height: 24px;
    border-radius: 4px;
    border: 1px solid var(--accent-border);
    background: rgba(12, 16, 28, 0.85);
    backdrop-filter: blur(8px);
    color: var(--text-tertiary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 150ms ease;
  }

  .render-close:hover {
    color: var(--text-primary);
    border-color: var(--accent-border-active);
    background: var(--bg-surface-hover);
  }

  .render-frame {
    width: 100%;
    height: 100%;
    border: none;
    background: #0d0d14;
  }

  .render-markdown {
    padding: 20px;
    overflow: auto;
    height: 100%;
  }

  .render-svg {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%; height: 100%;
  }
  .render-svg :global(svg) {
    max-width: 100%;
    max-height: 100%;
  }

  .render-image {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    padding: 16px;
  }

  .render-code {
    margin: 0;
    padding: 16px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-primary);
    overflow: auto;
    height: 100%;
    white-space: pre;
    tab-size: 2;
  }

  .msg-group {
    display: flex;
    flex-direction: column;
  }

  /* --- Empty state --- */

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 24px;
    animation: fade-in 600ms ease-out;
  }

  .empty-orb {
    width: 320px;
    height: 320px;
    border-radius: 50%;
    background: radial-gradient(circle at 40% 40%, var(--accent-subtle), var(--accent-ghost));
    box-shadow:
      0 0 40px rgba(0, 220, 255, 0.15),
      0 0 80px rgba(0, 220, 255, 0.08),
      inset 0 0 20px rgba(0, 220, 255, 0.1);
    animation: orb-pulse 3s ease-in-out infinite;
  }

  @keyframes orb-pulse {
    0%, 100% {
      box-shadow:
        0 0 40px rgba(0, 220, 255, 0.15),
        0 0 80px rgba(0, 220, 255, 0.08),
        inset 0 0 20px rgba(0, 220, 255, 0.1);
      transform: scale(1);
    }
    50% {
      box-shadow:
        0 0 60px rgba(0, 220, 255, 0.25),
        0 0 120px rgba(0, 220, 255, 0.12),
        inset 0 0 30px rgba(0, 220, 255, 0.15);
      transform: scale(1.05);
    }
  }

  .empty-text {
    font-family: 'Exo 2', sans-serif;
    font-size: 15px;
    font-weight: 400;
    color: var(--text-secondary);
    letter-spacing: 0.02em;
  }

  @keyframes fade-in {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }

  /* --- Scroll & thinking --- */

  .scroll-to-bottom {
    position: absolute;
    bottom: 12px;
    left: 50%;
    transform: translateX(-50%);
    width: 32px; height: 32px;
    border-radius: 50%;
    border: 1px solid var(--accent-border-active);
    background: rgba(12, 16, 28, 0.92);
    backdrop-filter: blur(12px);
    color: var(--accent);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: var(--accent-glow);
    transition: all 150ms ease;
    z-index: 5;
  }

  .scroll-to-bottom:hover {
    background: var(--accent-subtle);
    transform: translateX(-50%) translateY(-1px);
    box-shadow: var(--accent-glow-strong);
  }

  .thinking {
    display: flex; gap: 4px;
    padding: 6px 0;
  }

  .thinking-dot {
    width: 5px; height: 5px;
    border-radius: 50%;
    background: var(--accent-dim);
    animation: stream-pulse 1.4s ease-in-out infinite;
  }
  .thinking-dot:nth-child(2) { animation-delay: 0.2s; }
  .thinking-dot:nth-child(3) { animation-delay: 0.4s; }

  /* --- Responsive --- */

  @media (max-width: 1200px) {
    .render-side { width: 40% !important; min-width: 25% !important; }
  }

  @media (max-width: 800px) {
    .messages { padding: 16px 16px; }
    .messages.compact { padding: 16px 12px; }
    .input-wrap { padding: 0 12px; }
    .input-wrap.compact { padding: 0 8px; }
    .render-side {
      position: fixed;
      inset: 0;
      width: 100% !important;
      min-width: 0 !important;
      z-index: 40;
      background: var(--bg-base);
      border-left: none;
    }
    .render-divider { display: none; }
  }
</style>
