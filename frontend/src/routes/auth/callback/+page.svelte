<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { completeSignInFromLink } from '$lib/authentication';

	let status = $state<'pending' | 'error'>('pending');
	let errorMessage = $state('');

	onMount(async () => {
		try {
			await completeSignInFromLink(window.location.href);
			await goto(resolve('/'));
		} catch (err) {
			status = 'error';
			errorMessage = err instanceof Error ? err.message : 'Une erreur est survenue.';
		}
	});
</script>

<div class="flex min-h-dvh flex-col items-center justify-center px-6 text-center">
	{#if status === 'pending'}
		<div class="h-8 w-8 animate-spin rounded-full border-2 border-ink/20 border-t-ink"></div>
		<p class="mt-4 font-serif text-ink/70 italic">Connexion en cours…</p>
	{:else}
		<p role="alert" class="text-red-700">{errorMessage}</p>
		<a href={resolve('/auth/login')} class="mt-4 text-sm font-medium text-ink underline">
			Réessayer
		</a>
	{/if}
</div>
