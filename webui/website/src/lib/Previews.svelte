<script lang="ts">
  import { BrowserFrame } from '@hister/components';
</script>

<section class="w-full bg-brutal-bg px-6 md:px-12 py-12 md:py-16 flex flex-col items-center gap-12">
  <!-- Web UI -->
  <figure class="flex flex-col items-center gap-6 w-full max-w-[1100px] m-0">
    <figcaption class="flex items-center gap-4">
      <span class="w-1.5 h-10 bg-hister-cyan" aria-hidden="true"></span>
      <h2 class="font-space text-3xl md:text-5xl font-black tracking-[3px] text-[var(--text-primary)]">WEB UI</h2>
    </figcaption>
    <BrowserFrame>
      <img src="/landing_screenshot.png" alt="Hister Web UI Landing Page" class="w-full h-auto block" />
    </BrowserFrame>
  </figure>
  <figure class="flex flex-col items-center gap-6 w-full max-w-[1100px] m-0">
    <BrowserFrame>
      <img src="/screenshot.png" alt="Hister Web UI" class="w-full h-auto block" />
    </BrowserFrame>
  </figure>

  <!-- Terminal UI -->
  <figure class="flex flex-col items-center gap-6 w-full max-w-[1100px] m-0">
    <figcaption class="flex items-center gap-4">
      <span class="w-1.5 h-10 bg-hister-lime" aria-hidden="true"></span>
      <h2 class="font-space text-3xl md:text-5xl font-black tracking-[3px] text-[var(--text-primary)]">TERMINAL UI</h2>
    </figcaption>
    <BrowserFrame bg="#0D1117" tabBarClass="bg-[#161B22] border-b-2 border-[#30363D]" tabBarHeight="h-9">
      <img src="/demo.gif" alt="Hister Terminal UI" class="w-full h-auto block" />
    </BrowserFrame>
  </figure>
</section>
