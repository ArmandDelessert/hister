<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchConfig, apiFetch } from '$lib/api';
  import { Input } from '@hister/components/ui/input';
  import { Textarea } from '@hister/components/ui/textarea';
  import { Label } from '@hister/components/ui/label';
  import { Button } from '@hister/components/ui/button';
  import * as Card from '@hister/components/ui/card';
  import * as Alert from '@hister/components/ui/alert';
  import AlertCircle from '@lucide/svelte/icons/circle-alert';
  import CheckCircle from '@lucide/svelte/icons/circle-check';
  import { Save } from 'lucide-svelte';

  let url = $state('');
  let title = $state('');
  let text = $state('');
  let message = $state('');
  let isError = $state(false);
  let submitting = $state(false);

  onMount(async () => {
    await fetchConfig();
  });

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (submitting) return;
    submitting = true;
    message = '';
    try {
      const res = await apiFetch('/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url, title, text })
      });
      if (res.status === 201) {
        message = 'Document added successfully.';
        isError = false;
        url = '';
        title = '';
        text = '';
      } else if (res.status === 406) {
        message = 'URL skipped (matches skip rules or is a local URL).';
        isError = false;
      } else {
        message = 'Failed to add document.';
        isError = true;
      }
    } catch (err) {
      message = String(err);
      isError = true;
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Hister - Add</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center pt-12 px-6 overflow-y-auto">
  <Card.Root color="hister-indigo" class="w-full max-w-160">
    <Card.Header color="hister-indigo" class="justify-between px-7 py-5 gap-2">
      <Card.Title class="font-outfit font-black text-[22px] text-white">Add Entry</Card.Title>
      <Card.Description class="font-inter text-[13px] font-medium text-white/70">Manually index a page</Card.Description>
    </Card.Header>

    <Card.Content class="space-y-6">
      {#if message}
        <Alert.Root variant={isError ? 'error' : 'success'}>
          {#if isError}
            <AlertCircle class="size-4 shrink-0" />
          {:else}
            <CheckCircle class="size-4 shrink-0" />
          {/if}
          <Alert.Description class="font-inter text-sm">{message}</Alert.Description>
        </Alert.Root>
      {/if}

      <form onsubmit={handleSubmit} class="space-y-6">
        <div class="space-y-2">
          <Label class="font-outfit text-sm font-bold text-text-brand">URL</Label>
          <Input
            type="text"
            variant="brutal"
            bind:value={url}
            placeholder="https://..."
            required
            class="border-hister-indigo focus-visible:border-hister-coral"
          />
        </div>

        <div class="space-y-2">
          <Label class="font-outfit text-sm font-bold text-text-brand">Title</Label>
          <Input
            type="text"
            variant="brutal"
            bind:value={title}
            placeholder="Page title..."
            class="border-hister-indigo font-inter focus-visible:border-hister-coral"
          />
        </div>

        <div class="space-y-2">
          <Label class="font-outfit text-sm font-bold text-text-brand">Content</Label>
          <Textarea
            bind:value={text}
            placeholder="Paste or type page content..."
            class="w-full min-h-45 p-4 bg-page-bg border-[3px] border-hister-indigo font-inter text-sm text-text-brand placeholder:text-text-brand-muted rounded-none outline-none focus-visible:ring-0 focus-visible:border-hister-coral transition-colors resize-y"
          />
        </div>

        <Button
          type="submit"
          disabled={submitting}
          size="lg"
          class="w-full h-13 bg-hister-coral shadow-[4px_4px_0px_var(--hister-coral)] text-white font-outfit text-base font-extrabold tracking-[1px] hover:bg-hister-coral/90"
        >
          <Save class="size-5 shrink-0" />
          <span>{submitting ? 'Saving...' : 'Save Entry'}</span>
        </Button>
      </form>
    </Card.Content>
  </Card.Root>
</div>
