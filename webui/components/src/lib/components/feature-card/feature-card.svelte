<script lang="ts">
  import type { Component } from 'svelte';
  import { cn } from '../../utils.js';

  let {
    icon,
    title,
    description,
    bg = 'bg-brutal-card',
    textColor = 'text-[var(--text-primary)]',
    descColor = 'text-[var(--text-secondary)]',
    iconBg = 'bg-hister-indigo',
    iconColor = 'text-white',
    horizontal = false,
    class: className = '',
  }: {
    icon: Component<{ size?: number; class?: string }>;
    title: string;
    description: string;
    bg?: string;
    textColor?: string;
    descColor?: string;
    iconBg?: string;
    iconColor?: string;
    horizontal?: boolean;
    class?: string;
  } = $props();
</script>

<div class={cn(
  bg, 'p-7 border-[3px] border-brutal-border shadow-brutal-lg',
  horizontal ? 'flex items-center gap-5' : 'flex flex-col gap-4',
  className,
)}>
  {#if icon}
    {@const Icon = icon}
    <div class={cn(iconBg, 'w-12 h-12 flex items-center justify-center', horizontal && 'shrink-0')}>
      <Icon size={24} class={iconColor} />
    </div>
  {/if}
  {#if horizontal}
    <div class="flex flex-col gap-2">
      <h3 class="{textColor} font-space text-xl md:text-3xl font-extrabold tracking-[1px]">{title}</h3>
      <p class="{descColor} font-inter text-sm md:text-lg leading-relaxed">{description}</p>
    </div>
  {:else}
    <h3 class="{textColor} font-space text-xl md:text-3xl font-extrabold tracking-[1px]">{title}</h3>
    <p class="{descColor} font-inter text-sm md:text-lg leading-relaxed">{description}</p>
  {/if}
</div>
