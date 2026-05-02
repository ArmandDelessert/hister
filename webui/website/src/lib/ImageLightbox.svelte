<script lang="ts">
  import X from '@lucide/svelte/icons/x';

  let openSrc = $state<string | null>(null);
  let openAlt = $state('');

  function close() {
    openSrc = null;
  }

  $effect(() => {
    function handleClick(e: MouseEvent) {
      const target = e.target as Element;
      if (
        target.tagName === 'IMG' &&
        target.closest('.content') &&
        target.parentElement?.tagName !== 'A'
      ) {
        const img = target as HTMLImageElement;
        openSrc = img.src;
        openAlt = img.alt;
      }
    }
    document.addEventListener('click', handleClick);
    return () => document.removeEventListener('click', handleClick);
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if openSrc}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 md:p-10"
    onclick={close}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') close(); }}
  >
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Image preview"
      tabindex="-1"
      class="relative flex max-h-full max-w-full items-center justify-center"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <img
        src={openSrc}
        alt={openAlt}
        class="block max-h-[120vh] max-w-[120vw] border-[3px] border-white object-contain shadow-[8px_8px_0_rgba(255,255,255,0.15)]"
      />
      <button
        onclick={close}
        aria-label="Close image preview"
        class="absolute -top-3 -right-3 flex h-8 w-8 items-center justify-center border-[3px] border-white bg-white text-black shadow-[3px_3px_0_rgba(255,255,255,0.2)] transition-colors hover:bg-gray-100"
      >
        <X size={14} />
      </button>
    </div>
  </div>
{/if}
