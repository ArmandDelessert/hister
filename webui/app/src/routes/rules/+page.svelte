<script lang="ts">
   import { onMount } from 'svelte';
   import { fetchConfig, apiFetch } from '$lib/api';
   import { Button } from '@hister/components/ui/button';
   import { Input } from '@hister/components/ui/input';
   import { Badge } from '@hister/components/ui/badge';
   import { Label } from '@hister/components/ui/label';
   import * as Card from '@hister/components/ui/card';
   import * as Alert from '@hister/components/ui/alert';
   import * as Table from '@hister/components/ui/table';
   import { Shield, Link2, Plus, Trash2, AlertCircle, CheckCircle } from 'lucide-svelte';
   import FilterBar from '$lib/components/FilterBar.svelte';

  interface RulesData {
    skip: string[];
    priority: string[];
    aliases: Record<string, string>;
  }

  interface RuleRow {
    pattern: string;
    type: 'skip' | 'priority';
  }

  let rules: RulesData = $state({ skip: [], priority: [], aliases: {} });
  let loading = $state(true);
  let saving = $state(false);
  let message = $state('');
  let isError = $state(false);
  let newAliasKeyword = $state('');
  let newAliasValue = $state('');
  let newRulePattern = $state('');
  let newRuleType: 'skip' | 'priority' = $state('skip');

  const ruleRows = $derived.by(() => {
    const rows: RuleRow[] = [];
    for (const p of rules.skip) rows.push({ pattern: p, type: 'skip' });
    for (const p of rules.priority) rows.push({ pattern: p, type: 'priority' });
    return rows;
  });

  onMount(async () => {
    await fetchConfig();
    await loadRules();
  });

  async function loadRules() {
    loading = true;
    try {
      const res = await apiFetch('/rules', { headers: { 'Accept': 'application/json' } });
      if (!res.ok) throw new Error('Failed to load rules');
      rules = await res.json();
    } catch (e) {
      message = String(e);
      isError = true;
    } finally {
      loading = false;
    }
  }

  async function saveRules() {
    if (saving) return;
    saving = true;
    message = '';
    try {
      const formData = new URLSearchParams();
      formData.set('skip', rules.skip.join('\n'));
      formData.set('priority', rules.priority.join('\n'));
      const res = await apiFetch('/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: formData.toString()
      });
      if (!res.ok) throw new Error('Failed to save rules');
      message = 'Rules saved successfully';
      isError = false;
      await loadRules();
    } catch (e) {
      message = String(e);
      isError = true;
    } finally {
      saving = false;
    }
  }

  function removeRule(pattern: string, type: 'skip' | 'priority') {
    if (type === 'skip') {
      rules.skip = rules.skip.filter(p => p !== pattern);
    } else {
      rules.priority = rules.priority.filter(p => p !== pattern);
    }
    saveRules();
  }

  function addRule() {
    if (!newRulePattern.trim()) return;
    if (newRuleType === 'skip') {
      rules.skip = [...rules.skip, newRulePattern.trim()];
    } else {
      rules.priority = [...rules.priority, newRulePattern.trim()];
    }
    newRulePattern = '';
    saveRules();
  }

  async function deleteAlias(keyword: string) {
    const formData = new URLSearchParams({ alias: keyword });
    const res = await apiFetch('/delete_alias', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: formData.toString()
    });
    if (res.ok) await loadRules();
  }

  async function addAlias(e: Event) {
    e.preventDefault();
    if (!newAliasKeyword || !newAliasValue) return;
    const formData = new URLSearchParams({
      'alias-keyword': newAliasKeyword,
      'alias-value': newAliasValue
    });
    const res = await apiFetch('/add_alias', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: formData.toString()
    });
    if (res.ok) {
      newAliasKeyword = '';
      newAliasValue = '';
      await loadRules();
    }
  }
</script>

<svelte:head>
  <title>Hister Beta - Rules</title>
</svelte:head>

<div class="px-3 md:px-6 py-4 md:py-5 space-y-4 md:space-y-5 overflow-y-auto flex-1">
  <!-- Header -->
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-2">
    <div class="space-y-1">
      <h1 class="flex items-center gap-2"><span class="w-1.5 h-8 bg-hister-coral"></span><span class="font-space text-lg md:text-xl tracking-[1px] font-extrabold text-text-brand uppercase">Search Rules & Aliases</span></h1>
      <p class="font-inter text-xs md:text-sm text-text-brand-secondary">Configure how Hister indexes and searches your browsing history</p>
    </div>
    <div class="flex items-center gap-2 md:gap-4">
      <Badge variant="outline" class="border-[3px] border-brutal-border bg-muted-surface text-text-brand-secondary font-inter text-xs font-semibold gap-1.5 px-2 md:px-3 py-1 md:py-1.5">
        <Shield class="size-3.5 text-hister-coral" />
        {ruleRows.length} rules
      </Badge>
      <Badge variant="outline" class="border-[3px] border-brutal-border bg-muted-surface text-text-brand-secondary font-inter text-xs font-semibold gap-1.5 px-2 md:px-3 py-1 md:py-1.5">
        <Link2 class="size-3.5 text-hister-indigo" />
        {Object.keys(rules.aliases).length} aliases
      </Badge>
    </div>
  </div>

  {#if message}
    <Alert.Root class="border-[3px] rounded-none {isError ? 'border-hister-rose bg-hister-rose/10 text-hister-rose' : 'border-hister-teal bg-hister-teal/10 text-hister-teal'}">
      {#if isError}
        <AlertCircle class="size-4" />
      {:else}
        <CheckCircle class="size-4" />
      {/if}
      <Alert.Description class="font-inter text-sm">{message}</Alert.Description>
    </Alert.Root>
  {/if}

  {#if loading}
    <p class="font-inter text-sm text-text-brand-muted text-center py-8">Loading rules...</p>
  {:else}
    <!-- Search Aliases Card -->
    <Card.Root class="bg-card-surface border-[3px] border-brutal-border rounded-none py-0 gap-0 overflow-hidden shadow-[5px_5px_0_var(--brutal-shadow)]">
      <Card.Header class="flex-row items-center justify-between px-4 py-3 bg-hister-indigo gap-2">
        <Card.Title class="font-outfit text-base md:text-lg font-extrabold text-white">Search Aliases</Card.Title>
        <span class="font-inter text-sm md:text-base font-medium text-white/70">{Object.keys(rules.aliases).length} aliases</span>
      </Card.Header>

      <Card.Content class="p-0">
        <!-- Desktop table -->
        <div class="hidden md:block">
          <Table.Root>
            <Table.Header>
              <Table.Row class="bg-muted-surface border-b-[3px] border-brutal-border hover:bg-muted-surface">
                <Table.Head class="font-inter text-xs font-bold text-text-brand-muted w-[120px] px-4 py-2 h-auto">Keyword</Table.Head>
                <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-4 py-2 h-auto">Expands To</Table.Head>
                <Table.Head class="w-8 px-4 py-2 h-auto"></Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each Object.entries(rules.aliases) as [keyword, value]}
                <Table.Row class="border-b-[3px] border-brutal-border">
                  <Table.Cell class="font-fira text-sm font-semibold text-text-brand w-[120px] px-4 py-2.5">{keyword}</Table.Cell>
                  <Table.Cell class="font-fira text-sm text-text-brand-secondary truncate px-4 py-2.5 max-w-0">{value}</Table.Cell>
                  <Table.Cell class="w-8 px-4 py-2.5">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      class="shrink-0 text-text-brand-muted hover:text-hister-rose"
                      onclick={() => deleteAlias(keyword)}
                    >
                      <Trash2 class="size-4" />
                    </Button>
                  </Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        </div>

        <!-- Mobile stacked list -->
        <div class="md:hidden divide-y-[3px] divide-brutal-border">
          {#each Object.entries(rules.aliases) as [keyword, value]}
            <div class="flex items-center gap-2 px-3 py-2.5">
              <div class="flex-1 min-w-0">
                <span class="font-fira text-sm font-semibold text-text-brand">{keyword}</span>
                <span class="font-inter text-xs text-text-brand-muted mx-1">&rarr;</span>
                <span class="font-fira text-sm text-text-brand-secondary truncate block">{value}</span>
              </div>
              <Button
                variant="ghost"
                size="icon-sm"
                class="shrink-0 text-text-brand-muted hover:text-hister-rose"
                onclick={() => deleteAlias(keyword)}
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
          {/each}
        </div>
      </Card.Content>

      <Card.Footer class="px-3 md:px-4 py-2.5 bg-muted-surface">
        <form onsubmit={addAlias} class="flex flex-col md:flex-row items-stretch md:items-center gap-2 md:gap-3 w-full">
          <div class="flex items-center gap-2 md:contents">
            <Input
              type="text"
              bind:value={newAliasKeyword}
              placeholder="keyword..."
              class="w-24 md:w-[120px] h-9 px-3 bg-card-surface border-[3px] border-brutal-border font-fira text-xs text-text-brand shadow-none focus-visible:ring-0 focus-visible:border-hister-indigo"
            />
            <Input
              type="text"
              bind:value={newAliasValue}
              placeholder="expands to..."
              class="flex-1 h-9 px-3 bg-card-surface border-[3px] border-brutal-border font-fira text-xs text-text-brand shadow-none focus-visible:ring-0 focus-visible:border-hister-indigo"
            />
          </div>
          <Button
            type="submit"
            size="sm"
            class="bg-hister-indigo text-white font-inter text-sm font-bold border-0 hover:bg-hister-indigo/90 shadow-none gap-1.5 leading-none"
          >
            <Plus class="size-3.5 shrink-0" />
            <span>Add</span>
          </Button>
        </form>
      </Card.Footer>
    </Card.Root>

    <!-- Indexing Rules Card -->
    <Card.Root class="bg-card-surface border-[3px] border-brutal-border rounded-none py-0 gap-0 overflow-hidden shadow-[5px_5px_0_var(--brutal-shadow)]">
      <Card.Header class="flex-row items-center justify-between px-4 py-3 bg-hister-coral gap-2">
        <Card.Title class="font-outfit text-base md:text-lg font-extrabold text-white">Indexing Rules</Card.Title>
        <span class="font-inter text-sm md:text-base font-medium text-white/70">{ruleRows.length} rules</span>
      </Card.Header>

      <Card.Content class="p-0">
        <!-- Desktop table -->
        <div class="hidden md:block">
          <Table.Root>
            <Table.Header>
              <Table.Row class="bg-muted-surface border-b-[3px] border-brutal-border hover:bg-muted-surface">
                <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-4 py-2 h-auto">Pattern</Table.Head>
                <Table.Head class="font-inter text-xs font-bold text-text-brand-muted px-4 py-2 h-auto w-24">Type</Table.Head>
                <Table.Head class="w-8 px-4 py-2 h-auto"></Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each ruleRows as row}
                <Table.Row class="border-b-[3px] border-brutal-border">
                  <Table.Cell class="font-fira text-sm text-text-brand truncate px-4 py-2.5 max-w-0">{row.pattern}</Table.Cell>
                  <Table.Cell class="px-4 py-2.5 w-24">
                    <Badge
                      variant="default"
                      class="text-xs font-bold px-2.5 py-0.5 border-0 {row.type === 'skip' ? 'bg-hister-rose text-white' : 'bg-hister-teal text-white'}"
                    >
                      {row.type === 'skip' ? 'Skip' : 'Priority'}
                    </Badge>
                  </Table.Cell>
                  <Table.Cell class="w-8 px-4 py-2.5">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      class="shrink-0 text-text-brand-muted hover:text-hister-rose"
                      onclick={() => removeRule(row.pattern, row.type)}
                    >
                      <Trash2 class="size-4" />
                    </Button>
                  </Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        </div>

        <!-- Mobile stacked list -->
        <div class="md:hidden divide-y-[3px] divide-brutal-border">
          {#each ruleRows as row}
            <div class="flex items-center gap-2 px-3 py-2.5">
              <div class="flex-1 min-w-0">
                <span class="font-fira text-sm text-text-brand block truncate">{row.pattern}</span>
              </div>
              <Badge
                variant="default"
                class="text-xs font-bold px-2 py-0.5 border-0 shrink-0 {row.type === 'skip' ? 'bg-hister-rose text-white' : 'bg-hister-teal text-white'}"
              >
                {row.type === 'skip' ? 'Skip' : 'Priority'}
              </Badge>
              <Button
                variant="ghost"
                size="icon-sm"
                class="shrink-0 text-text-brand-muted hover:text-hister-rose"
                onclick={() => removeRule(row.pattern, row.type)}
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
          {/each}
        </div>

        {#if ruleRows.length === 0}
          <p class="px-4 py-4 text-center font-inter text-sm text-text-brand-muted">No rules defined yet.</p>
        {/if}
      </Card.Content>

      <Card.Footer class="px-3 md:px-4 py-2.5 bg-muted-surface">
        <div class="flex flex-col md:flex-row items-stretch md:items-center gap-2 md:gap-3 w-full">
          <div class="flex items-center gap-2 md:contents">
            <Input
              type="text"
              bind:value={newRulePattern}
              placeholder="Enter regex pattern..."
              class="flex-1 h-9 px-3 bg-card-surface border-[3px] border-brutal-border font-fira text-xs text-text-brand shadow-none focus-visible:ring-0 focus-visible:border-hister-coral"
            />
            <select
              bind:value={newRuleType}
              class="h-9 px-3 w-[90px] md:w-[100px] bg-card-surface border-[3px] border-brutal-border font-inter text-xs font-semibold text-text-brand outline-none cursor-pointer appearance-none text-center shrink-0"
            >
              <option value="skip">Skip</option>
              <option value="priority">Priority</option>
            </select>
          </div>
          <Button
            type="button"
            size="sm"
            onclick={addRule}
            class="bg-hister-coral text-white font-inter text-sm font-bold border-0 hover:bg-hister-coral/90 shadow-none gap-1.5 leading-none"
          >
            <Plus class="size-3.5 shrink-0" />
            <span>Add</span>
          </Button>
        </div>
      </Card.Footer>
    </Card.Root>
  {/if}
</div>
