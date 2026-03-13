<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import Github from '@lucide/svelte/icons/github';
  import { Button } from '@hister/components';
</script>

<section class="bg-brutal-bg flex w-full flex-col items-center gap-8 px-6 py-12 md:py-24">
  <h1
    class="font-outfit max-w-[1000px] text-center text-5xl leading-[0.95] font-black tracking-[-2px] text-[var(--text-primary)] uppercase md:text-7xl lg:text-[100px]"
  >
    Your Own <br /> Search Engine
  </h1>

  <p
    class="font-inter text-center text-base leading-relaxed text-[var(--text-secondary)] md:text-2xl"
  >
    Full-text search across your files, browsing history and beyond.<br />
    Private. Fast. Yours.
  </p>

  <nav class="flex flex-wrap items-center justify-center gap-4">
    <Button
      href="https://demo.hister.org/"
      class="bg-hister-indigo hover:bg-hister-indigo/90 font-space border-brutal-border brutal-press-lg h-auto rounded-none border-[3px] px-10 py-5 text-lg font-bold tracking-[1.5px] text-white no-underline md:px-14"
    >
      TRY THE DEMO
      <ArrowRight size={20} />
    </Button>
    <Button
      href="https://github.com/asciimoo/hister"
      target="_blank"
      rel="noopener noreferrer"
      class="bg-brutal-card hover:bg-brutal-card font-space border-brutal-border brutal-press-lg h-auto rounded-none border-[3px] px-10 py-5 text-lg font-bold tracking-[1.5px] text-[var(--text-primary)] no-underline md:px-14"
    >
      VIEW ON GITHUB
      <Github size={20} />
    </Button>
  </nav>
</section>
