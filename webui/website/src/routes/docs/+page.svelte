<script lang="ts">
  import Chrome from '@lucide/svelte/icons/chrome';
  import Globe from '@lucide/svelte/icons/globe';
  import * as Card from '@hister/components/ui/card';

  let { data } = $props();

  const categoryColors: Record<string, string> = {
    'Getting Started': 'bg-hister-indigo',
    Reference: 'bg-hister-teal',
    Deployment: 'bg-hister-coral'
  };
</script>

<svelte:head>
  <title>Documentation | Hister</title>
</svelte:head>

<section class="max-w-4xl mx-auto px-6 md:px-12 py-12">
  <h1 class="font-space text-4xl md:text-5xl font-black tracking-[-1px] uppercase text-(--text-primary) mb-10">
    Documentation
  </h1>

  {#each data.categories as category}
    <div class="mb-8">
      <div class="flex items-center gap-2 mb-3">
        <div class="w-2.5 h-2.5 {categoryColors[category.name] ?? 'bg-brutal-border'}"></div>
        <span class="font-space text-xs font-bold tracking-[2px] uppercase text-(--text-secondary)">
          {category.name}
        </span>
      </div>

      <ul class="flex flex-col gap-3 list-none m-0 p-0">
        {#each category.docs as doc}
          <li>
            <Card.Root href="/docs/{doc.slug}/" class="p-5 bg-brutal-card">
              <h2 class="font-space text-lg font-extrabold tracking-[0.5px] text-(--text-primary)">
                {doc.title}
              </h2>
            </Card.Root>
          </li>
        {/each}
      </ul>
    </div>
  {/each}

  <div class="mb-8">
    <div class="flex items-center gap-2 mb-3">
      <div class="w-2.5 h-2.5 bg-hister-indigo"></div>
      <span class="font-space text-xs font-bold tracking-[2px] uppercase text-(--text-secondary)">
        Browser Extensions
      </span>
    </div>

    <ul class="flex flex-col gap-3 list-none m-0 p-0">
      <li>
        <Card.Root
          href="https://chromewebstore.google.com/detail/hister/cciilamhchpmbdnniabclekddabkifhb"
          target="_blank"
          rel="noopener noreferrer"
          class="p-5 bg-brutal-card flex-row items-center gap-4"
        >
          <Chrome size={20} class="text-(--text-secondary) shrink-0" />
          <h2 class="font-space text-lg font-extrabold tracking-[0.5px] text-(--text-primary)">
            Chrome Extension
          </h2>
        </Card.Root>
      </li>
      <li>
        <Card.Root
          href="https://addons.mozilla.org/en-US/firefox/addon/hister/"
          target="_blank"
          rel="noopener noreferrer"
          class="p-5 bg-brutal-card flex-row items-center gap-4"
        >
          <Globe size={20} class="text-(--text-secondary) shrink-0" />
          <h2 class="font-space text-lg font-extrabold tracking-[0.5px] text-(--text-primary)">
            Firefox Add-on
          </h2>
        </Card.Root>
      </li>
    </ul>
  </div>
</section>
