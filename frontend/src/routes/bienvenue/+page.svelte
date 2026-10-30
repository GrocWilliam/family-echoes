<script lang="ts">
	import { resolve } from '$app/paths';
	import { authStore } from '$lib/authentication';
	import { showAllTips } from '$lib/components/Tip.svelte';

	let tipsShown = $state(false);

	function resetTips() {
		showAllTips();
		tipsShown = true;
	}
</script>

{#snippet feature(icon: string, title: string, text: string)}
	<li class="flex gap-4 rounded-xl bg-paper-dark p-4 ring-1 ring-black/5">
		<span aria-hidden="true" class="text-3xl">{icon}</span>
		<div>
			<h3 class="font-serif text-lg font-semibold text-ink">{title}</h3>
			<p class="mt-1 text-base leading-snug text-ink/80">{text}</p>
		</div>
	</li>
{/snippet}

{#snippet question(title: string, answer: string)}
	<details class="group rounded-xl bg-paper-dark ring-1 ring-black/5">
		<summary
			class="flex min-h-12 cursor-pointer list-none items-center justify-between gap-3 px-4 py-3 text-base font-medium text-ink"
		>
			{title}
			<span aria-hidden="true" class="text-ink/50 transition-transform group-open:rotate-45">+</span
			>
		</summary>
		<p class="px-4 pb-4 text-base leading-snug text-ink/80">{answer}</p>
	</details>
{/snippet}

<div class="mx-auto max-w-2xl px-4 pt-10 pb-16">
	<div class="text-center">
		<p class="text-5xl" aria-hidden="true">📷</p>
		<h1 class="mt-3 font-serif text-4xl font-semibold text-ink">Echoes</h1>
		<p class="mt-2 font-serif text-lg text-ink/60 italic">L'album photo privé de la famille</p>
	</div>

	<p class="mt-8 text-lg leading-relaxed text-ink">
		Echoes est un endroit où la famille range et partage ses photos. Seules les personnes invitées
		peuvent les voir : rien n'est public.
	</p>

	{#if $authStore.isAuth}
		<a
			href={resolve('/')}
			class="mt-6 flex min-h-12 w-full items-center justify-center rounded-xl bg-ink px-4 py-3 text-base font-medium text-paper active:bg-ink/80"
		>
			Voir les albums
		</a>
	{:else}
		<a
			href={resolve('/auth/login')}
			class="mt-6 flex min-h-12 w-full items-center justify-center rounded-xl bg-ink px-4 py-3 text-base font-medium text-paper active:bg-ink/80"
		>
			Se connecter
		</a>
	{/if}

	<h2 class="mt-12 font-serif text-2xl font-semibold text-ink">Comment ça marche ?</h2>
	<ul class="mt-4 space-y-3">
		{@render feature(
			'✉️',
			'Se connecter sans mot de passe',
			'Écrivez votre adresse email : vous recevez un message avec un lien. Touchez ce lien, et vous êtes connecté. Rien à retenir !'
		)}
		{@render feature(
			'📚',
			'Parcourir les albums',
			"Sur la page d'accueil, chaque album est une couverture avec son nom. Touchez-en une pour voir toutes ses photos."
		)}
		{@render feature(
			'🔍',
			'Regarder les photos en grand',
			'Touchez une photo pour l’agrandir. Les boutons « Suivant » et « Précédent » passent à la photo d’à côté, la croix ✕ ramène à l’album.'
		)}
		{@render feature(
			'➕',
			'Ajouter des photos',
			'Certains membres de la famille, les administrateurs, peuvent ajouter des photos depuis leur téléphone ou leur ordinateur, renommer les albums et supprimer des photos.'
		)}
	</ul>

	<h2 class="mt-12 font-serif text-2xl font-semibold text-ink">Questions fréquentes</h2>
	<div class="mt-4 space-y-2">
		{@render question(
			'Pourquoi n’y a-t-il pas de mot de passe ?',
			'Pour que ce soit plus simple et plus sûr : c’est votre boîte email qui prouve que c’est bien vous. Il n’y a donc aucun mot de passe à retenir ni à se faire voler.'
		)}
		{@render question(
			'Je ne reçois pas l’email de connexion.',
			'Attendez deux ou trois minutes, puis regardez dans le dossier « Courrier indésirable » ou « Spam ». Vérifiez aussi que l’adresse est bien écrite. Si cela ne marche toujours pas, demandez à un membre de la famille de vérifier que votre adresse est bien invitée.'
		)}
		{@render question(
			'Dois-je me reconnecter à chaque fois ?',
			'Non. Une fois connecté, Echoes se souvient de vous sur ce téléphone ou cet ordinateur. Il faudra seulement vous reconnecter si vous touchez « Déconnexion » ou si vous changez d’appareil.'
		)}
		{@render question(
			'Je ne vois pas le bouton pour ajouter des photos.',
			'Seuls les administrateurs de la famille peuvent ajouter, renommer ou supprimer. Si vous aimeriez le faire, demandez-leur de vous ajouter à la liste.'
		)}
		{@render question(
			'Qui peut voir les photos ?',
			'Uniquement les personnes dont l’adresse email a été invitée par la famille. Les photos ne sont pas visibles sur internet.'
		)}
		{@render question(
			'Comment mettre Echoes sur l’écran d’accueil de mon téléphone ?',
			'Sur iPhone : ouvrez Echoes dans Safari, touchez le bouton de partage (le carré avec une flèche vers le haut), puis « Sur l’écran d’accueil ». Sur Android : dans Chrome, touchez les trois points en haut à droite, puis « Ajouter à l’écran d’accueil ».'
		)}
	</div>

	{#if $authStore.isAuth}
		<div class="mt-12 rounded-xl p-4 text-center ring-1 ring-ink/10">
			<p class="text-base text-ink/80">Vous avez masqué des conseils et souhaitez les revoir ?</p>
			<button
				type="button"
				onclick={resetTips}
				disabled={tipsShown}
				class="mt-3 min-h-11 rounded-full px-4 text-base font-medium text-ink underline active:bg-ink/5 disabled:no-underline disabled:opacity-60"
			>
				{tipsShown ? 'Les conseils sont de nouveau affichés ✓' : 'Réafficher tous les conseils'}
			</button>
		</div>
	{/if}
</div>
