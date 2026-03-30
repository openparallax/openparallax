<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { ShieldCheck, Zap, ArrowRight, Check, X, Loader2 } from 'lucide-svelte';

  const dispatch = createEventDispatcher();

  let step = 0;
  let agentName = '';
  let avatar = '⬡';
  let provider = '';
  let apiKey = '';
  let model = '';
  let testStatus: 'idle' | 'testing' | 'success' | 'error' = 'idle';
  let testError = '';
  let embProvider = '';
  let embApiKey = '';
  let embTestStatus: 'idle' | 'testing' | 'success' | 'error' = 'idle';
  let workspace = '';
  let launching = false;

  const avatars = [
    { emoji: '⬡', label: 'hexagon' },
    { emoji: '🤖', label: 'robot' },
    { emoji: '🧠', label: 'brain' },
    { emoji: '⚡', label: 'lightning' },
    { emoji: '🛡️', label: 'shield' },
  ];

  const providerCards = [
    { id: 'anthropic', name: 'Anthropic', model: 'claude-sonnet-4-20250514', shieldModel: 'claude-haiku-4-5-20251001' },
    { id: 'openai', name: 'OpenAI', model: 'gpt-4o', shieldModel: 'gpt-4o-mini' },
    { id: 'google', name: 'Google', model: 'gemini-2.0-flash', shieldModel: 'gemini-2.0-flash' },
    { id: 'ollama', name: 'Ollama', model: 'llama3.2', shieldModel: 'llama3.2' },
  ];

  const apiKeyUrls: Record<string, string> = {
    anthropic: 'https://console.anthropic.com/settings/keys',
    openai: 'https://platform.openai.com/api-keys',
    google: 'https://aistudio.google.com/apikey',
  };

  const embOptions = [
    { id: 'openai', label: 'OpenAI (recommended)', model: 'text-embedding-3-small' },
    { id: 'google', label: 'Google', model: 'text-embedding-004' },
    { id: '', label: 'Skip (keyword search only)', model: '' },
  ];

  $: displayName = agentName || 'Atlas';
  $: selectedProvider = providerCards.find(p => p.id === provider);
  $: needsEmbedding = provider === 'anthropic';
  $: canProceedStep2 = provider !== '';
  $: canProceedStep3 = provider === 'ollama' || (testStatus === 'success');

  function selectProvider(id: string) {
    provider = id;
    const p = providerCards.find(c => c.id === id);
    if (p) model = p.model;
    testStatus = 'idle';
    testError = '';
    apiKey = '';
  }

  async function testConnection() {
    testStatus = 'testing';
    testError = '';
    try {
      const resp = await fetch('/api/setup/test-provider', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider, api_key: apiKey, model, base_url: '' }),
      });
      const data = await resp.json();
      if (data.success) {
        testStatus = 'success';
      } else {
        testStatus = 'error';
        testError = data.error || 'Connection failed';
      }
    } catch (e: any) {
      testStatus = 'error';
      testError = e.message || 'Network error';
    }
  }

  async function testEmbedding() {
    embTestStatus = 'testing';
    try {
      const resp = await fetch('/api/setup/test-embedding', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: embProvider,
          api_key: embApiKey,
          model: embOptions.find(e => e.id === embProvider)?.model || '',
          base_url: '',
        }),
      });
      const data = await resp.json();
      embTestStatus = data.success ? 'success' : 'error';
    } catch {
      embTestStatus = 'error';
    }
  }

  async function launch() {
    launching = true;
    const name = agentName || 'Atlas';
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    const ws = workspace || `~/.openparallax/${slug}`;

    const embModel = embOptions.find(e => e.id === embProvider)?.model || '';

    try {
      const resp = await fetch('/api/setup/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          agent: { name, avatar },
          llm: { provider, api_key: apiKey, model, base_url: '' },
          embedding: { provider: embProvider, api_key: embApiKey, model: embModel },
          workspace: ws,
        }),
      });
      const data = await resp.json();
      if (data.success) {
        dispatch('complete', data);
      } else {
        launching = false;
      }
    } catch {
      launching = false;
    }
  }

  function next() { step++; }
  function prev() { step--; }
</script>

<div class="wizard-overlay">
  <div class="wizard-container">
    {#if step === 0}
      <!-- Welcome -->
      <div class="wizard-card" style="text-align: center;">
        <h1 class="wizard-title">OPENPARALLAX</h1>
        <p class="wizard-tagline">Your personal AI agent, secured.</p>
        <p class="wizard-desc">
          Everything runs on your machine. Your data never leaves.<br />
          Setup takes under a minute.
        </p>
        <button class="wizard-btn primary" on:click={next}>
          Get Started <ArrowRight size={16} />
        </button>
      </div>

    {:else if step === 1}
      <!-- Name + Avatar -->
      <div class="wizard-card">
        <h2 class="wizard-heading">Name your agent</h2>
        <p class="wizard-sub">This is how they'll introduce themselves.</p>
        <input
          class="wizard-input"
          bind:value={agentName}
          placeholder="Atlas"
          maxlength="20"
        />
        <div class="wizard-preview">{displayName} {avatar}</div>

        <h3 class="wizard-label">Pick an avatar</h3>
        <div class="avatar-grid">
          {#each avatars as a}
            <button
              class="avatar-btn"
              class:selected={avatar === a.emoji}
              on:click={() => avatar = a.emoji}
            >
              <span class="avatar-emoji">{a.emoji}</span>
              <span class="avatar-label">{a.label}</span>
            </button>
          {/each}
        </div>

        <div class="wizard-nav">
          <button class="wizard-btn secondary" on:click={prev}>Back</button>
          <button class="wizard-btn primary" on:click={next}>
            Continue <ArrowRight size={16} />
          </button>
        </div>
      </div>

    {:else if step === 2}
      <!-- Provider -->
      <div class="wizard-card">
        <h2 class="wizard-heading">Choose your provider</h2>
        <div class="provider-grid">
          {#each providerCards as p}
            <button
              class="provider-card"
              class:selected={provider === p.id}
              on:click={() => selectProvider(p.id)}
            >
              <span class="provider-name">{p.name}</span>
              <span class="provider-model">{p.model}</span>
            </button>
          {/each}
        </div>

        {#if provider && provider !== 'ollama'}
          <div class="api-key-section">
            <input
              class="wizard-input"
              type="password"
              bind:value={apiKey}
              placeholder="Enter API key..."
            />
            <a class="key-link" href={apiKeyUrls[provider]} target="_blank" rel="noopener">
              Get an API key
            </a>
            <button
              class="wizard-btn test-btn"
              on:click={testConnection}
              disabled={!apiKey.trim() || testStatus === 'testing'}
            >
              {#if testStatus === 'testing'}
                <Loader2 size={14} class="spin" /> Testing...
              {:else if testStatus === 'success'}
                <Check size={14} /> Connected
              {:else if testStatus === 'error'}
                <X size={14} /> Retry
              {:else}
                Test Connection
              {/if}
            </button>
            {#if testError}
              <p class="test-error">{testError}</p>
            {/if}
          </div>
        {/if}

        <div class="wizard-nav">
          <button class="wizard-btn secondary" on:click={prev}>Back</button>
          <button class="wizard-btn primary" on:click={next} disabled={!canProceedStep3 && canProceedStep2}>
            Continue <ArrowRight size={16} />
          </button>
        </div>
      </div>

    {:else if step === 3 && needsEmbedding}
      <!-- Embedding (Anthropic only) -->
      <div class="wizard-card">
        <h2 class="wizard-heading">Embedding provider</h2>
        <p class="wizard-sub">Anthropic doesn't offer embeddings. Choose one for semantic search:</p>
        <div class="emb-grid">
          {#each embOptions as e}
            <button
              class="provider-card small"
              class:selected={embProvider === e.id}
              on:click={() => { embProvider = e.id; embTestStatus = 'idle'; }}
            >
              {e.label}
            </button>
          {/each}
        </div>

        {#if embProvider && embProvider !== 'ollama'}
          <input
            class="wizard-input"
            type="password"
            bind:value={embApiKey}
            placeholder="Embedding API key..."
          />
        {/if}

        <div class="wizard-nav">
          <button class="wizard-btn secondary" on:click={prev}>Back</button>
          <button class="wizard-btn primary" on:click={() => step = 4}>
            Continue <ArrowRight size={16} />
          </button>
        </div>
      </div>

    {:else}
      <!-- Ready -->
      <div class="wizard-card" style="text-align: center;">
        <div class="ready-avatar">{avatar}</div>
        <h2 class="ready-name">{displayName}</h2>
        <div class="ready-details">
          <div class="ready-row">
            <span class="ready-label">Provider</span>
            <span class="ready-value">{selectedProvider?.name || provider}</span>
          </div>
          <div class="ready-row">
            <span class="ready-label">Model</span>
            <span class="ready-value">{model}</span>
          </div>
          <div class="ready-row">
            <span class="ready-label">Shield</span>
            <span class="ready-value"><ShieldCheck size={12} /> default policy</span>
          </div>
        </div>

        <p class="wizard-sub">You can change any of these in Settings.</p>

        <button class="wizard-btn primary launch-btn" on:click={launch} disabled={launching}>
          {#if launching}
            <Loader2 size={16} class="spin" /> Launching...
          {:else}
            <Zap size={16} /> Launch {displayName}
          {/if}
        </button>

        <div class="wizard-nav" style="justify-content: center;">
          <button class="wizard-btn secondary" on:click={prev}>Back</button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .wizard-overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
  }

  .wizard-container {
    width: 100%;
    max-width: 520px;
  }

  .wizard-card {
    background: rgba(12, 16, 28, 0.88);
    backdrop-filter: blur(24px);
    border: 1px solid var(--accent-border-active);
    border-radius: 16px;
    padding: 40px 36px;
    animation: fadeIn 300ms ease;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(12px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .wizard-title {
    font-family: 'Exo 2', sans-serif;
    font-weight: 800;
    font-size: 32px;
    letter-spacing: 0.06em;
    color: var(--text-primary);
    margin: 0 0 8px;
  }

  .wizard-tagline {
    font-size: 15px;
    color: var(--accent);
    margin: 0 0 16px;
    font-weight: 500;
  }

  .wizard-desc {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 28px;
  }

  .wizard-heading {
    font-family: 'Exo 2', sans-serif;
    font-weight: 700;
    font-size: 20px;
    color: var(--text-primary);
    margin: 0 0 4px;
  }

  .wizard-sub {
    font-size: 13px;
    color: var(--text-tertiary);
    margin: 0 0 20px;
  }

  .wizard-label {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    margin: 20px 0 10px;
  }

  .wizard-input {
    width: 100%;
    padding: 12px 16px;
    background: rgba(12, 16, 28, 0.6);
    border: 1px solid var(--accent-border);
    border-radius: 8px;
    color: var(--text-primary);
    font-family: 'Exo 2', sans-serif;
    font-size: 15px;
    outline: none;
    transition: border-color 200ms ease;
    box-sizing: border-box;
  }
  .wizard-input:focus {
    border-color: var(--accent-border-active);
  }
  .wizard-input::placeholder { color: var(--text-tertiary); }

  .wizard-preview {
    text-align: center;
    font-family: 'Exo 2', sans-serif;
    font-weight: 800;
    font-size: 28px;
    color: var(--text-primary);
    letter-spacing: 0.04em;
    margin: 16px 0 0;
  }

  .avatar-grid {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .avatar-btn {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 10px 14px;
    background: rgba(12, 16, 28, 0.5);
    border: 1px solid var(--accent-border);
    border-radius: 8px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 150ms ease;
    font-size: 11px;
  }
  .avatar-btn:hover { border-color: var(--accent-border-active); }
  .avatar-btn.selected {
    border-color: var(--accent);
    background: var(--accent-ghost);
  }
  .avatar-emoji { font-size: 22px; }
  .avatar-label { font-family: 'JetBrains Mono', monospace; }

  .provider-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
    margin-bottom: 16px;
  }

  .provider-card {
    padding: 16px;
    background: rgba(12, 16, 28, 0.5);
    border: 1px solid var(--accent-border);
    border-radius: 10px;
    cursor: pointer;
    text-align: left;
    color: var(--text-primary);
    transition: all 150ms ease;
  }
  .provider-card:hover { border-color: var(--accent-border-active); }
  .provider-card.selected {
    border-color: var(--accent);
    background: var(--accent-ghost);
  }
  .provider-card.small { padding: 10px 14px; font-size: 13px; }
  .provider-name {
    display: block;
    font-weight: 600;
    font-size: 15px;
    margin-bottom: 4px;
  }
  .provider-model {
    display: block;
    font-size: 11px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
  }

  .api-key-section {
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .key-link {
    font-size: 11px;
    color: var(--accent);
    text-decoration: none;
  }
  .key-link:hover { text-decoration: underline; }

  .test-error {
    font-size: 12px;
    color: var(--error);
    margin: 0;
  }

  .emb-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 16px;
  }

  .wizard-nav {
    display: flex;
    justify-content: space-between;
    margin-top: 24px;
    gap: 10px;
  }

  .wizard-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 10px 20px;
    border-radius: 8px;
    border: none;
    font-family: 'Exo 2', sans-serif;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 200ms ease;
  }
  .wizard-btn.primary {
    background: linear-gradient(135deg, var(--accent), rgba(0, 180, 220, 1));
    color: #06060c;
    box-shadow: 0 0 15px rgba(0, 220, 255, 0.2);
  }
  .wizard-btn.primary:hover:not(:disabled) {
    box-shadow: var(--accent-glow-strong);
    transform: translateY(-1px);
  }
  .wizard-btn.secondary {
    background: rgba(12, 16, 28, 0.6);
    border: 1px solid var(--accent-border);
    color: var(--text-secondary);
  }
  .wizard-btn.secondary:hover { border-color: var(--accent-border-active); }
  .wizard-btn.test-btn {
    background: rgba(12, 16, 28, 0.6);
    border: 1px solid var(--accent-border);
    color: var(--text-primary);
    align-self: flex-start;
  }
  .wizard-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .launch-btn { font-size: 16px; padding: 14px 32px; }

  .ready-avatar { font-size: 48px; margin-bottom: 8px; }
  .ready-name {
    font-family: 'Exo 2', sans-serif;
    font-weight: 800;
    font-size: 36px;
    letter-spacing: 0.04em;
    color: var(--text-primary);
    margin: 0 0 20px;
    text-transform: uppercase;
  }

  .ready-details {
    text-align: left;
    margin: 0 auto 20px;
    max-width: 280px;
  }
  .ready-row {
    display: flex;
    justify-content: space-between;
    padding: 6px 0;
    font-size: 13px;
    border-bottom: 1px solid var(--accent-border);
  }
  .ready-label { color: var(--text-tertiary); }
  .ready-value {
    color: var(--text-primary);
    font-family: 'JetBrains Mono', monospace;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
