<script lang="ts">
  import ImageLightbox from '$lib/ImageLightbox.svelte';
  import Seo from '$lib/Seo.svelte';

  let { data } = $props();
</script>

<Seo
  title={`${data.meta.title} | Hister`}
  description={(data.meta.description as string) ?? `Read ${data.meta.title} on the Hister blog.`}
  path={`/posts/${data.slug}`}
  type="article"
  image={data.meta.image ? `/posts/${data.meta.image as string}` : undefined}
  imageAlt={data.meta.image ? `${data.meta.title} social image` : undefined}
  publishedTime={data.publishedTime}
  section="Post"
/>

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
