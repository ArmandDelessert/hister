<script lang="ts">
  import * as Card from '@hister/components/ui/card';

  let { data } = $props();
</script>

<svelte:head>
  <title>Posts | Hister</title>
</svelte:head>

<section class="max-w-4xl mx-auto px-6 md:px-12 py-12">
  <h1 class="font-space text-4xl md:text-5xl font-black tracking-[-1px] text-(--text-primary) mb-10">POSTS</h1>

  <ul class="flex flex-col gap-4 list-none m-0 p-0">
    {#each data.posts as post}
      <li>
        <Card.Root href="/posts/{post.slug}/" class="p-6 bg-brutal-card">
          <h2 class="font-space text-xl font-extrabold tracking-[0.5px] text-(--text-primary) mb-2">{post.title}</h2>
          {#if post.date}
            <time class="font-inter text-sm text-(--text-secondary)" datetime={post.date}>
              {new Date(post.date).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}
            </time>
          {/if}
        </Card.Root>
      </li>
    {/each}
  </ul>
</section>
