<script lang="ts">
	import type { Photo } from '$lib/types';

	let { photos, onSelect }: { photos: Photo[]; onSelect: (index: number) => void } = $props();
</script>

<div class="grid grid-cols-3 gap-2 sm:gap-3">
	{#each photos as photo, index (photo.id)}
		<button
			type="button"
			class="aspect-square overflow-hidden rounded-[3px] bg-white p-1 shadow-sm shadow-black/15 ring-1 ring-black/5 transition-transform active:scale-[0.96]"
			onclick={() => onSelect(index)}
		>
			<div class="h-full w-full overflow-hidden bg-stone-200">
				<img
					src={photo.thumb_url ?? photo.url}
					alt={photo.file_name}
					loading="lazy"
					decoding="async"
					class="h-full w-full object-cover"
				/>
			</div>
		</button>
	{/each}
</div>
