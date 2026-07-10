<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { authStore } from '$lib/authentication';
	import type { AlbumSummary } from '$lib/types';
	import AlbumCard from '$lib/components/AlbumCard.svelte';

	let albums = $state<AlbumSummary[]>([]);
	let status = $state<'loading' | 'ready' | 'error'>('loading');
	let showNewAlbum = $state(false);
	let newAlbumName = $state('');

	onMount(loadAlbums);

	async function loadAlbums() {
		status = 'loading';
		try {
			const res = await apiFetch('/api/albums');
			if (!res.ok) throw new Error();
			const data = await res.json();
			albums = data.albums ?? [];
			status = 'ready';
		} catch {
			status = 'error';
		}
	}

	function createAlbum(event: SubmitEvent) {
		event.preventDefault();
		const name = newAlbumName.trim();
		if (!name) return;
		goto(resolve(`/album/${encodeURIComponent(name)}`));
	}
</script>

<div class="mx-auto max-w-2xl px-4 pt-6 pb-8">
	<div class="mb-1 flex items-start justify-between">
		<div>
			<h1 class="font-serif text-3xl font-semibold text-ink">Nos albums</h1>
			<p class="mt-1 font-serif text-sm text-ink/50 italic">Les souvenirs de la famille</p>
		</div>
		{#if $authStore.isAdmin}
			<button
				type="button"
				class="mt-1 rounded-full bg-ink px-3 py-1.5 text-sm font-medium text-paper active:bg-ink/80"
				onclick={() => (showNewAlbum = !showNewAlbum)}
			>
				+ Nouvel album
			</button>
		{/if}
	</div>

	<div class="mt-4 mb-6 h-px w-full bg-ink/10"></div>

	{#if showNewAlbum && $authStore.isAdmin}
		<form onsubmit={createAlbum} class="mb-6 flex gap-2">
			<input
				type="text"
				bind:value={newAlbumName}
				placeholder="Nom de l'album"
				class="flex-1 rounded-xl border border-ink/15 bg-paper px-3 py-2 text-base text-ink outline-none focus:border-ink/40"
			/>
			<button
				type="submit"
				class="rounded-xl bg-ink px-4 py-2 text-sm font-medium text-paper active:bg-ink/80"
			>
				Créer
			</button>
		</form>
	{/if}

	{#if status === 'loading'}
		<p class="text-ink/50">Chargement…</p>
	{:else if status === 'error'}
		<p class="text-red-700">Impossible de charger les albums.</p>
	{:else if albums.length === 0}
		<p class="text-ink/50">Aucun album pour l'instant.</p>
	{:else}
		<div class="grid grid-cols-2 gap-4 sm:grid-cols-3">
			{#each albums as album (album.name)}
				<AlbumCard {album} />
			{/each}
		</div>
	{/if}
</div>
