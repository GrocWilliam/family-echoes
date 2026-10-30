<script lang="ts">
	import { resolve } from '$app/paths';
	import { requestMagicLink } from '$lib/authentication';

	let email = $state('');
	let status = $state<'idle' | 'sending' | 'sent' | 'error'>('idle');
	let errorMessage = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		status = 'sending';
		errorMessage = '';

		try {
			await requestMagicLink(email.trim());
			status = 'sent';
		} catch (err) {
			status = 'error';
			errorMessage = err instanceof Error ? err.message : 'Une erreur est survenue.';
		}
	}
</script>

{#snippet step(n: number, title: string, text: string)}
	<li class="flex gap-3">
		<span
			aria-hidden="true"
			class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-ink font-medium text-paper"
		>
			{n}
		</span>
		<div>
			<p class="font-medium text-ink">{title}</p>
			<p class="text-base leading-snug text-ink/70">{text}</p>
		</div>
	</li>
{/snippet}

<div class="flex min-h-dvh flex-col items-center justify-center px-6 py-12">
	<div class="w-full max-w-sm">
		<div class="mb-8 text-center">
			<p class="text-4xl" aria-hidden="true">📷</p>
			<h1 class="mt-3 font-serif text-3xl font-semibold text-ink">Echoes</h1>
			<p class="mt-1 font-serif text-sm text-ink/50 italic">L'album photo de la famille</p>
		</div>

		{#if status === 'sent'}
			<div class="rounded-xl bg-paper-dark p-5 text-center shadow-sm ring-1 ring-black/5">
				<p class="text-3xl" aria-hidden="true">✉️</p>
				<p class="mt-2 text-base text-ink">
					Un lien de connexion a été envoyé à <strong class="break-all">{email}</strong>.
				</p>
			</div>

			<h2 class="mt-8 font-serif text-xl font-semibold text-ink">Et maintenant ?</h2>
			<ol class="mt-4 space-y-4">
				{@render step(
					2,
					'Ouvrez vos emails',
					'De préférence sur ce même téléphone ou ordinateur. Cherchez le message qui contient un lien de connexion.'
				)}
				{@render step(
					3,
					'Touchez le lien dans le message',
					'Echoes s’ouvre et vous êtes connecté.'
				)}
			</ol>

			<div class="mt-8 rounded-xl p-4 ring-1 ring-ink/10">
				<p class="font-medium text-ink">Le message n'arrive pas ?</p>
				<ul class="mt-2 list-disc space-y-1 pl-5 text-base leading-snug text-ink/70">
					<li>Attendez deux ou trois minutes.</li>
					<li>Regardez dans le dossier « Courrier indésirable » ou « Spam ».</li>
					<li>Vérifiez que l'adresse ci-dessus est bien écrite.</li>
				</ul>
				<button
					type="button"
					onclick={() => (status = 'idle')}
					class="mt-3 min-h-11 text-base font-medium text-ink underline"
				>
					Corriger l'adresse ou renvoyer un lien
				</button>
			</div>
		{:else}
			<h2 class="font-serif text-xl font-semibold text-ink">Se connecter en 3 étapes</h2>
			<p class="mt-1 text-base text-ink/70">Pas de mot de passe à retenir.</p>
			<ol class="mt-4 mb-6 space-y-4">
				{@render step(
					1,
					'Écrivez votre adresse email',
					'Celle que vous avez donnée à la famille, puis touchez le bouton.'
				)}
				{@render step(
					2,
					'Ouvrez vos emails',
					'Vous recevrez un message avec un lien de connexion.'
				)}
				{@render step(
					3,
					'Touchez le lien dans le message',
					'Echoes s’ouvre et vous êtes connecté.'
				)}
			</ol>

			<form onsubmit={handleSubmit} class="space-y-3">
				<label for="email" class="block font-medium text-ink">Votre adresse email</label>
				<input
					id="email"
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
					class="min-h-12 w-full rounded-xl bg-ink px-4 py-3 text-base font-medium text-paper active:bg-ink/80 disabled:opacity-50"
				>
					{status === 'sending' ? 'Envoi…' : 'Recevoir le lien de connexion'}
				</button>
			</form>
			{#if status === 'error'}
				<p role="alert" class="mt-3 text-center text-base text-red-700">{errorMessage}</p>
			{/if}
		{/if}

		<a
			href={resolve('/bienvenue')}
			class="mt-10 flex min-h-11 items-center justify-center text-base text-ink/60 underline"
		>
			Qu'est-ce qu'Echoes ? Questions fréquentes
		</a>
	</div>
</div>
