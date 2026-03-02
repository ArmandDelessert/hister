<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { Button } from '@hister/components/ui/button';
  import { Sun } from 'lucide-svelte';
  import "../style.css";

  let { children } = $props();

  const navItems = [
    { label: 'History', href: 'history' },
    { label: 'Rules', href: 'rules' },
    { label: 'Add', href: 'add' }
  ];

  function applyTheme(theme: string) {
    document.documentElement.setAttribute('data-theme', theme);
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  onMount(() => {
    const theme = localStorage.getItem('theme') ||
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    applyTheme(theme);
  });

  function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'dark' ? 'light' : 'dark';
    applyTheme(next);
    localStorage.setItem('theme', next);
  }
</script>

<header class="h-16 px-6 bg-brutal-bg border-b-[3px] border-brutal-border flex items-center justify-between sticky top-0 z-50">
  <h1 class="flex items-center gap-2">
    <img src="static/logo.png" alt="Hister logo" class="h-8 w-8" />
    <a data-sveltekit-reload href="./" class="font-space text-[28px] tracking-[2px] font-extrabold text-text-brand no-underline hover:underline uppercase">
      Hister
    </a>
  </h1>
  <nav class="flex items-center gap-6">
    {#each navItems as item (item.href)}
      <a
        class="font-space text-[13px] tracking-[1.5px] font-semibold no-underline hover:underline uppercase {$page.url.pathname === new URL(item.href, $page.url).pathname ? 'text-text-brand font-bold' : 'text-text-brand-secondary hover:text-text-brand'}"
        href={item.href}
      >
        {item.label}
      </a>
    {/each}
  </nav>
  <Button
    variant="ghost"
    size="icon"
    class="text-text-brand-muted hover:text-hister-indigo transition-all hover:scale-110"
    title="Toggle theme"
    onclick={toggleTheme}
  >
    <Sun class="size-6" />
  </Button>
</header>

<main class="flex flex-col overflow-clip">
  {@render children()}
</main>

<footer class="h-12 px-6 bg-brutal-bg border-t-[3px] border-brutal-border flex items-center justify-center gap-6 text-sm">
  <a href="help" class="font-space text-[13px] tracking-[1px] text-text-brand-secondary hover:text-hister-indigo no-underline hover:underline uppercase">Help</a>
  <a href="about" class="font-space text-[13px] tracking-[1px] text-text-brand-secondary hover:text-hister-indigo no-underline hover:underline uppercase">About</a>
  <a href="api" class="font-space text-[13px] tracking-[1px] text-text-brand-secondary hover:text-hister-indigo no-underline hover:underline uppercase">API</a>
  <a href="https://github.com/asciimoo/hister/" class="font-space text-[13px] tracking-[1px] text-text-brand-secondary hover:text-hister-indigo no-underline hover:underline uppercase" target="_blank" rel="noopener">GitHub</a>
</footer>
