<script lang="ts">
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import { Button, Separator } from '@hister/components';
  import ImageLightbox from '$lib/ImageLightbox.svelte';

  let { data } = $props();

  interface TocEntry {
    id: string;
    text: string;
    level: number;
  }

  let toc = $state<TocEntry[]>([]);
  let activeId = $state('');

  $effect(() => {
    // Track data.content as a dependency so this re-runs when navigating between docs
    void data.content;
    activeId = '';

    const article = document.querySelector('[data-doc-content]');
    if (!article) return;

    const headings = article.querySelectorAll('h2, h3');
    toc = Array.from(headings).map((h) => ({
      id: h.id,
      text: h.textContent ?? '',
      level: h.tagName === 'H2' ? 2 : 3,
    }));

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeId = entry.target.id;
          }
        }
      },
      { rootMargin: '-80px 0px -60% 0px', threshold: 0 },
    );

    headings.forEach((h) => observer.observe(h));
    return () => observer.disconnect();
  });
</script>

<svelte:head>
  <title>{data.meta.title} | Hister Documentation</title>
</svelte:head>

<div class="flex gap-10">
  <article class="min-w-0 flex-1" data-doc-content>
    <div class="content doc-content">
      <data.content />
    </div>

    <!-- Prev / Next -->
    <Separator class="bg-brutal-border mt-12 h-0.75" />
    <nav class="flex items-center justify-between gap-4 pt-8">
      {#if data.prev}
        <Button
          variant="ghost"
          href="/docs/{data.prev.slug}"
          class="group flex h-auto items-center gap-3 rounded-none px-2 py-2 text-(--text-secondary) no-underline transition-colors hover:text-(--text-primary)"
        >
          <ArrowLeft size={18} class="transition-transform group-hover:-translate-x-1" />
          <div class="flex flex-col items-start">
            <span
              class="font-space text-[10px] font-bold tracking-[2px] text-(--text-secondary) uppercase"
              >Previous</span
            >
            <span class="font-inter text-sm font-semibold">{data.prev.title}</span>
          </div>
        </Button>
      {:else}
        <div></div>
      {/if}

      {#if data.next}
        <Button
          variant="ghost"
          href="/docs/{data.next.slug}"
          class="group flex h-auto items-center gap-3 rounded-none px-2 py-2 text-right text-(--text-secondary) no-underline transition-colors hover:text-(--text-primary)"
        >
          <div class="flex flex-col items-end">
            <span
              class="font-space text-[10px] font-bold tracking-[2px] text-(--text-secondary) uppercase"
              >Next</span
            >
            <span class="font-inter text-sm font-semibold">{data.next.title}</span>
          </div>
          <ArrowRight size={18} class="transition-transform group-hover:translate-x-1" />
        </Button>
      {:else}
        <div></div>
      {/if}
    </nav>
  </article>

  <!-- TOC Sidebar (xl only) -->
  {#if toc.length > 0}
    <aside class="hidden w-48 shrink-0 xl:block">
      <nav class="sticky top-24 flex flex-col gap-0.5">
        <span
          class="font-space mb-3 text-[10px] font-bold tracking-[2px] text-(--text-secondary) uppercase"
          >On This Page</span
        >
        {#each toc as entry}
          <Button
            variant="ghost"
            href="#{entry.id}"
            class="font-inter h-auto justify-start rounded-none border-l-2 py-1 text-left text-[13px] whitespace-normal no-underline transition-colors
              {entry.level === 3 ? 'pl-5' : 'pl-3'}
              {activeId === entry.id
              ? 'border-hister-indigo font-medium text-(--text-primary)'
              : 'hover:border-brutal-border border-transparent text-(--text-secondary) hover:text-(--text-primary)'}"
          >
            {entry.text}
          </Button>
        {/each}
      </nav>
    </aside>
  {/if}
</div>

<ImageLightbox />
