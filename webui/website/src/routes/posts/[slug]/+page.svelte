<script lang="ts">
  import ImageLightbox from '$lib/ImageLightbox.svelte';

  let { data } = $props();
</script>

<svelte:head>
  <title>{data.meta.title} | Hister</title>
  {#if data.meta.description}
    <meta name="description" content={data.meta.description as string} />
  {/if}
  <meta property="og:title" content={data.meta.title as string} />
  {#if data.meta.description}
    <meta property="og:description" content={data.meta.description as string} />
  {/if}
  <meta property="og:type" content="article" />
  <meta property="article:section" content="Post" />
</svelte:head>

<header class="w-full bg-(--text-primary) px-6 py-12 md:py-16">
  <div class="mx-auto max-w-3xl">
    {#if data.date}
      <time class="font-inter text-sm text-white/40" datetime={data.date}>{data.date}</time>
    {/if}
    <h1
      class="font-space mt-3 text-3xl leading-tight font-black tracking-[-1px] text-white md:text-5xl"
    >
      {data.meta.title}
    </h1>
  </div>
</header>

<article class="mx-auto max-w-3xl px-6 py-12">
  <div class="content">
    <data.content />
  </div>
</article>

<ImageLightbox />
