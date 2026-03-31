<script lang="ts">
  import { BrowserFrame } from '@hister/components';

  import landing_scrnshot from '$lib/assets/landing_screenshot.png';
  import scrnshot from '$lib/assets/screenshot.png';
  import demo from '$lib/assets/demo.gif';
</script>

<section class="bg-brutal-bg flex w-full flex-col items-center gap-12 px-6 py-12 md:px-12 md:py-16">
  <!-- Web UI -->
  <figure class="m-0 flex w-full max-w-[1100px] flex-col items-center gap-6">
    <figcaption class="flex items-center gap-4">
      <span class="bg-hister-cyan h-10 w-1.5" aria-hidden="true"></span>
      <h2
        class="font-space text-3xl font-black tracking-[3px] text-[var(--text-primary)] uppercase md:text-5xl"
      >
        Web UI
      </h2>
    </figcaption>
    <BrowserFrame>
      <img src={landing_scrnshot} alt="Hister Web UI Landing Page" class="block h-auto w-full" />
    </BrowserFrame>
  </figure>
  <figure class="m-0 flex w-full max-w-[1100px] flex-col items-center gap-6">
    <BrowserFrame>
      <img src={scrnshot} alt="Hister Web UI" class="block h-auto w-full" />
    </BrowserFrame>
  </figure>

  <!-- Terminal UI -->
  <figure class="m-0 flex w-full max-w-[1100px] flex-col items-center gap-6">
    <figcaption class="flex items-center gap-4">
      <span class="bg-hister-lime h-10 w-1.5" aria-hidden="true"></span>
      <h2
        class="font-space text-3xl font-black tracking-[3px] text-[var(--text-primary)] uppercase md:text-5xl"
      >
        Terminal UI
      </h2>
    </figcaption>
    <BrowserFrame
      bg="#0D1117"
      tabBarClass="bg-[#161B22] border-b-2 border-[#30363D]"
      tabBarHeight="h-9"
    >
      <img src={demo} alt="Hister Terminal UI" class="block h-auto w-full" />
    </BrowserFrame>
  </figure>
</section>
