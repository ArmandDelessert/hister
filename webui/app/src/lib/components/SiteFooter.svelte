<script lang="ts">
  import { page } from '$app/stores';
  import { Button } from '@hister/components/ui/button';
  import { toggleMode, mode } from 'mode-watcher';
  import { Sun, Moon, Keyboard } from '@lucide/svelte';
  import { showHelp } from '$lib/stores';

  const links = [
    { label: 'Help', href: 'help', color: 'var(--hister-indigo)' },
    { label: 'Extractors', href: 'extractors', color: 'var(--hister-cyan)' },
    { label: 'About', href: 'about', color: 'var(--hister-teal)' },
    { label: 'API', href: 'api-docs', color: 'var(--hister-coral)' },
    {
      label: 'GitHub',
      href: 'https://github.com/asciimoo/hister/',
      color: 'var(--hister-amber)',
      external: true,
    },
  ];

  const iconBtn =
    'text-text-brand-muted hover:text-hister-indigo hover:bg-muted-surface size-8 shrink-0 transition-all';
  const linkCls =
    'footer-link font-space text-text-brand-muted flex h-8 items-center px-1.5 text-[11px] uppercase no-underline transition-colors hover:no-underline md:px-2 md:text-[12px]';
</script>

<footer
  class="site-footer bg-brutal-bg border-brutal-border relative grid h-12 shrink-0 grid-cols-[1fr_auto_1fr] items-center border-t-[2px] px-4 text-sm md:px-6"
>
  <span></span>

  <nav class="flex items-center gap-2 overflow-hidden px-1" aria-label="Secondary">
    {#each links as link (link.href)}
      <a
        href={link.href}
        class={linkCls}
        style="--footer-link-color: {link.color};"
        target={link.external ? '_blank' : undefined}
        rel={link.external ? 'noopener' : undefined}>{link.label}</a
      >
    {/each}
  </nav>

  <div class="flex items-center justify-end gap-1">
    <Button variant="ghost" size="icon" class={iconBtn} title="Toggle theme" onclick={toggleMode}>
      {#if mode.current === 'dark'}<Sun class="size-5" />{:else}<Moon class="size-5" />{/if}
    </Button>
    {#if $page.url.pathname === '/'}
      <Button
        variant="ghost"
        size="icon"
        class={iconBtn}
        title="Keyboard shortcuts (?)"
        aria-label="Show keyboard shortcuts"
        onclick={() => ($showHelp = !$showHelp)}
      >
        <Keyboard class="size-5" />
      </Button>
    {/if}
  </div>
</footer>

<style>
  .site-footer {
    box-shadow: 0 -1px 0 color-mix(in srgb, white 6%, transparent) inset;
  }

  :global(.dark) .site-footer {
    box-shadow: 0 -1px 0 color-mix(in srgb, white 7%, transparent) inset;
  }

  .footer-link:hover {
    color: color-mix(in srgb, var(--footer-link-color) 78%, var(--text-primary-brand));
  }

  :global(.dark) .footer-link:hover {
    color: color-mix(in srgb, var(--footer-link-color) 86%, white);
  }
</style>
