<script lang="ts">
	import { requestMagicLink } from '$lib/authentication';

	let email = $state('');
	let status = $state<'idle' | 'sending' | 'sent' | 'error'>('idle');
	let errorMessage = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		status = 'sending';
		errorMessage = '';

		try {
			await requestMagicLink(email);
			status = 'sent';
		} catch (err) {
			status = 'error';
			errorMessage = err instanceof Error ? err.message : 'Une erreur est survenue.';
		}
	}
</script>

<div class="flex min-h-dvh flex-col items-center justify-center px-6 py-12">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<p class="text-4xl">📷</p>
			<h1 class="mt-3 font-serif text-3xl font-semibold text-ink">Echoes</h1>
			<p class="mt-1 font-serif text-sm text-ink/50 italic">L'album photo de la famille</p>
		</div>

		{#if status === 'sent'}
			<div class="rounded-md bg-paper-dark p-5 text-center shadow-sm ring-1 ring-black/5">
				<p class="text-ink">Un lien de connexion a été envoyé à <strong>{email}</strong>.</p>
				<p class="mt-1 text-sm text-ink/50">Ouvrez-le depuis cet appareil pour vous connecter.</p>
			</div>
		{:else}
			<form onsubmit={handleSubmit} class="space-y-3">
				<input
					type="email"
					name="email"
					bind:value={email}
					required
					autocomplete="email"
					inputmode="email"
					placeholder="votre@email.com"
					class="w-full rounded-xl border border-ink/15 bg-paper px-4 py-3 text-base text-ink outline-none focus:border-ink/40"
				/>
				<button
					type="submit"
					disabled={status === 'sending'}
					class="w-full rounded-xl bg-ink px-4 py-3 text-base font-medium text-paper active:bg-ink/80 disabled:opacity-50"
				>
					{status === 'sending' ? 'Envoi…' : 'Recevoir le lien de connexion'}
				</button>
			</form>
			{#if status === 'error'}
				<p role="alert" class="mt-3 text-center text-sm text-red-700">{errorMessage}</p>
			{/if}
		{/if}
	</div>
</div>
