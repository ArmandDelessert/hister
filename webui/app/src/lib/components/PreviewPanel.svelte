<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
  import VideoPreview from './VideoPreview.svelte';
  import { apiFetch } from '$lib/api';
  import { formatTimestamp, formatMetaDate } from '$lib/search';
  import { ScrollArea } from '@hister/components/ui/scroll-area';
  import { Button } from '@hister/components/ui/button';
  import { Eye, X } from 'lucide-svelte';

  interface Props {
    url: string;
    hintTitle?: string;
    onclose: () => void;
  }

  let { url, hintTitle = '', onclose }: Props = $props();

  let title = $state('');
  let content = $state('');
  let template = $state('');
  let templateData = $state<any>(null);
  let meta = $state<Record<string, any> | null>(null);
  let added = $state<number | null>(null);
  let loading = $state(false);

  function parseTemplateData(c: string): any | null {
    try {
      return JSON.parse(c);
    } catch {
      return null;
    }
  }

  $effect(() => {
    if (url) {
      loadContent(url, hintTitle);
    }
  });

  async function loadContent(u: string, hint: string) {
    loading = true;
    content = '';
    template = '';
    templateData = null;
    meta = null;
    added = null;
    title = hint;
    try {
      const resp = await apiFetch(`/preview?url=${encodeURIComponent(u)}`);
      if (!resp.ok) {
        content = `<p class="text-hister-rose">Failed to load readable content. Status: ${resp.status}</p>`;
      } else {
        const data = await resp.json();
        title = data.title || hint;
        added = data.added ?? null;
        meta = data.meta ?? null;
        template = data.template || '';
        templateData = template === 'video' ? parseTemplateData(data.content) : null;
        content = template === 'video' ? '' : data.content || '<p>No content available</p>';
      }
    } catch (err) {
      content = `<p class="text-hister-rose">Failed to load: ${err}</p>`;
    } finally {
      loading = false;
    }
  }
</script>

<div
  class="border-border-brand bg-card-surface flex flex-1 shrink-0 flex-col overflow-hidden border-l-[3px]"
>
  {#if loading}
    <div
      class="border-border-brand-muted flex shrink-0 items-center justify-end border-b-[2px] px-2 py-1"
    >
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-text-brand-muted hover:text-text-brand"
        onclick={onclose}
      >
        <X class="size-4" />
      </Button>
    </div>
    <div class="flex flex-1 items-center justify-center">
      <span class="font-inter text-text-brand-muted text-sm">Loading…</span>
    </div>
  {:else if content || templateData}
    <div
      class="border-border-brand-muted flex shrink-0 items-start gap-2 border-b-[2px] px-4 py-2.5"
    >
      <div class="flex flex-1 flex-col gap-0.5">
        <h2
          class="font-outfit text-text-brand line-clamp-2 text-lg leading-snug font-bold md:text-3xl"
        >
          <a href={url} target="_blank" rel="noopener noreferrer" class="hover:underline">{title}</a
          >
        </h2>
        {#if meta?.author || meta?.published || meta?.type}
          <span class="font-inter text-text-brand-muted text-xs">
            {#if meta?.author}<span>{meta.author}</span>{/if}
            {#if meta?.author && meta?.published}<span class="mx-1">·</span>{/if}
            {#if meta?.published}<span>{formatMetaDate(meta.published)}</span>{/if}
            {#if (meta?.author || meta?.published) && meta?.type}<span class="mx-1">·</span>{/if}
            {#if meta?.type}<span class="uppercase">{meta.type}</span>{/if}
          </span>
        {/if}
        {#if added}
          <span class="font-inter text-text-brand-muted text-xs" title={formatTimestamp(added)}
            >indexed {formatTimestamp(added)}</span
          >
        {/if}
        {#if meta?.description}
          <p class="font-inter text-text-brand-secondary mt-1 line-clamp-3 text-sm">
            {meta.description}
          </p>
        {/if}
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-text-brand-muted hover:text-text-brand mt-1 shrink-0"
        onclick={onclose}
      >
        <X class="size-4" />
      </Button>
    </div>
    <ScrollArea class="min-h-0 flex-1">
      <div
        class="font-inter text-text-brand-secondary prose dark:prose-invert prose-a:text-hister-teal w-full max-w-[60em] p-4 text-sm"
      >
        {#if template === 'video' && templateData}
          <VideoPreview data={templateData} />
        {:else}
          {@html content}
        {/if}
        {#if meta?.jsonld}
          <details class="not-prose border-border-brand-muted mt-6 border-t pt-3">
            <summary
              class="font-inter text-text-brand-muted cursor-pointer text-xs tracking-wide uppercase"
            >
              Extracted JSON-LD ({meta.jsonld.length})
            </summary>
            <pre
              class="bg-card-surface-muted text-text-brand-secondary mt-2 overflow-x-auto rounded p-2 text-[11px] leading-snug">{JSON.stringify(
                meta.jsonld,
                null,
                2,
              )}</pre>
          </details>
        {/if}
      </div>
    </ScrollArea>
  {:else}
    <div
      class="border-border-brand-muted flex shrink-0 items-center justify-end border-b-[2px] px-2 py-1"
    >
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-text-brand-muted hover:text-text-brand"
        onclick={onclose}
      >
        <X class="size-4" />
      </Button>
    </div>
    <div class="flex flex-1 flex-col items-center justify-center gap-2 opacity-40">
      <Eye class="size-6" />
      <p class="font-inter text-text-brand-muted text-sm">Focus a result to read it</p>
    </div>
  {/if}
</div>
