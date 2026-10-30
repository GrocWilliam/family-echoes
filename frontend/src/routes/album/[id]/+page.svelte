<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { authStore } from '$lib/authentication';
	import { prepareUpload, UploadError } from '$lib/images';
	import type { Photo } from '$lib/types';
	import PhotoGrid from '$lib/components/PhotoGrid.svelte';
	import PhotoViewer from '$lib/components/PhotoViewer.svelte';
	import Tip from '$lib/components/Tip.svelte';

	// Each upload decodes a full-size photo in memory to make its thumbnail;
	// more than two at once risks running phones out of memory.
	const UPLOAD_CONCURRENCY = 2;

	// SvelteKit already decodes params; decoding again breaks names with "%".
	let albumName = $derived(page.params.id ?? '');
	let photos = $state<Photo[]>([]);
	let status = $state<'loading' | 'ready' | 'error'>('loading');
	let activeIndex = $state<number | null>(null);

	let fileInput: HTMLInputElement | undefined = $state();
	let uploading = $state(false);
	let uploadProgress = $state({ done: 0, total: 0 });
	let uploadFailures = $state<{ name: string; reason: string }[]>([]);

	let renaming = $state(false);
	let renameValue = $state('');
	let renameSaving = $state(false);
	let renameError = $state('');

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

	function reasonForStatus(code: number): string {
		if (code === 413) return 'trop volumineuse (max 20 Mo)';
		if (code === 415) return 'format non pris en charge';
		return "échec de l'envoi";
	}

	async function uploadOne(file: File) {
		try {
			const prepared = await prepareUpload(file);
			const form = new FormData();
			form.append('photo', prepared.photo, prepared.fileName);
			if (prepared.thumbnail) form.append('thumbnail', prepared.thumbnail, 'thumbnail.jpg');

			const res = await apiFetch(`/api/albums/${encodeURIComponent(albumName)}/photos`, {
				method: 'POST',
				body: form
			});
			if (!res.ok) {
				uploadFailures.push({ name: file.name, reason: reasonForStatus(res.status) });
				return;
			}

			// Show each photo as soon as it lands rather than after the batch.
			const photo: Photo = await res.json();
			photos = [...photos, photo].sort((a, b) => a.uploaded_at.localeCompare(b.uploaded_at));
			if (status !== 'ready') status = 'ready';
		} catch (err) {
			const reason = err instanceof UploadError ? err.message : "échec de l'envoi";
			uploadFailures.push({ name: file.name, reason });
		} finally {
			uploadProgress.done += 1;
		}
	}

	async function handleFiles(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const files = input.files ? Array.from(input.files) : [];
		input.value = '';
		if (files.length === 0) return;

		uploading = true;
		uploadFailures = [];
		uploadProgress = { done: 0, total: files.length };

		const queue = [...files];
		const worker = async () => {
			for (let file = queue.shift(); file; file = queue.shift()) {
				await uploadOne(file);
			}
		};
		await Promise.all(Array.from({ length: Math.min(UPLOAD_CONCURRENCY, files.length) }, worker));

		uploading = false;
	}

	async function deletePhoto(photo: Photo) {
		const res = await apiFetch(
			`/api/albums/${encodeURIComponent(albumName)}/photos/${encodeURIComponent(photo.id)}`,
			{ method: 'DELETE' }
		);
		// 404 means it's already gone, which is what the user wanted anyway.
		if (!res.ok && res.status !== 404) throw new Error();

		photos = photos.filter((p) => p.id !== photo.id);
		if (photos.length === 0) {
			activeIndex = null;
		} else if (activeIndex !== null) {
			activeIndex = Math.min(activeIndex, photos.length - 1);
		}
	}

	function startRename() {
		renameValue = albumName;
		renameError = '';
		renaming = true;
	}

	async function submitRename(event: SubmitEvent) {
		event.preventDefault();
		const newName = renameValue.trim();
		if (!newName || newName === albumName) {
			renaming = false;
			return;
		}
		if (/[/\\]/.test(newName)) {
			renameError = 'Le nom ne peut pas contenir « / » ni « \\ ».';
			return;
		}

		renameSaving = true;
		renameError = '';
		try {
			// An album with no photos exists only as a URL, so there's nothing
			// to rename server-side.
			if (photos.length > 0) {
				const res = await apiFetch(`/api/albums/${encodeURIComponent(albumName)}`, {
					method: 'PATCH',
					body: JSON.stringify({ name: newName })
				});
				if (res.status === 409) {
					renameError = 'Un album porte déjà ce nom.';
					return;
				}
				if (!res.ok) throw new Error();
			}

			renaming = false;
			await goto(resolve(`/album/${encodeURIComponent(newName)}`), { replaceState: true });
		} catch {
			renameError = "Impossible de renommer l'album.";
		} finally {
			renameSaving = false;
		}
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

	{#if renaming}
		<form onsubmit={submitRename} class="flex gap-2 px-1">
			<!-- svelte-ignore a11y_autofocus -->
			<input
				type="text"
				bind:value={renameValue}
				maxlength="200"
				autofocus
				aria-label="Nouveau nom de l'album"
				class="min-w-0 flex-1 rounded-xl border border-ink/15 bg-paper px-3 py-2 font-serif text-xl text-ink outline-none focus:border-ink/40"
			/>
			<button
				type="submit"
				disabled={renameSaving}
				class="rounded-xl bg-ink px-4 py-2 text-sm font-medium text-paper active:bg-ink/80 disabled:opacity-50"
			>
				{renameSaving ? '…' : 'OK'}
			</button>
			<button
				type="button"
				onclick={() => (renaming = false)}
				class="rounded-xl px-3 py-2 text-sm text-ink/60 active:bg-ink/5"
			>
				Annuler
			</button>
		</form>
		<p class="mt-2 px-1 text-base text-ink/70">Le nouveau nom sera visible par toute la famille.</p>
		{#if renameError}
			<p role="alert" class="mt-2 px-1 text-sm text-red-700">{renameError}</p>
		{/if}
	{:else}
		<div class="flex items-start justify-between gap-2 px-1">
			<h1 class="min-w-0 truncate font-serif text-3xl font-semibold text-ink">{albumName}</h1>
			{#if $authStore.isAdmin}
				<button
					type="button"
					onclick={startRename}
					class="mt-1 shrink-0 rounded-full px-3 py-1.5 text-sm text-ink/50 active:bg-ink/5"
				>
					Renommer
				</button>
			{/if}
		</div>
	{/if}
	<div class="mt-2 mb-5 h-px w-16 bg-ink/20"></div>

	{#if photos.length > 0}
		<Tip id="album-view">
			<p>Touchez une photo pour la voir en grand.</p>
			<p>
				Ensuite, « Suivant » et « Précédent » passent à la photo d'à côté, et la croix ✕ ramène à
				l'album. La flèche ← en haut ramène à la liste des albums.
			</p>
		</Tip>
	{/if}
	{#if $authStore.isAdmin}
		<Tip id="album-admin">
			<p>
				Pour ajouter des photos, touchez le bouton rond « + » en bas à droite, puis choisissez-les
				dans votre galerie. Vous pouvez en sélectionner plusieurs d'un coup.
			</p>
			<p>Les photos d'iPhone sont acceptées. Chaque photo peut faire jusqu'à 20 Mo.</p>
			<p>
				« Renommer » change le nom de l'album. Pour supprimer une photo, ouvrez-la en grand et
				touchez « Supprimer ».
			</p>
		</Tip>
	{/if}

	{#if status === 'loading'}
		<p class="text-ink/50">Chargement…</p>
	{:else if status === 'error'}
		<p class="text-red-700">
			Impossible de charger les photos. Vérifiez votre connexion internet, puis rechargez la page.
		</p>
	{:else if photos.length === 0}
		<p class="text-base text-ink/60">
			{$authStore.isAdmin
				? "Aucune photo dans cet album pour l'instant. Ajoutez-en avec le bouton rond « + » en bas à droite. Tant qu'il n'a pas de photo, l'album n'est pas gardé."
				: 'Aucune photo dans cet album pour le moment.'}
		</p>
	{:else}
		<PhotoGrid {photos} onSelect={(i) => (activeIndex = i)} />
	{/if}

	{#if uploadFailures.length > 0}
		<div
			role="alert"
			class="mt-4 rounded-md bg-red-50 p-3 text-sm text-red-800 ring-1 ring-red-200"
		>
			<p class="font-medium">
				{uploadFailures.length === 1
					? "1 photo n'a pas pu être envoyée :"
					: `${uploadFailures.length} photos n'ont pas pu être envoyées :`}
			</p>
			<ul class="mt-1 space-y-0.5">
				{#each uploadFailures as failure, i (i)}
					<li class="truncate">{failure.name} — {failure.reason}</li>
				{/each}
			</ul>
		</div>
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
			role="status"
			class="fixed inset-x-4 bottom-24 z-30 mx-auto w-fit rounded-2xl bg-ink px-4 py-2 text-center text-base text-paper shadow-lg"
		>
			Envoi {uploadProgress.done}/{uploadProgress.total}…
			<span class="block text-sm text-paper/70">Gardez cette page ouverte jusqu'à la fin.</span>
		</div>
	{/if}
{/if}

{#if activeIndex !== null}
	<PhotoViewer
		{photos}
		index={activeIndex}
		onClose={() => (activeIndex = null)}
		onNavigate={(i) => (activeIndex = i)}
		onDelete={$authStore.isAdmin ? deletePhoto : undefined}
	/>
{/if}
