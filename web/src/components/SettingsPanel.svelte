<script lang="ts">
  import { onMount } from 'svelte';
  import { X, User, Cpu, Shield, Brain, Mail, Calendar, Globe, Info, Save, RotateCw, Loader2, Check } from 'lucide-svelte';
  import { settingsOpen } from '../stores/settings';
  import { getSettings, putSettings } from '../lib/api';

  let settings: Record<string, any> = {};
  let loading = true;
  let saving = false;
  let saveResult: { success: boolean; restart_required: boolean; changed: string[] } | null = null;
  let dirty = false;

  // Editable fields.
  let agentName = '';
  let agentAvatar = '';
  let llmProvider = '';
  let llmModel = '';
  let llmBaseURL = '';
  let shieldEvalProvider = '';
  let shieldEvalModel = '';
  let tier2Budget = 50;
  let embProvider = '';
  let embModel = '';
  let webPort = 3100;

  const sections = [
    { id: 'identity', label: 'Agent Identity', icon: User },
    { id: 'llm', label: 'LLM Provider', icon: Cpu },
    { id: 'shield', label: 'Shield', icon: Shield },
    { id: 'memory', label: 'Memory', icon: Brain },
    { id: 'email', label: 'Email', icon: Mail },
    { id: 'calendar', label: 'Calendar', icon: Calendar },
    { id: 'web', label: 'Web UI', icon: Globe },
    { id: 'about', label: 'About', icon: Info },
  ];

  let expandedSection: string | null = 'identity';

  onMount(async () => {
    await loadSettings();
  });

  async function loadSettings() {
    loading = true;
    try {
      settings = await getSettings();
      agentName = settings.agent?.name || '';
      agentAvatar = settings.agent?.avatar || '';
      llmProvider = settings.llm?.provider || '';
      llmModel = settings.llm?.model || '';
      llmBaseURL = settings.llm?.base_url || '';
      shieldEvalProvider = settings.shield?.evaluator?.provider || '';
      shieldEvalModel = settings.shield?.evaluator?.model || '';
      tier2Budget = settings.shield?.tier2_budget || 50;
      embProvider = settings.memory?.embedding?.provider || '';
      embModel = settings.memory?.embedding?.model || '';
      webPort = settings.web?.port || 3100;
    } catch {
      /* engine not ready */
    }
    loading = false;
  }

  function markDirty() { dirty = true; saveResult = null; }

  async function save() {
    saving = true;
    const changes: Record<string, any> = {};

    if (agentName !== (settings.agent?.name || '')) {
      changes.agent = { ...changes.agent, name: agentName };
    }
    if (agentAvatar !== (settings.agent?.avatar || '')) {
      changes.agent = { ...changes.agent, avatar: agentAvatar };
    }
    if (llmProvider !== (settings.llm?.provider || '')) {
      changes.llm = { ...changes.llm, provider: llmProvider };
    }
    if (llmModel !== (settings.llm?.model || '')) {
      changes.llm = { ...changes.llm, model: llmModel };
    }
    if (llmBaseURL !== (settings.llm?.base_url || '')) {
      changes.llm = { ...changes.llm, base_url: llmBaseURL };
    }
    if (shieldEvalProvider !== (settings.shield?.evaluator?.provider || '')) {
      changes.shield = { ...changes.shield, evaluator: { ...changes.shield?.evaluator, provider: shieldEvalProvider } };
    }
    if (shieldEvalModel !== (settings.shield?.evaluator?.model || '')) {
      changes.shield = { ...changes.shield, evaluator: { ...changes.shield?.evaluator, model: shieldEvalModel } };
    }
    if (tier2Budget !== (settings.shield?.tier2_budget || 50)) {
      changes.shield = { ...changes.shield, tier2_budget: tier2Budget };
    }
    if (embProvider !== (settings.memory?.embedding?.provider || '')) {
      changes.memory = { embedding: { ...changes.memory?.embedding, provider: embProvider } };
    }
    if (embModel !== (settings.memory?.embedding?.model || '')) {
      changes.memory = { embedding: { ...changes.memory?.embedding, model: embModel } };
    }
    if (webPort !== (settings.web?.port || 3100)) {
      changes.web = { port: webPort };
    }

    if (Object.keys(changes).length === 0) {
      saving = false;
      dirty = false;
      return;
    }

    try {
      saveResult = await putSettings(changes);
      dirty = false;
      await loadSettings();
    } catch (e: any) {
      saveResult = { success: false, restart_required: false, changed: [] };
    }
    saving = false;
  }

  async function restart() {
    try {
      await fetch('/api/restart', { method: 'POST' });
      saveResult = null;
    } catch { /* ws will drop and reconnect */ }
  }

  function close() { settingsOpen.set(false); }
  function handleKeydown(e: KeyboardEvent) { if (e.key === 'Escape') close(); }
  function handleBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('settings-backdrop')) close();
  }
  function toggleSection(id: string) { expandedSection = expandedSection === id ? null : id; }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $settingsOpen}
  <div class="settings-backdrop" on:click={handleBackdropClick} role="presentation">
    <div class="settings-panel glass-elevated">
      <div class="settings-header">
        <h2 class="settings-title">Settings</h2>
        <div class="header-actions">
          {#if dirty}
            <button class="save-btn" on:click={save} disabled={saving}>
              {#if saving}<Loader2 size={14} class="spin" />{:else}<Save size={14} />{/if}
              Save
            </button>
          {/if}
          <button class="close-btn" on:click={close}><X size={16} /></button>
        </div>
      </div>

      {#if saveResult?.restart_required}
        <div class="restart-banner">
          Some changes require a restart.
          <button class="restart-btn" on:click={restart}><RotateCw size={12} /> Restart Now</button>
        </div>
      {/if}
      {#if saveResult?.success && !saveResult?.restart_required}
        <div class="success-banner"><Check size={12} /> Settings saved</div>
      {/if}

      <div class="settings-body">
        {#if loading}
          <div class="loading"><Loader2 size={20} class="spin" /></div>
        {:else}
          {#each sections as section (section.id)}
            <div class="settings-section">
              <button class="section-header" on:click={() => toggleSection(section.id)}>
                <svelte:component this={section.icon} size={14} />
                <span>{section.label}</span>
              </button>
              {#if expandedSection === section.id}
                <div class="section-content">
                  {#if section.id === 'identity'}
                    <label class="field">
                      <span class="field-label">AGENT NAME</span>
                      <input class="field-input" bind:value={agentName} on:input={markDirty} />
                    </label>
                    <label class="field">
                      <span class="field-label">AVATAR</span>
                      <input class="field-input" bind:value={agentAvatar} on:input={markDirty} maxlength="2" />
                    </label>

                  {:else if section.id === 'llm'}
                    <label class="field">
                      <span class="field-label">PROVIDER</span>
                      <select class="field-input" bind:value={llmProvider} on:change={markDirty}>
                        <option value="anthropic">Anthropic</option>
                        <option value="openai">OpenAI</option>
                        <option value="google">Google</option>
                        <option value="ollama">Ollama</option>
                      </select>
                    </label>
                    <label class="field">
                      <span class="field-label">MODEL</span>
                      <input class="field-input" bind:value={llmModel} on:input={markDirty} />
                    </label>
                    <div class="field">
                      <span class="field-label">API KEY</span>
                      <div class="field-value">{settings.llm?.api_key_configured ? '✓ Configured' : '✗ Not set'}</div>
                    </div>
                    <label class="field">
                      <span class="field-label">BASE URL</span>
                      <input class="field-input" bind:value={llmBaseURL} on:input={markDirty} placeholder="Default" />
                    </label>

                  {:else if section.id === 'shield'}
                    <div class="field">
                      <span class="field-label">POLICY</span>
                      <div class="field-value">{settings.shield?.policy || 'default'}</div>
                    </div>
                    <label class="field">
                      <span class="field-label">EVALUATOR PROVIDER</span>
                      <select class="field-input" bind:value={shieldEvalProvider} on:change={markDirty}>
                        <option value="anthropic">Anthropic</option>
                        <option value="openai">OpenAI</option>
                        <option value="google">Google</option>
                        <option value="ollama">Ollama</option>
                      </select>
                    </label>
                    <label class="field">
                      <span class="field-label">EVALUATOR MODEL</span>
                      <input class="field-input" bind:value={shieldEvalModel} on:input={markDirty} />
                    </label>
                    <label class="field">
                      <span class="field-label">TIER 2 BUDGET</span>
                      <input class="field-input" type="number" min="10" max="500" bind:value={tier2Budget} on:input={markDirty} />
                    </label>
                    <div class="field">
                      <span class="field-label">TIER 2 USED TODAY</span>
                      <div class="field-value">{settings.shield?.tier2_used_today || 0}</div>
                    </div>

                  {:else if section.id === 'memory'}
                    <label class="field">
                      <span class="field-label">EMBEDDING PROVIDER</span>
                      <select class="field-input" bind:value={embProvider} on:change={markDirty}>
                        <option value="">None</option>
                        <option value="openai">OpenAI</option>
                        <option value="google">Google</option>
                        <option value="ollama">Ollama</option>
                      </select>
                    </label>
                    <label class="field">
                      <span class="field-label">EMBEDDING MODEL</span>
                      <input class="field-input" bind:value={embModel} on:input={markDirty} />
                    </label>
                    <div class="field">
                      <span class="field-label">API KEY</span>
                      <div class="field-value">{settings.memory?.embedding?.api_key_configured ? '✓ Configured' : '✗ Not set'}</div>
                    </div>

                  {:else if section.id === 'email'}
                    <div class="field">
                      <span class="field-label">STATUS</span>
                      <div class="field-value">{settings.email?.configured ? '✓ Configured' : 'Not configured'}</div>
                    </div>
                    {#if settings.email?.configured}
                      <div class="field">
                        <span class="field-label">FROM</span>
                        <div class="field-value">{settings.email?.from || ''}</div>
                      </div>
                    {/if}

                  {:else if section.id === 'calendar'}
                    <div class="field">
                      <span class="field-label">STATUS</span>
                      <div class="field-value">{settings.calendar?.configured ? `✓ ${settings.calendar.provider}` : 'Not configured'}</div>
                    </div>

                  {:else if section.id === 'web'}
                    <label class="field">
                      <span class="field-label">PORT</span>
                      <input class="field-input" type="number" min="1024" max="65535" bind:value={webPort} on:input={markDirty} />
                    </label>

                  {:else if section.id === 'about'}
                    <div class="field">
                      <span class="field-label">SANDBOX</span>
                      <div class="field-value">{settings.sandbox?.active ? `✓ ${settings.sandbox.mode}` : '✗ Inactive'}</div>
                    </div>
                    <div class="field">
                      <span class="field-label">MCP SERVERS</span>
                      <div class="field-value">{settings.mcp?.servers?.length || 0} configured</div>
                    </div>
                    <div class="field">
                      <span class="field-label">LICENSE</span>
                      <div class="field-value">Apache 2.0</div>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .settings-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: flex;
  }

  .settings-panel {
    width: 400px;
    height: 100%;
    display: flex;
    flex-direction: column;
    animation: slide-from-left 300ms ease-out;
    overflow: hidden;
  }

  @keyframes slide-from-left {
    from { transform: translateX(-100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }

  .settings-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid var(--accent-border);
  }

  .settings-title {
    font-family: 'Exo 2', sans-serif;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .header-actions {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .save-btn {
    display: flex; align-items: center; gap: 4px;
    padding: 6px 14px;
    border-radius: 6px;
    border: none;
    background: linear-gradient(135deg, var(--accent), rgba(0, 180, 220, 1));
    color: #06060c;
    font-family: 'Exo 2', sans-serif;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 150ms ease;
  }
  .save-btn:hover:not(:disabled) { box-shadow: var(--accent-glow); }
  .save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .close-btn {
    width: 32px; height: 32px;
    border-radius: 6px;
    border: 1px solid var(--accent-border);
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    transition: all 150ms ease;
  }
  .close-btn:hover {
    border-color: var(--accent-border-active);
    color: var(--text-primary);
  }

  .restart-banner {
    padding: 10px 24px;
    background: rgba(255, 171, 0, 0.08);
    border-bottom: 1px solid rgba(255, 171, 0, 0.15);
    color: var(--warning);
    font-size: 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .restart-btn {
    display: flex; align-items: center; gap: 4px;
    padding: 4px 10px;
    border-radius: 4px;
    border: 1px solid rgba(255, 171, 0, 0.3);
    background: none;
    color: var(--warning);
    font-size: 11px;
    cursor: pointer;
    white-space: nowrap;
  }
  .restart-btn:hover { background: rgba(255, 171, 0, 0.1); }

  .success-banner {
    padding: 8px 24px;
    background: rgba(0, 230, 118, 0.06);
    border-bottom: 1px solid rgba(0, 230, 118, 0.12);
    color: var(--success);
    font-size: 12px;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .loading {
    display: flex;
    justify-content: center;
    padding: 40px;
    color: var(--text-tertiary);
  }

  .settings-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .settings-section {
    border-radius: var(--radius);
    border: 1px solid var(--accent-border);
    overflow: hidden;
  }

  .section-header {
    width: 100%;
    padding: 12px 16px;
    display: flex;
    align-items: center;
    gap: 10px;
    border: none;
    background: var(--accent-ghost);
    color: var(--text-primary);
    font-family: 'Exo 2', sans-serif;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: background 150ms ease;
    text-align: left;
  }
  .section-header:hover { background: var(--accent-subtle); }

  .section-content {
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    border-top: 1px solid var(--accent-border);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-label {
    font-size: 10px;
    color: var(--text-tertiary);
    font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.06em;
  }

  .field-value {
    font-size: 13px;
    color: var(--text-primary);
  }

  .field-input {
    width: 100%;
    padding: 8px 10px;
    background: rgba(12, 16, 28, 0.5);
    border: 1px solid var(--accent-border);
    border-radius: 4px;
    color: var(--text-primary);
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    outline: none;
    box-sizing: border-box;
    transition: border-color 150ms ease;
  }
  .field-input:focus { border-color: var(--accent-border-active); }
  .field-input::placeholder { color: var(--text-tertiary); }

  select.field-input {
    appearance: none;
    cursor: pointer;
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
