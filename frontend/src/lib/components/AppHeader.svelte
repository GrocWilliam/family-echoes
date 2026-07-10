<script lang="ts">
	import { resolve } from '$app/paths';
	import { authStore, signOut } from '$lib/authentication';

	async function handleSignOut() {
		await signOut();
	}
</script>

{#if $authStore.isAuth}
	<header
		class="sticky top-0 z-40 border-b border-ink/10 bg-paper/90 pt-[env(safe-area-inset-top)] backdrop-blur"
	>
		<div class="mx-auto flex max-w-2xl items-center justify-between px-4 py-3">
			<a href={resolve('/')} class="font-serif text-xl font-semibold tracking-tight text-ink">
				Echoes
			</a>
			<button
				type="button"
				class="rounded-full px-3 py-1.5 text-sm text-ink/50 active:bg-ink/5"
				onclick={handleSignOut}
			>
				Déconnexion
			</button>
		</div>
	</header>
{/if}
