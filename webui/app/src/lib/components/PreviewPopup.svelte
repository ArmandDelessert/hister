<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
  import * as Dialog from '@hister/components/ui/dialog';
  import VideoPreview from './VideoPreview.svelte';
  import { apiFetch } from '$lib/api';
  import { formatMetaDate } from '$lib/search';

  interface Props {
    open: boolean;
    url: string;
    hintTitle?: string;
  }

  let { open = $bindable(false), url, hintTitle = '' }: Props = $props();

  let title = $state('');
  let content = $state('');
  let template = $state('');
  let templateData = $state<any>(null);
  let meta = $state<Record<string, any> | null>(null);

  function parseTemplateData(c: string): any | null {
    try {
      return JSON.parse(c);
    } catch {
      return null;
    }
  }

  $effect(() => {
    if (open && url) {
      loadContent(url, hintTitle);
    }
  });

  async function loadContent(u: string, hint: string) {
    title = hint;
    content = '';
    template = '';
    templateData = null;
    meta = null;
    try {
      const resp = await apiFetch(`/preview?url=${encodeURIComponent(u)}`);
      if (!resp.ok) {
        title = 'Error';
        content = `<p class="text-hister-rose">Failed to load readable content. Status: ${resp.status}</p>`;
        return;
      }
      const data = await resp.json();
      title = data.title || hint;
      meta = data.meta ?? null;
      template = data.template || '';
      templateData = template === 'video' ? parseTemplateData(data.content) : null;
      content = template === 'video' ? '' : data.content || '<p>No content available</p>';
    } catch (err) {
      title = 'Error';
      content = `<p class="text-hister-rose">Failed to parse response: ${err}</p>`;
    }
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content
    escapeKeydownBehavior="ignore"
    class="border-border-brand bg-card-surface max-h-[80vh] max-w-2xl overflow-auto rounded-none border-[3px] p-6 shadow-[6px_6px_0px_var(--hister-indigo)]"
  >
    <Dialog.Header class="border-border-brand-muted border-b-[3px] pb-4">
      <Dialog.Title class="font-outfit text-text-brand text-lg font-bold">
        <a href={url} target="_blank" rel="noopener noreferrer" class="hover:underline">{title}</a>
      </Dialog.Title>
      {#if meta?.author || meta?.published || meta?.type}
        <div class="font-inter text-text-brand-muted mt-1 text-xs">
          {#if meta?.author}<span>{meta.author}</span>{/if}
          {#if meta?.author && meta?.published}<span class="mx-1">·</span>{/if}
          {#if meta?.published}<span>{formatMetaDate(meta.published)}</span>{/if}
          {#if (meta?.author || meta?.published) && meta?.type}<span class="mx-1">·</span>{/if}
          {#if meta?.type}<span class="uppercase">{meta.type}</span>{/if}
        </div>
      {/if}
      {#if meta?.description}
        <p class="font-inter text-text-brand-secondary mt-1 line-clamp-3 text-sm">
          {meta.description}
        </p>
      {/if}
    </Dialog.Header>
    <div
      class="font-inter text-text-brand-secondary prose dark:prose-invert prose-a:text-hister-teal max-w-none text-sm"
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
  </Dialog.Content>
</Dialog.Root>
