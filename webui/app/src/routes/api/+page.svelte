<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '$lib/api';
  import { Badge } from '@hister/components/ui/badge';
  import * as Card from '@hister/components/ui/card';
  import * as Table from '@hister/components/ui/table';

  interface EndpointArg {
    name: string;
    type: string;
    required: boolean;
    description: string;
  }

  interface APIEndpoint {
    name: string;
    path: string;
    method: string;
    csrf_required: boolean;
    description: string;
    args: EndpointArg[] | null;
  }

  let endpoints: APIEndpoint[] = $state([]);
  let loading = $state(true);

  function slugify(name: string, method: string): string {
    return name.toLowerCase().replace(/\s+/g, '_') + '_' + method.toLowerCase();
  }

  onMount(async () => {
    try {
      const res = await apiFetch('/', {
        headers: { 'Accept': 'application/json' }
      });
      if (res.ok) endpoints = await res.json();
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>Hister - API</title>
</svelte:head>

<div class="px-3 md:px-6 py-4 md:py-5 space-y-4 md:space-y-5 overflow-y-auto flex-1">
  <div class="space-y-1">
    <h1 class="flex items-center gap-2"><span class="w-1.5 h-8 bg-hister-teal"></span><span class="font-space text-lg md:text-xl tracking-[1px] font-extrabold text-text-brand uppercase">API Documentation</span></h1>
    <p class="font-inter text-xs md:text-sm text-text-brand-secondary">Available HTTP endpoints for integrating with Hister</p>
  </div>

  {#if loading}
    <p class="font-inter text-sm text-text-brand-muted text-center py-8">Loading endpoints...</p>
  {:else}
    <nav class="flex flex-wrap gap-2">
      {#each endpoints as ep}
        <a
          href="#{slugify(ep.name, ep.method)}"
          class="font-inter text-xs font-semibold text-hister-indigo hover:underline no-underline px-2 py-1 border-[3px] border-brutal-border bg-muted-surface hover:border-hister-indigo shadow-[2px_2px_0_var(--brutal-shadow)] hover:shadow-[1px_1px_0_var(--brutal-shadow)] hover:translate-x-[1px] hover:translate-y-[1px] transition-all"
        >
          {ep.name}
        </a>
      {/each}
    </nav>

    <div class="space-y-4">
      {#each endpoints as ep}
        <Card.Root id={slugify(ep.name, ep.method)} class="bg-card-surface border-[3px] border-brutal-border rounded-none py-0 gap-0 overflow-hidden shadow-[4px_4px_0_var(--brutal-shadow)]">
          <Card.Header class="flex-col md:flex-row md:items-center justify-between px-4 py-3 gap-2">
            <div class="space-y-1">
              <Card.Title class="font-outfit text-base font-extrabold text-text-brand">{ep.name}</Card.Title>
              <div class="flex items-center gap-2 flex-wrap">
                <Badge
                  variant="default"
                  class="text-xs font-bold px-2.5 py-0.5 border-0 {ep.method === 'GET' ? 'bg-hister-teal text-white' : 'bg-hister-coral text-white'}"
                >
                  {ep.method}
                </Badge>
                <code class="font-fira text-sm text-text-brand-secondary">{ep.path}</code>
                {#if ep.csrf_required}
                  <Badge variant="outline" class="text-xs font-semibold border-[2px] border-hister-amber text-hister-amber px-2 py-0.5">
                    CSRF
                  </Badge>
                {/if}
              </div>
            </div>
          </Card.Header>

          <Card.Content class="px-4 py-3 border-t-[3px] border-brutal-border">
            <p class="font-inter text-sm text-text-brand-secondary">{ep.description}</p>

            {#if ep.args && ep.args.length > 0}
              <div class="mt-3">
                <h4 class="font-outfit text-sm font-bold text-text-brand mb-2">Arguments</h4>
                <div class="hidden md:block">
                  <Table.Root>
                    <Table.Header>
                      <Table.Row class="bg-muted-surface border-b-[3px] border-brutal-border hover:bg-muted-surface">
                        <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-3 py-2 h-auto">Name</Table.Head>
                        <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-3 py-2 h-auto">Type</Table.Head>
                        <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-3 py-2 h-auto">Required</Table.Head>
                        <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-3 py-2 h-auto">Description</Table.Head>
                      </Table.Row>
                    </Table.Header>
                    <Table.Body>
                      {#each ep.args as arg}
                        <Table.Row class="border-b-[3px] border-brutal-border">
                          <Table.Cell class="font-fira text-sm font-semibold text-text-brand px-3 py-2"><code>{arg.name}</code></Table.Cell>
                          <Table.Cell class="font-fira text-sm text-text-brand-secondary px-3 py-2"><code>{arg.type}</code></Table.Cell>
                          <Table.Cell class="px-3 py-2">
                            <Badge variant="default" class="text-xs px-2 py-0 border-0 {arg.required ? 'bg-hister-rose text-white' : 'bg-muted-surface text-text-brand-muted'}">
                              {arg.required ? 'Yes' : 'No'}
                            </Badge>
                          </Table.Cell>
                          <Table.Cell class="font-inter text-sm text-text-brand-secondary px-3 py-2">{arg.description}</Table.Cell>
                        </Table.Row>
                      {/each}
                    </Table.Body>
                  </Table.Root>
                </div>
                <div class="md:hidden divide-y-[3px] divide-brutal-border">
                  {#each ep.args as arg}
                    <div class="py-2 space-y-1">
                      <div class="flex items-center gap-2">
                        <code class="font-fira text-sm font-semibold text-text-brand">{arg.name}</code>
                        <code class="font-fira text-xs text-text-brand-muted">{arg.type}</code>
                        {#if arg.required}
                          <Badge variant="default" class="text-xs px-1.5 py-0 border-0 bg-hister-rose text-white">required</Badge>
                        {/if}
                      </div>
                      <p class="font-inter text-xs text-text-brand-secondary">{arg.description}</p>
                    </div>
                  {/each}
                </div>
              </div>
            {:else}
              <p class="mt-3 font-inter text-xs text-text-brand-muted">No arguments for this endpoint</p>
            {/if}
          </Card.Content>
        </Card.Root>
      {/each}
    </div>
  {/if}
</div>
