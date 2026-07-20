<script lang="ts">
  import { SITE_URL } from '$lib/config';

  interface Props {
    title: string;
    description: string;
    path: string;
    type?: 'website' | 'article';
    image?: string | null;
    imageAlt?: string;
    publishedTime?: string;
    section?: string;
  }

  let {
    title,
    description,
    path,
    type = 'website',
    image = '/logo.png',
    imageAlt = 'Hister logo',
    publishedTime,
    section,
  }: Props = $props();

  const canonicalUrl = $derived(new URL(path, SITE_URL).href);
  const imageUrl = $derived(image ? new URL(image, SITE_URL).href : null);
  const twitterCard = $derived(!image || image === '/logo.png' ? 'summary' : 'summary_large_image');
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href={canonicalUrl} />

  <meta property="og:site_name" content="Hister" />
  <meta property="og:locale" content="en_US" />
  <meta property="og:title" content={title} />
  <meta property="og:description" content={description} />
  <meta property="og:type" content={type} />
  <meta property="og:url" content={canonicalUrl} />

  <meta name="twitter:card" content={twitterCard} />
  <meta name="twitter:title" content={title} />
  <meta name="twitter:description" content={description} />

  {#if imageUrl}
    <meta property="og:image" content={imageUrl} />
    <meta property="og:image:alt" content={imageAlt} />
    <meta name="twitter:image" content={imageUrl} />
    <meta name="twitter:image:alt" content={imageAlt} />
  {/if}

  {#if type === 'article'}
    {#if publishedTime}
      <meta property="article:published_time" content={publishedTime} />
    {/if}
    {#if section}
      <meta property="article:section" content={section} />
    {/if}
  {/if}
</svelte:head>
