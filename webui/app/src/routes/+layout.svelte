<script lang="ts">
  import { page } from '$app/stores';
  import { ModeWatcher, toggleMode, mode } from 'mode-watcher';
  import { Button } from '@hister/components/ui/button';
  import { Sun, Moon, LogIn, LogOut } from 'lucide-svelte';
  import '../style.css';
  import { fetchConfig, logout, resetConfig, type AppConfig } from '$lib/api';

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

  const navItems = [
    { label: 'History', href: 'history' },
    { label: 'Rules', href: 'rules' },
    { label: 'Add', href: 'add' },
  ];
</script>

<ModeWatcher />

<div class="flex h-dvh flex-col overflow-hidden">
  <header
    class="bg-brutal-bg border-brutal-border sticky top-0 z-50 flex h-12 shrink-0 items-center justify-between gap-2 overflow-hidden border-b-[3px] px-3 md:grid md:h-16 md:grid-cols-[4rem_auto_4rem] md:justify-stretch md:gap-4 md:px-6"
  >
    <h1 class="flex shrink-0 items-center gap-1.5 md:gap-2">
      <img src="static/logo.png" alt="Hister logo" class="h-6 w-6 md:h-8 md:w-8" />
      <a
        data-sveltekit-reload
        href="./"
        class="font-space text-text-brand text-lg font-extrabold tracking-[1px] uppercase no-underline hover:underline md:text-[28px] md:tracking-[2px]"
      >
        Hister
      </a>
    </h1>
    <nav class="flex items-center justify-self-center">
      {#each navItems as item (item.href)}
        <a
          class="font-space p-3 text-[11px] font-semibold tracking-[1px] uppercase no-underline hover:underline md:p-6 md:text-[13px] md:tracking-[1.5px] {$page
            .url.pathname === new URL(item.href, $page.url).pathname
            ? 'text-text-brand font-bold'
            : 'text-text-brand-secondary hover:text-text-brand'}"
          href={item.href}
        >
          {item.label}
        </a>
      {/each}
    </nav>
    <div class="flex items-center justify-self-end">
      {#if config?.authMode === 'user'}
        {#if config?.username}
          <Button
            variant="ghost"
            size="icon"
            class="text-text-brand-muted hover:text-hister-indigo size-8 shrink-0 transition-all hover:scale-110 md:size-10"
            title="Logout {config.username}"
            onclick={handleLogout}
          >
            <LogOut class="size-5" />
          </Button>
        {:else}
          <Button
            variant="ghost"
            size="icon"
            class="text-text-brand-muted hover:text-hister-indigo size-8 shrink-0 transition-all hover:scale-110 md:size-10"
            title="Login"
            onclick={() => (window.location.href = '/auth')}
          >
            <LogIn class="size-5" />
          </Button>
        {/if}
      {/if}
    </div>
  </header>

  <main class="flex min-h-0 flex-1 flex-col overflow-clip">
    {@render children()}
  </main>

  <footer
    class="bg-brutal-bg border-brutal-border flex h-12 items-center justify-center gap-6 border-t-[3px] px-6 text-sm"
  >
    <a
      href="help"
      class="font-space text-text-brand-secondary hover:text-hister-indigo text-[13px] tracking-[1px] uppercase no-underline hover:underline"
      >Help</a
    >
    <a
      href="about"
      class="font-space text-text-brand-secondary hover:text-hister-indigo text-[13px] tracking-[1px] uppercase no-underline hover:underline"
      >About</a
    >
    <a
      href="api-docs"
      class="font-space text-text-brand-secondary hover:text-hister-indigo text-[11px] tracking-[1px] uppercase no-underline hover:underline md:text-[13px]"
      >API</a
    >
    <a
      href="https://github.com/asciimoo/hister/"
      class="font-space text-text-brand-secondary hover:text-hister-indigo text-[13px] tracking-[1px] uppercase no-underline hover:underline"
      target="_blank"
      rel="noopener">GitHub</a
    >
    <Button
      variant="ghost"
      size="icon"
      class="text-text-brand-muted hover:text-hister-indigo size-8 shrink-0 transition-all hover:scale-110"
      title="Toggle theme"
      onclick={toggleMode}
    >
      {#if mode.current === 'dark'}<Sun class="size-5" />{:else}<Moon class="size-5" />{/if}
    </Button>
  </footer>
</div>
