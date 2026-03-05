<script lang="ts">
	import type { HTMLAttributes } from "svelte/elements";
	import { cn, type WithElementRef } from "@hister/components/utils";

	let {
		ref = $bindable(null),
		color,
		href,
		target,
		rel,
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> & {
		color?: string;
		href?: string;
		target?: string;
		rel?: string;
	} = $props();
</script>

<svelte:element
	this={href ? 'a' : 'div'}
	bind:this={ref}
	data-slot="card"
	{href}
	{target}
	{rel}
	class={cn(
		"bg-card-surface text-card-foreground border-[3px] border-brutal-border rounded-none py-0 gap-0 overflow-hidden flex flex-col shadow-[6px_6px_0_var(--brutal-shadow)]",
		href && "brutal-press-card no-underline block",
		className
	)}
	style={color ? `border-color: var(--${color}); box-shadow: 6px 6px 0 var(--${color});` : undefined}
	{...restProps}
>
	{@render children?.()}
</svelte:element>
