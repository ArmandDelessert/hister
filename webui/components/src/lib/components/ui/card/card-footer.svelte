<script lang="ts">
	import { cn, type WithElementRef } from "@hister/components/utils";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> = $props();
</script>

<div
	bind:this={ref}
	data-slot="card-footer"
	class={cn("px-4 md:px-5 py-4 md:py-5 bg-muted-surface border-t-[3px] border-brutal-border flex items-center", className)}
	{...restProps}
>
	{@render children?.()}
</div>
