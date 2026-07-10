<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { authStore } from '$lib/authentication';
	import type { Photo } from '$lib/types';
	import PhotoGrid from '$lib/components/PhotoGrid.svelte';
	import PhotoViewer from '$lib/components/PhotoViewer.svelte';

	let albumName = $derived(decodeURIComponent(page.params.id ?? ''));
	let photos = $state<Photo[]>([]);
	let status = $state<'loading' | 'ready' | 'error'>('loading');
	let activeIndex = $state<number | null>(null);

	let fileInput: HTMLInputElement | undefined = $state();
	let uploading = $state(false);
	let uploadProgress = $state({ done: 0, total: 0 });
	let uploadError = $state('');

	$effect(() => {
		load(albumName);
	});

	async function load(name: string) {
		if (!name) return;
		status = 'loading';
		try {
			const res = await apiFetch(`/api/albums/${encodeURIComponent(name)}/photos`);
			if (!res.ok) throw new Error();
			const data = await res.json();
			photos = data.photos ?? [];
			status = 'ready';
		} catch {
			status = 'error';
		}
	}

	function pickFiles() {
		fileInput?.click();
	}

	async function handleFiles(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const files = input.files ? Array.from(input.files) : [];
		input.value = '';
		if (files.length === 0) return;

		uploading = true;
		uploadError = '';
		uploadProgress = { done: 0, total: files.length };

		for (const file of files) {
			try {
				const form = new FormData();
				form.append('photo', file);
				const res = await apiFetch(`/api/albums/${encodeURIComponent(albumName)}/photos`, {
					method: 'POST',
					body: form
				});
				if (!res.ok) throw new Error();
			} catch {
				uploadError = "Certaines photos n'ont pas pu être envoyées.";
			}
			uploadProgress = { ...uploadProgress, done: uploadProgress.done + 1 };
		}

		uploading = false;
		await load(albumName);
	}
</script>

<div class="mx-auto max-w-2xl px-3 pt-4 pb-24">
	<a
		href={resolve('/')}
		class="mb-2 -ml-1 inline-flex h-9 w-9 items-center justify-center rounded-full text-ink/60 active:bg-ink/5"
		aria-label="Retour"
	>
		←
	</a>
	<h1 class="truncate px-1 font-serif text-3xl font-semibold text-ink">{albumName}</h1>
	<div class="mt-2 mb-5 h-px w-16 bg-ink/20"></div>

	{#if status === 'loading'}
		<p class="text-ink/50">Chargement…</p>
	{:else if status === 'error'}
		<p class="text-red-700">Impossible de charger les photos.</p>
	{:else if photos.length === 0}
		<p class="text-ink/50">
			{$authStore.isAdmin
				? 'Aucune photo dans cet album. Ajoutez-en avec le bouton +.'
				: 'Aucune photo dans cet album.'}
		</p>
	{:else}
		<PhotoGrid {photos} onSelect={(i) => (activeIndex = i)} />
	{/if}

	{#if uploadError}
		<p role="alert" class="mt-3 text-sm text-red-700">{uploadError}</p>
	{/if}
</div>

{#if $authStore.isAdmin}
	<input
		bind:this={fileInput}
		type="file"
		accept="image/*"
		multiple
		class="hidden"
		onchange={handleFiles}
	/>

	<button
		type="button"
		onclick={pickFiles}
		disabled={uploading}
		class="fixed right-5 bottom-[max(1.25rem,env(safe-area-inset-bottom))] z-30 flex h-14 w-14 items-center justify-center rounded-full bg-ink text-2xl leading-none text-paper shadow-lg active:bg-ink/80 disabled:opacity-50"
		aria-label="Ajouter des photos"
	>
		+
	</button>

	{#if uploading}
		<div
			class="fixed inset-x-0 bottom-24 z-30 mx-auto w-fit rounded-full bg-ink px-4 py-2 text-sm text-paper shadow-lg"
		>
			Envoi {uploadProgress.done}/{uploadProgress.total}…
		</div>
	{/if}
{/if}

{#if activeIndex !== null}
	<PhotoViewer
		{photos}
		index={activeIndex}
		onClose={() => (activeIndex = null)}
		onNavigate={(i) => (activeIndex = i)}
	/>
{/if}
