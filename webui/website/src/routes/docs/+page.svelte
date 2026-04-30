<script lang="ts">
  import Globe from '@lucide/svelte/icons/globe';
  import * as Card from '@hister/components/ui/card';

  let { data } = $props();
</script>

<svelte:head>
  <title>Documentation | Hister</title>
</svelte:head>

<section class="mx-auto max-w-4xl px-6 py-12 md:px-12">
  <h1
    class="font-space mb-10 text-4xl font-black tracking-[-1px] text-(--text-primary) uppercase md:text-5xl"
  >
    Documentation
  </h1>

  {#each data.categories as category}
    <div class="mb-8">
      <div class="mb-3 flex items-center gap-2">
        <div class="h-2.5 w-2.5 bg-hister-{category.color}"></div>
        <span
          class="font-space text-xs font-bold tracking-[2px] text-[var(--text-secondary)] uppercase"
        >
          {category.name}
        </span>
      </div>

      <ul class="m-0 flex list-none flex-col gap-3 p-0">
        {#each category.docs as doc}
          <li>
            <Card.Root href="/docs/{doc.slug}" class="bg-brutal-card p-5">
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
    <div class="mb-3 flex items-center gap-2">
      <div class="bg-hister-indigo h-2.5 w-2.5"></div>
      <span class="font-space text-xs font-bold tracking-[2px] text-(--text-secondary) uppercase">
        Browser Extensions
      </span>
    </div>

    <ul class="m-0 flex list-none flex-col gap-3 p-0">
      <li>
        <Card.Root
          href="https://chromewebstore.google.com/detail/hister/cciilamhchpmbdnniabclekddabkifhb"
          target="_blank"
          rel="noopener noreferrer"
          class="bg-brutal-card flex-row items-center gap-4 p-5"
        >
          <Globe size={20} class="shrink-0 text-(--text-secondary)" />
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
          class="bg-brutal-card flex-row items-center gap-4 p-5"
        >
          <Globe size={20} class="shrink-0 text-(--text-secondary)" />
          <h2 class="font-space text-lg font-extrabold tracking-[0.5px] text-(--text-primary)">
            Firefox Add-on
          </h2>
        </Card.Root>
      </li>
    </ul>
  </div>
</section>
