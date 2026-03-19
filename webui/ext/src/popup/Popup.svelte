<script lang="ts">
  import { Button } from '@hister/components/ui/button';
  import { Label } from '@hister/components/ui/label';
  import { Switch } from '@hister/components/ui/switch';
  import * as Card from '@hister/components/ui/card';
  import SettingsInput from '../options/SettingsInput.svelte';
  import { Settings, Sun, Moon } from 'lucide-svelte';
  import { slide } from 'svelte/transition';
  import { ModeWatcher, toggleMode, mode } from 'mode-watcher';

  const defaultURL = 'http://127.0.0.1:4433/';

  let url = $state(defaultURL);
  let token = $state('');
  let customHeaders: { name: string; value: string }[] = $state([]);
  let indexingEnabled = $state(true);
  let showTokenInput = $state(false);
  let message = $state('');
  let messageType: 'success' | 'error' = $state('success');
  let showSettings = $state(false);
  let messageKey = $state(0); // used to reappear message every time it is updated

  function setMessage(mType, msg) {
    message = msg;
    messageType = mType;
    messageKey++;
  }

  function setErrorMessage(msg) {
    setMessage('error', msg);
  }

  function setSuccessMessage(msg) {
    setMessage('success', msg);
  }

  chrome.storage.local.get(
    ['histerURL', 'histerToken', 'histerCustomHeaders', 'indexingEnabled'],
    (data) => {
      if (!data['histerURL']) {
        chrome.storage.local.set({ histerURL: defaultURL });
      }
      url = data['histerURL'] || defaultURL;
      token = data['histerToken'] || '';
      customHeaders = data['histerCustomHeaders'] || [];
      indexingEnabled = data['indexingEnabled'] !== false;
      showTokenInput = !token;

      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        if (!tabs?.length) return;
        chrome.action.getBadgeText({ tabId: tabs[0].id! }, (badgeText) => {
          if (badgeText === '!') {
            setErrorMessage('Failed to send page data to server');
          }
        });
      });
    },
  );

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
    for (const h of customHeaders) {
      if (h.name) {
        headers[h.name] = h.value || '';
      }
    }

    fetch(verifyURL + 'api/config', { headers })
      .then((response) => {
        if (response.status !== 200) {
          if (response.status == 403) {
            setErrorMessage('Invalid access token');
            return;
          }
          setErrorMessage(`Server returned status ${response.status}`);
          return;
        }
        return response
          .json()
          .then(() => {
            chrome.storage.local
              .set({ histerURL: url, histerToken: token, histerCustomHeaders: customHeaders })
              .then(() => {
                setSuccessMessage('Settings saved');
                showTokenInput = !token;

                chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
                  if (tabs?.length) {
                    chrome.action.setBadgeText({ text: '', tabId: tabs[0].id! });
                  }
                });
              });
          })
          .catch(() => {
            setErrorMessage('Server response is not valid JSON - probably invalid server URL.');
          });
      })
      .catch((err) => {
        setErrorMessage(err.message);
      });
  }

  function changeToken() {
    showTokenInput = true;
  }

  function toggleIndexing() {
    chrome.storage.local.set({ indexingEnabled: indexingEnabled });
    setSuccessMessage(`Automatic indexing ${indexingEnabled ? 'enabled' : 'disabled'}`);
  }

  function reindex() {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs?.length) return;
      chrome.tabs.sendMessage(tabs[0].id!, { action: 'reindex' }, (r) => {
        if (r?.status === 'ok' && r.status_code === 201) {
          setSuccessMessage('Reindex successful');
          return;
        }
        let msg = 'Reindex failed';
        if (r?.error) {
          msg += ': ' + r.error;
        }
        if (r?.status_code === 403) {
          msg += ': Unauthorized - invalid access token';
        }
        setErrorMessage(msg);
      });
    });
  }

  function toggleSettings() {
    showSettings = !showSettings;
    message = '';
  }
</script>

<ModeWatcher />

<main class="w-80">
  <!-- Header bar -->
  <div
    class="bg-hister-indigo/90 border-brutal-border flex items-center justify-between border-b-[3px] px-5 py-3"
  >
    <span class="font-outfit text-lg font-black tracking-widest text-white uppercase">Hister</span>
    <div class="flex items-center gap-2">
      <button
        onclick={toggleSettings}
        class="hover:text-hister-coral cursor-pointer border-0 bg-transparent p-0 text-white transition-colors"
        aria-label="Settings"
      >
        <Settings size={20} />
      </button>
    </div>
  </div>

  {#if showSettings}
    <!-- Settings View -->
    <Card.Root
      class="border-brutal-border gap-0 rounded-none border-0 border-b-[3px] py-0 shadow-none"
    >
      <Card.Content class="space-y-4 p-5">
        <form onsubmit={save} class="space-y-4">
          <SettingsInput label="Server URL" bind:value={url} placeholder="Server URL..." />

          {#if showTokenInput}
            <SettingsInput label="Access Token" bind:value={token} placeholder="Token..." />
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

          <Button
            type="submit"
            class="bg-hister-coral border-brutal-border font-outfit h-9 w-full border-[3px] text-sm font-bold tracking-wide text-white shadow-[3px_3px_0_var(--brutal-shadow)] transition-all hover:translate-x-px hover:translate-y-px hover:shadow-[1px_1px_0_var(--brutal-shadow)]"
          >
            Save
          </Button>

          <div class="flex items-center justify-between">
            <Label class="font-outfit text-text-brand text-sm font-bold">Theme</Label>
            <button
              onclick={toggleMode}
              class="border-brutal-border hover:border-hister-indigo flex cursor-pointer items-center gap-2 rounded border-[3px] bg-transparent px-3 py-1.5 transition-all"
              aria-label="Toggle theme"
            >
              {#if mode.current === 'light'}
                <Sun size={16} />
                <span class="font-outfit text-text-brand text-sm font-bold">Light</span>
              {:else}
                <Moon size={16} />
                <span class="font-outfit text-text-brand text-sm font-bold">Dark</span>
              {/if}
            </button>
          </div>
        </form>
      </Card.Content>
    </Card.Root>
  {:else}
    <!-- Main View -->
    <!-- Automatic Indexing Toggle -->
    <div class="border-brutal-border border-b-[3px] px-5 py-4">
      <div class="flex items-center justify-between">
        <Label for="indexing" class="font-outfit text-text-brand cursor-pointer text-sm font-bold">
          Automatic indexing
        </Label>
        <Switch id="indexing" bind:checked={indexingEnabled} onCheckedChange={toggleIndexing} />
      </div>
    </div>

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
  {/if}
  <!-- Status message -->
  {#if message}
    {#key messageKey}
      <div
        transition:slide
        class="font-inter mx-5 my-4 border-l-[4px] px-4 py-3 text-sm {messageType === 'success'
          ? 'border-l-hister-teal bg-hister-teal/10 text-hister-teal'
          : 'border-l-hister-rose bg-hister-rose/10 text-hister-rose'}"
      >
        {message}
      </div>
    {/key}
  {/if}
</main>
