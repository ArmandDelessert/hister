<script lang="ts">
  import { resolve } from '$app/paths';
  import { Globe } from '@lucide/svelte';

  interface Props {
    favicon?: string;
    faviconKey?: string;
    class?: string;
  }

  let { favicon, faviconKey, class: className = '' }: Props = $props();

  let imageFailed = $state(false);

  const src = $derived.by(() => {
    if (favicon) return favicon;
    if (faviconKey) return `${resolve('/')}api/favicon?key=${encodeURIComponent(faviconKey)}`;
    return '';
  });

  $effect(() => {
    src;
    imageFailed = false;
  });
</script>

<div class="flex h-5 w-5 shrink-0 items-center justify-center overflow-hidden {className}">
  {#if src && !imageFailed}
    <img
      {src}
      alt=""
      class="h-full w-full object-cover"
      onerror={() => {
        imageFailed = true;
      }}
    />
  {:else}
    <Globe class="text-text-brand-muted size-3" />
  {/if}
</div>
