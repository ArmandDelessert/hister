<script lang="ts">
  import Hero from '$lib/Hero.svelte';
  import HowItWorks from '$lib/HowItWorks.svelte';
  import Previews from '$lib/Previews.svelte';
  import WhyHister from '$lib/WhyHister.svelte';
  import Features from '$lib/Features.svelte';
  import FinalCTA from '$lib/FinalCTA.svelte';
</script>

<svelte:head>
  <title>Hister | Your Own Search Engine</title>
  <meta
    name="description"
    content="Hister is a private, self hosted search engine for the pages you visit and the files you keep. Search from the web, terminal, command line, or MCP."
  />
  <meta property="og:title" content="Hister | Your Own Search Engine" />
  <meta
    property="og:description"
    content="Turn the pages you visit and files you keep into a private search index you control."
  />
  <meta property="og:type" content="website" />
</svelte:head>

<Hero />
<FinalCTA />
<Previews />
<HowItWorks />
<WhyHister />
<Features />
