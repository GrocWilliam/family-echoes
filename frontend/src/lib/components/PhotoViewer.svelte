<script lang="ts">
	import { fade } from 'svelte/transition';
	import type { Photo } from '$lib/types';

	let {
		photos,
		index,
		onClose,
		onNavigate
	}: {
		photos: Photo[];
		index: number;
		onClose: () => void;
		onNavigate: (index: number) => void;
	} = $props();

	let photo = $derived(photos[index]);
	let formattedDate = $derived(
		photo
			? new Intl.DateTimeFormat('fr-FR', { dateStyle: 'long' }).format(new Date(photo.uploaded_at))
			: ''
	);

	function next() {
		if (index < photos.length - 1) onNavigate(index + 1);
	}

	function prev() {
		if (index > 0) onNavigate(index - 1);
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') onClose();
		if (event.key === 'ArrowRight') next();
		if (event.key === 'ArrowLeft') prev();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="fixed inset-0 z-50 flex flex-col bg-[#211a14]/97" role="dialog" aria-modal="true">
	<button
		type="button"
		class="absolute top-[max(0.75rem,env(safe-area-inset-top))] right-3 z-10 flex h-10 w-10 items-center justify-center rounded-full bg-black/40 text-white"
		onclick={onClose}
		aria-label="Fermer"
	>
		✕
	</button>

	<div class="flex flex-1 flex-col items-center justify-center overflow-hidden px-4">
		{#if photo}
			{#key photo.id}
				<div
					in:fade={{ duration: 180 }}
					class="flex max-h-full max-w-full flex-col items-center rounded-sm bg-white p-2 shadow-2xl"
				>
					<img
						src={photo.url}
						alt={photo.file_name}
						class="max-h-[70vh] max-w-full object-contain"
					/>
					<p class="mt-2 max-w-full truncate px-2 font-serif text-sm text-ink/70 italic">
						{formattedDate}
					</p>
				</div>
			{/key}
		{/if}
	</div>

	<div
		class="flex items-center justify-between px-4 pt-2 pb-[max(1rem,env(safe-area-inset-bottom))]"
	>
		<button
			type="button"
			class="rounded-full bg-white/10 px-4 py-2 text-sm text-white disabled:opacity-30"
			onclick={prev}
			disabled={index === 0}
		>
			← Précédent
		</button>
		<p class="text-xs text-white/60">{index + 1} / {photos.length}</p>
		<button
			type="button"
			class="rounded-full bg-white/10 px-4 py-2 text-sm text-white disabled:opacity-30"
			onclick={next}
			disabled={index === photos.length - 1}
		>
			Suivant →
		</button>
	</div>
</div>
