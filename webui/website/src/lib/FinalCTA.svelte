<script lang="ts">
  import { onMount } from 'svelte';
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import Bot from '@lucide/svelte/icons/bot';
  import Browser from '@lucide/svelte/icons/app-window';
  import Braces from '@lucide/svelte/icons/curly-braces';
  import Download from '@lucide/svelte/icons/download';
  import ExternalLink from '@lucide/svelte/icons/external-link';
  import Terminal from '@lucide/svelte/icons/terminal';
  import { Button } from '@hister/components';

  const accessMethods = [
    {
      icon: Browser,
      label: 'Web',
      title: 'Web interface',
      description: 'Search, filter results, open stored previews, and manage history in a browser.',
      fact: 'Browser based search and administration',
      color: 'var(--hister-cyan)',
    },
    {
      icon: Terminal,
      label: 'Terminal',
      title: 'Terminal client',
      description: 'Run interactive, keyboard driven searches without leaving the terminal.',
      fact: 'Interactive terminal search',
      color: 'var(--hister-amber)',
    },
    {
      icon: Braces,
      label: 'HTTP API',
      title: 'HTTP API',
      description: 'Search, add, label, delete, and manage indexed documents from other clients.',
      fact: 'Scriptable HTTP endpoints',
      color: 'var(--hister-lime)',
    },
    {
      icon: Bot,
      label: 'MCP',
      title: 'MCP server',
      description: 'Compatible assistants can search the index and retrieve stored previews.',
      fact: 'Search and preview retrieval tools',
      color: 'var(--hister-indigo)',
    },
  ];

  let activeMethod = $state(0);
  let rotationPaused = $state(false);
  let reducedMotion = $state(false);
  let contentVisible = $state(true);
  let currentMethod = $derived(accessMethods[activeMethod]);
  let CurrentMethodIcon = $derived(currentMethod.icon);
  let transitionTimer: ReturnType<typeof setTimeout> | undefined;

  function selectMethod(index: number) {
    if (index === activeMethod) return;

    if (reducedMotion) {
      activeMethod = index;
      return;
    }

    contentVisible = false;
    if (transitionTimer) window.clearTimeout(transitionTimer);
    transitionTimer = window.setTimeout(() => {
      activeMethod = index;
      window.requestAnimationFrame(() => (contentVisible = true));
    }, 180);
  }

  onMount(() => {
    const motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)');
    const updateMotionPreference = () => (reducedMotion = motionPreference.matches);

    updateMotionPreference();
    motionPreference.addEventListener('change', updateMotionPreference);

    const interval = window.setInterval(() => {
      if (!rotationPaused && !reducedMotion) {
        selectMethod((activeMethod + 1) % accessMethods.length);
      }
    }, 4000);

    return () => {
      window.clearInterval(interval);
      if (transitionTimer) window.clearTimeout(transitionTimer);
      motionPreference.removeEventListener('change', updateMotionPreference);
    };
  });
</script>

<section
  class="relative isolate overflow-hidden border-b-[3px] border-brutal-border bg-hister-coral"
>
  <div class="cta-grid" aria-hidden="true"></div>
  <div class="relative z-10 mx-auto w-full max-w-[1500px] px-6 py-16 md:px-12 md:py-24">
    <div
      class="grid min-w-0 gap-14 xl:grid-cols-[minmax(0,1fr)_minmax(420px,0.72fr)] xl:items-stretch xl:gap-16"
    >
      <div class="min-w-0">
        <h2
          class="font-outfit max-w-[880px] text-4xl leading-[0.88] font-black tracking-[-0.05em] text-[var(--text-primary)] uppercase md:text-7xl lg:text-8xl"
        >
          Preserve Knowledge.<br />Find It Again.
        </h2>
        <p class="mt-7 max-w-[650px] text-lg leading-[1.75] text-[#1f1f1f]">
          A local installation uses one binary containing the server and terminal client. The server
          also provides the web interface and HTTP API.
        </p>

        <div class="mt-9 flex flex-col gap-4 sm:flex-row">
          <Button
            href="/docs/installing"
            class="font-space brutal-press-lg h-auto justify-center rounded-none border-[3px] border-brutal-border bg-[var(--text-primary)] px-8 py-4 text-[15px] font-bold tracking-[1.3px] text-white uppercase no-underline"
          >
            Choose how to install
            <ArrowRight size={18} />
          </Button>
          <Button
            href="https://github.com/asciimoo/hister/releases/latest"
            target="_blank"
            rel="noopener noreferrer"
            class="bg-brutal-card font-space brutal-press-lg h-auto justify-center rounded-none border-[3px] border-brutal-border px-8 py-4 text-[15px] font-bold tracking-[1.3px] text-[var(--text-primary)] uppercase no-underline hover:bg-[var(--text-primary)] hover:text-white"
          >
            Download Hister
            <Download size={18} />
          </Button>
        </div>

        <a
          href="https://demo.hister.org/"
          class="font-fira mt-6 flex max-w-full flex-wrap items-center gap-2 text-xs leading-relaxed font-bold text-[#1f1f1f] underline decoration-2 underline-offset-4 hover:no-underline"
        >
          Want to look around first? Try the live demo
          <ExternalLink size={13} />
        </a>
      </div>

      <aside
        class="access-panel flex min-w-0 flex-col border-[3px] border-brutal-border bg-[var(--text-primary)] shadow-[9px_9px_0_var(--brutal-shadow)]"
        class:paused={rotationPaused}
        aria-label="Ways to access a Hister index"
        onmouseenter={() => (rotationPaused = true)}
        onmouseleave={() => (rotationPaused = false)}
        onfocusin={() => (rotationPaused = true)}
        onfocusout={() => (rotationPaused = false)}
      >
        <div
          class="flex items-center justify-between gap-4 border-b-[3px] border-white/20 px-5 py-4"
        >
          <span class="font-fira text-[10px] font-bold tracking-[1.5px] text-white uppercase"
            >Choose your interface</span
          >
          <span class="font-fira text-[9px] tracking-[1px] text-white/50 uppercase"
            >One shared index</span
          >
        </div>

        <div
          class="access-detail relative min-h-[285px] flex-1 overflow-hidden text-[var(--text-primary)]"
          style="--method-color: {currentMethod.color}"
        >
          <div
            class:content-hidden={!contentVisible}
            class="method-content absolute inset-0 flex flex-col justify-center p-7 sm:p-9"
          >
            <div
              class="flex h-14 w-14 items-center justify-center border-[3px] border-brutal-border bg-[var(--method-color)] shadow-brutal"
            >
              <CurrentMethodIcon size={26} strokeWidth={2.3} />
            </div>
            <h3 class="font-space mt-8 text-3xl font-black tracking-[-0.03em] uppercase">
              {currentMethod.title}
            </h3>
            <p class="mt-4 max-w-[430px] text-sm leading-[1.7] text-[var(--text-secondary)]">
              {currentMethod.description}
            </p>
            <div class="font-fira mt-7 flex items-center gap-3 text-[10px] font-semibold">
              <span class="h-2.5 w-2.5 bg-[var(--method-color)]"></span>
              {currentMethod.fact}
            </div>
          </div>
          <div class="absolute inset-x-0 bottom-0 h-1.5 bg-black/10">
            {#key activeMethod}
              <span class="method-progress block h-full bg-[var(--method-color)]"></span>
            {/key}
          </div>
        </div>

        <div class="grid grid-cols-4 border-t-[3px] border-brutal-border">
          {#each accessMethods as method, index}
            {@const MethodIcon = method.icon}
            <button
              type="button"
              class="method-tab relative flex min-w-0 cursor-pointer flex-col items-center gap-2 border-r border-white/20 px-2 py-4 text-white transition-colors last:border-r-0 {index ===
              activeMethod
                ? 'bg-white/15'
                : 'text-white/55 hover:bg-white/10 hover:text-white'}"
              style="--tab-color: {method.color}"
              aria-label="Show {method.title}"
              aria-pressed={index === activeMethod}
              onclick={() => selectMethod(index)}
            >
              <span
                class="absolute inset-x-0 top-0 h-1 bg-[var(--tab-color)] {index === activeMethod
                  ? 'opacity-100'
                  : 'opacity-35'}"
              ></span>
              <MethodIcon size={18} />
              <span class="font-fira truncate text-[8px] font-bold tracking-[0.8px] uppercase">
                {method.label}
              </span>
            </button>
          {/each}
        </div>
      </aside>
    </div>
  </div>
</section>

<style>
  .cta-grid {
    position: absolute;
    inset: 0;
    opacity: 0.14;
    background-image:
      linear-gradient(var(--brutal-border) 2px, transparent 2px),
      linear-gradient(90deg, var(--brutal-border) 2px, transparent 2px);
    background-size: 44px 44px;
    mask-image: linear-gradient(90deg, black, transparent 75%);
  }

  .method-progress {
    transform-origin: left;
    animation: method-progress 4s linear;
  }

  .access-panel.paused .method-progress {
    animation-play-state: paused;
  }

  .access-detail {
    background-color: color-mix(in srgb, var(--method-color) 25%, var(--brutal-card));
    transition: background-color 420ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .method-content {
    opacity: 1;
    translate: 0 0;
    transition:
      opacity 280ms ease,
      translate 360ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .method-content.content-hidden {
    opacity: 0;
    translate: 0 4px;
    transition-duration: 180ms;
  }

  @keyframes method-progress {
    from {
      scale: 0 1;
    }
    to {
      scale: 1 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .access-detail,
    .method-content,
    .method-progress {
      transition: none;
      animation: none;
    }
  }
</style>
