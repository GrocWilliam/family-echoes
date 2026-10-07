<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { authStore } from '$lib/authentication';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AppHeader from '$lib/components/AppHeader.svelte';

	let { children } = $props();

	// /auth/* routes are reachable while signed out — in particular
	// /auth/callback, which starts signed out and must be left alone to finish
	// completeSignInFromLink() instead of being bounced to the login page.
	$effect(() => {
		if (!$authStore.isReady) return;

		const onAuthRoute = page.route.id?.startsWith('/auth/') ?? false;
		if (!$authStore.isAuth && !onAuthRoute) {
			goto(resolve('/auth/login'));
		} else if ($authStore.isAuth && page.route.id === '/auth/login') {
			goto(resolve('/'));
		}
	});
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<div class="min-h-dvh bg-paper text-ink">
	<AppHeader />
	<main>{@render children()}</main>
</div>
