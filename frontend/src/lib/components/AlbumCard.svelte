<script lang="ts">
	import { resolve } from '$app/paths';
	import type { AlbumSummary } from '$lib/types';

	let { album }: { album: AlbumSummary } = $props();
</script>

<a
	href={resolve(`/album/${encodeURIComponent(album.name)}`)}
	class="group relative block overflow-hidden rounded-md bg-paper-dark shadow-md shadow-black/15 ring-1 ring-black/5 transition-transform active:scale-[0.98]"
>
	<div class="absolute inset-y-0 left-0 z-10 w-[6px] bg-ink/80"></div>

	<div class="aspect-square w-full overflow-hidden bg-stone-300 pl-[6px]">
		{#if album.cover_url}
			<img
				src={album.cover_url}
				alt={album.name}
				loading="lazy"
				class="h-full w-full object-cover"
			/>
		{/if}
	</div>
	<div class="pt-2.5 pr-2.5 pb-2.5 pl-[calc(6px+0.625rem)]">
		<p class="truncate font-serif text-base font-semibold text-ink">{album.name}</p>
		<p class="text-xs text-ink/50 italic">{album.count} photo{album.count > 1 ? 's' : ''}</p>
	</div>
</a>
