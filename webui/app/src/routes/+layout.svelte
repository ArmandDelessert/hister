<script lang="ts">
  import { ModeWatcher } from 'mode-watcher';
  import SiteHeader from '$lib/components/SiteHeader.svelte';
  import SiteFooter from '$lib/components/SiteFooter.svelte';
  import { fetchConfig, logout, resetConfig, type AppConfig } from '$lib/api';
  import '../style.css';

  let { children } = $props();

  let config = $state<AppConfig | null>(null);

  $effect(() => {
    fetchConfig()
      .then((c) => (config = c))
      .catch(() => {});
  });

  async function handleLogout() {
    await logout();
    resetConfig();
    config = null;
    window.location.href = '/';
  }
</script>

<ModeWatcher />

<div class="flex h-dvh flex-col overflow-hidden">
  <SiteHeader {config} onLogout={handleLogout} />

  <main class="flex min-h-0 flex-1 flex-col overflow-clip">
    {@render children()}
  </main>

  <SiteFooter />
</div>
