<script lang="ts">
  import { Button } from "@hister/components/ui/button";
  import { Input } from "@hister/components/ui/input";
  import { Label } from "@hister/components/ui/label";
  import { Separator } from "@hister/components/ui/separator";
  import { Switch } from "@hister/components/ui/switch";

  const defaultURL = "http://127.0.0.1:4433/";

  let url = $state(defaultURL);
  let token = $state("");
  let indexingEnabled = $state(true);
  let showTokenInput = $state(false);
  let message = $state("");

  chrome.storage.local.get(["histerURL", "histerToken", "indexingEnabled"], (data) => {
    if (!data["histerURL"]) {
      chrome.storage.local.set({ histerURL: defaultURL });
    }
    url = data["histerURL"] || defaultURL;
    token = data["histerToken"] || "";
    indexingEnabled = data["indexingEnabled"] !== false;
    showTokenInput = !token;
  });

  function save(e: Event) {
    e.preventDefault();

    let verifyURL = url;
    if(!verifyURL.endsWith('/')) {
      verifyURL += '/';
    }

    const headers: HeadersInit = {};
    if (token) {
      headers['X-Access-Token'] = token;
    }

    fetch(verifyURL + "api/config", { headers })
      .then((response) => {
        if (response.status !== 200) {
          if (response.status == 403) {
            message = `Error: Invalid access token`;
            return
          }
          message = `Error: Server returned status ${response.status}`;
          return;
        }
        return response.json().then((data) => {
          chrome.storage.local
            .set({ histerURL: url, histerToken: token, indexingEnabled: indexingEnabled })
            .then(() => {
              message = "Settings saved";
              showTokenInput = !token;
            });
        }).catch(() => {
          message = "Error: Server response is not valid JSON - probably invalid server URL.";
        });
      })
      .catch((err) => {
        message = `Error: ${err.message}`;
      });
  }

  function changeToken() {
    showTokenInput = true;
  }

  function reindex() {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs?.length) return;
      chrome.tabs.sendMessage(tabs[0].id!, { action: "reindex" }, (r) => {
        if (r?.status === "ok" && r.status_code === 201) {
          message = "Reindex successful";
          return;
        }
        message = "Reindex failed";
        if (r?.error) {
          message += ": " + r.error;
        }
        if (r?.status_code === 403) {
          message += ": Unauthorized - invalid access token";
        }
      });
    });
  }
</script>

<main class="w-80 p-4 bg-background text-foreground">
  <h1 class="text-lg font-semibold mb-3">Hister</h1>

  <form onsubmit={save} class="space-y-3">
    <div class="space-y-1">
      <Label for="url">Server URL</Label>
      <Input id="url" type="text" bind:value={url} placeholder="Server URL..." />
    </div>

    {#if showTokenInput}
      <div class="space-y-1">
        <Label for="token">Access token (optional)</Label>
        <Input id="token" type="text" bind:value={token} placeholder="Token..." />
      </div>
    {:else}
      <div class="space-y-1">
        <Label>Access token</Label>
        <Button type="button" variant="outline" onclick={changeToken} class="w-full">
          Change token
        </Button>
      </div>
    {/if}

    <div class="flex items-center justify-between">
      <Label for="indexing">Enable automatic indexing</Label>
      <Switch id="indexing" bind:checked={indexingEnabled} />
    </div>

    <Button type="submit" class="w-full">Save</Button>
  </form>

  <Separator class="my-3" />

  <div class="text-center">
    <Button variant="outline" onclick={reindex} class="w-full">
      Reindex page
    </Button>
  </div>

  {#if message}
    <p class="mt-3 text-sm text-muted-foreground">{message}</p>
  {/if}
</main>
