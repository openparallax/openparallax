<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ChatPanel from './components/ChatPanel.svelte';
  import DetailPanel from './components/DetailPanel.svelte';
  import Particles from './components/Particles.svelte';
  import { connect } from './lib/websocket';

  onMount(() => {
    connect();
  });
</script>

<div class="bg-gradient"></div>
<Particles />

<div class="app">
  <Sidebar />
  <ChatPanel />
  <DetailPanel />
</div>

<style>
  .bg-gradient {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    z-index: 0;
    background:
      radial-gradient(ellipse at 50% 40%, rgba(0, 220, 255, 0.03) 0%, transparent 60%),
      radial-gradient(ellipse at 20% 80%, rgba(0, 220, 255, 0.02) 0%, transparent 40%),
      var(--bg-void);
    pointer-events: none;
  }

  .app {
    position: relative;
    z-index: 1;
    display: flex;
    height: 100vh;
    gap: 1px;
    padding: 8px;
  }

  @media (max-width: 1200px) {
    .app { padding: 4px; }
  }
</style>
