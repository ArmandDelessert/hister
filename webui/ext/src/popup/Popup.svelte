<script lang="ts">
  import { Button } from '@hister/components/ui/button';
  import { Label } from '@hister/components/ui/label';
  import { Switch } from '@hister/components/ui/switch';
  import * as Card from '@hister/components/ui/card';
  import SettingsInput from '../options/SettingsInput.svelte';

  const defaultURL = 'http://127.0.0.1:4433/';

  let url = $state(defaultURL);
  let token = $state('');
  let indexingEnabled = $state(true);
  let showTokenInput = $state(false);
  let message = $state('');
  let messageType: 'success' | 'error' = $state('success');

  chrome.storage.local.get(['histerURL', 'histerToken', 'indexingEnabled'], (data) => {
    if (!data['histerURL']) {
      chrome.storage.local.set({ histerURL: defaultURL });
    }
    url = data['histerURL'] || defaultURL;
    token = data['histerToken'] || '';
    indexingEnabled = data['indexingEnabled'] !== false;
    showTokenInput = !token;
  });

  function save(e: Event) {
    e.preventDefault();

    let verifyURL = url;
    if (!verifyURL.endsWith('/')) {
      verifyURL += '/';
    }

    const headers: HeadersInit = {};
    if (token) {
      headers['X-Access-Token'] = token;
    }

    fetch(verifyURL + 'api/config', { headers })
      .then((response) => {
        if (response.status !== 200) {
          if (response.status == 403) {
            message = 'Error: Invalid access token';
            messageType = 'error';
            return;
          }
          message = `Error: Server returned status ${response.status}`;
          messageType = 'error';
          return;
        }
        return response
          .json()
          .then(() => {
            chrome.storage.local
              .set({ histerURL: url, histerToken: token, indexingEnabled: indexingEnabled })
              .then(() => {
                message = 'Settings saved';
                messageType = 'success';
                showTokenInput = !token;
              });
          })
          .catch(() => {
            message = 'Error: Server response is not valid JSON - probably invalid server URL.';
            messageType = 'error';
          });
      })
      .catch((err) => {
        message = `Error: ${err.message}`;
        messageType = 'error';
      });
  }

  function changeToken() {
    showTokenInput = true;
  }

  function reindex() {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs?.length) return;
      chrome.tabs.sendMessage(tabs[0].id!, { action: 'reindex' }, (r) => {
        if (r?.status === 'ok' && r.status_code === 201) {
          message = 'Reindex successful';
          messageType = 'success';
          return;
        }
        message = 'Reindex failed';
        messageType = 'error';
        if (r?.error) {
          message += ': ' + r.error;
        }
        if (r?.status_code === 403) {
          message += ': Unauthorized - invalid access token';
        }
      });
    });
  }
</script>

<main class="w-80">
  <!-- Header bar -->
  <div class="bg-hister-indigo border-brutal-border border-b-[3px] px-5 py-3">
    <span class="font-outfit text-lg font-black tracking-widest text-white uppercase">Hister</span>
  </div>

  <!-- Settings card -->
  <Card.Root
    class="border-brutal-border gap-0 rounded-none border-0 border-b-[3px] py-0 shadow-none"
  >
    <Card.Header class="bg-muted-surface border-brutal-border border-b-[3px] px-5 py-2.5">
      <Card.Title class="font-outfit text-text-brand text-xs font-bold tracking-widest uppercase"
        >Connection</Card.Title
      >
    </Card.Header>
    <Card.Content class="space-y-4 p-5">
      <form onsubmit={save} class="space-y-4">
        <SettingsInput
          label="Server URL"
          bind:value={url}
          placeholder="Server URL..."
        />

        {#if showTokenInput}
          <SettingsInput
            label="Access Token"
            bind:value={token}
            placeholder="Token..."
          />
        {:else}
          <div class="space-y-2">
            <Label class="font-outfit text-text-brand text-sm font-bold">Access Token</Label>
            <Button
              type="button"
              variant="outline"
              onclick={changeToken}
              class="border-brutal-border font-outfit hover:border-hister-indigo h-12 w-full border-[3px] text-sm font-bold tracking-wide transition-all"
            >
              Change token
            </Button>
          </div>
        {/if}

        <div class="flex items-center justify-between pt-1">
          <Label
            for="indexing"
            class="font-outfit text-text-brand cursor-pointer text-xs font-bold"
          >
            Automatic indexing
          </Label>
          <Switch id="indexing" bind:checked={indexingEnabled} />
        </div>

        <Button
          type="submit"
          class="bg-hister-coral border-brutal-border font-outfit h-9 w-full border-[3px] text-sm font-bold tracking-wide text-white shadow-[3px_3px_0_var(--brutal-shadow)] transition-all hover:translate-x-px hover:translate-y-px hover:shadow-[1px_1px_0_var(--brutal-shadow)]"
        >
          Save
        </Button>
      </form>
    </Card.Content>
  </Card.Root>

  <!-- Reindex section -->
  <div class="border-brutal-border border-b-[3px] px-5 py-4">
    <Button
      variant="outline"
      onclick={reindex}
      class="border-brutal-border font-outfit hover:border-hister-indigo h-9 w-full border-[3px] text-sm font-bold tracking-wide transition-all hover:shadow-[3px_3px_0_var(--brutal-shadow)]"
    >
      Reindex Page
    </Button>
  </div>

  <!-- Status message -->
  {#if message}
    <div
      class="font-inter mx-5 my-4 border-l-[4px] px-4 py-3 text-sm {messageType === 'success'
        ? 'border-l-hister-teal bg-hister-teal/10 text-hister-teal'
        : 'border-l-hister-rose bg-hister-rose/10 text-hister-rose'}"
    >
      {message}
    </div>
  {/if}
</main>
