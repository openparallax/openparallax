<script lang="ts">
  import { afterUpdate, onMount, tick } from 'svelte';
  import { ChevronDown } from 'lucide-svelte';
  import { messages, pendingSteps, streaming, streamingText, pendingApprovals, removeTier3Request } from '../stores/messages';
  import { scrollToMessageId, currentSessionId, suppressAutoScroll } from '../stores/session';
  import { logEntries } from '../stores/console';
  import { getStatus } from '../lib/api';
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
  /** True when the user is within PIN_TOLERANCE_PX of the bottom. While
   * true, new content auto-scrolls to keep them pinned. When the user
   * scrolls up, this flips false and afterUpdate stops fighting them. */
  let pinToBottom = true;

  /** Distance (px) from the bottom under which we consider the user
   * "at the bottom" and auto-scroll on new content. */
  const PIN_TOLERANCE_PX = 80;

  // Reset scroll state when switching sessions so the new session
  // always opens at the bottom, regardless of where the user was
  // scrolled in the previous session.
  $: if ($currentSessionId) {
    pinToBottom = true;
    prevMessageCount = 0;
  }

  afterUpdate(async () => {
    if (!messagesEl) return;
    const count = $messages.length;
    const delta = count - prevMessageCount;

    // Scroll to a specific message from search results. This check
    // comes first and returns early so the pinToBottom auto-scroll
    // below never fires when there is a pending target. If the
    // element isn't in the DOM yet (messages still loading), keep
    // the target and retry on the next afterUpdate cycle.
    const targetId = $scrollToMessageId;
    if (targetId) {
      await tick();
      const el = document.getElementById('msg-' + targetId);
      if (el) {
        el.scrollIntoView({ behavior: 'instant', block: 'center' });
        el.classList.add('highlight-flash');
        setTimeout(() => el.classList.remove('highlight-flash'), 2000);
        pinToBottom = false;
        scrollToMessageId.set(null);
        suppressAutoScroll.set(false);
      }
      prevMessageCount = count;
      return;
    }

    // Auto-scroll only when the user is pinned to the bottom and
    // auto-scroll hasn't been suppressed (e.g. by a search-result click
    // that needs to scroll to a specific message instead of the bottom).
    if ($suppressAutoScroll) {
      prevMessageCount = count;
      return;
    }
    if (pinToBottom && (delta > 0 || $streaming)) {
      if (delta > 2) {
        messagesEl.scrollTop = messagesEl.scrollHeight;
      } else {
        messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
      }
    }
    prevMessageCount = count;
  });

  function handleScroll() {
    if (!messagesEl) return;
    const distFromBottom = messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight;
    pinToBottom = distFromBottom < PIN_TOLERANCE_PX;
    showScrollBtn = distFromBottom > 200;
  }

  function scrollToBottom() {
    if (!messagesEl) return;
    messagesEl.scrollTo({ top: messagesEl.scrollHeight, behavior: 'smooth' });
    pinToBottom = true;
  }

  $: messageCount = $messages.filter(m => m.role !== 'system').length;

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

</script>

<div class="chat-wrapper glass" class:streaming={$streaming}>
  <div class="chat-side">
    <div class="chat-header">
      <span>CHAT</span>
      <span class="msg-count">{messageCount} messages{#if totalTokens > 0} &middot; {formatTokens(totalTokens)} tokens{/if}</span>
    </div>

    <div class="messages-container">
      <div class="messages" bind:this={messagesEl} on:scroll={handleScroll}>
        {#if $messages.length === 0 && !$streaming}
          <div class="empty-state">
            <div class="empty-orb"></div>
          </div>
        {/if}
        {#each $messages as msg (msg.id)}
          <div class="msg-group" id="msg-{msg.id}">
            {#if msg.role === 'assistant' && msg.thoughts && msg.thoughts.length > 0}
              <ToolCallEnvelope thoughts={msg.thoughts} />
            {/if}
            <Message message={msg} {agentName} {agentAvatar} />
          </div>
        {/each}

        {#if $pendingSteps.length > 0}
          <ToolCallEnvelope steps={$pendingSteps} live={true} />
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

        {#if $streaming && !$streamingText && $pendingSteps.length === 0}
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

    <div class="input-wrap">
      <InputArea />
    </div>
  </div>

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

  .input-wrap {
    padding: 0 clamp(6px, 3vw, 46px);
  }

  .msg-group {
    display: flex;
    flex-direction: column;
  }

  .msg-group :global(.highlight-flash) {
    animation: search-highlight 2s ease-out;
  }

  @keyframes search-highlight {
    0%, 30% { background: var(--accent-ghost); border-radius: 6px; }
    100% { background: transparent; }
  }

  :global(.highlight-flash) {
    animation: search-highlight 2s ease-out;
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

  @media (max-width: 800px) {
    .messages { padding: 16px 16px; }
    .input-wrap { padding: 0 12px; }
  }
</style>
