<script lang="ts">
  import { page } from '$app/state';

  let { children, data } = $props();

  const isIndex = $derived(page.url.pathname === '/docs' || page.url.pathname === '/docs/');

  const currentDoc = $derived(
    !isIndex ? data.docs.find((d) => page.url.pathname === `/docs/${d.slug}`) : null,
  );
</script>

{#if isIndex}
  {@render children()}
{:else}
  <!-- Dark header banner -->
  <header class="w-full bg-(--text-primary) px-6 py-10 md:py-14">
    <div class="mx-auto max-w-7xl">
      <nav
        aria-label="Breadcrumb"
        class="font-space mb-4 flex items-center gap-2 text-[11px] font-bold tracking-[2px] text-white/40 uppercase"
      >
        <a
          href="/docs"
          class="font-space text-[11px] font-bold tracking-[2px] text-white/40 no-underline transition-colors hover:text-white/60"
          >Docs</a
        >
        <span aria-hidden="true">/</span>
        <span aria-current="page" class="text-white/70">{currentDoc?.title}</span>
      </nav>
      <h1
        class="font-space text-3xl leading-tight font-black tracking-[-1px] text-white md:text-5xl"
      >
        {currentDoc?.title}
      </h1>
    </div>
  </header>

  <!-- Sidebar + Content -->
  <div class="mx-auto flex max-w-7xl flex-col gap-10 px-6 py-10 md:flex-row md:px-12">
    <aside class="hidden shrink-0 md:block md:w-56">
      <nav aria-label="Documentation" class="flex flex-col gap-5 md:sticky md:top-24">
        {#each data.categories as category}
          <div class="flex flex-col gap-1">
            <div class="mb-1 flex items-center gap-2">
              <div class="h-2 w-2 bg-hister-{category.color}"></div>
              <span
                class="font-space text-[10px] font-bold tracking-[2px] text-(--text-secondary) uppercase"
                >{category.name}</span
              >
            </div>
            {#each category.docs as doc}
              <a
                href="/docs/{doc.slug}"
                aria-current={page.url.pathname === `/docs/${doc.slug}` ? 'page' : undefined}
                class="font-inter border-l-[3px] px-3 py-2 text-sm no-underline transition-colors {page
                  .url.pathname === `/docs/${doc.slug}`
                  ? 'border-hister-indigo bg-hister-indigo/5 font-semibold text-(--text-primary)'
                  : 'hover:border-brutal-border border-transparent text-(--text-secondary) hover:text-(--text-primary)'}"
              >
                {doc.title}
              </a>
            {/each}
          </div>
        {/each}
      </nav>
    </aside>

    <div class="min-w-0 flex-1">
      {@render children()}
    </div>
  </div>
{/if}
