<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { authStore } from '$lib/authentication';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import AppHeader from '$lib/components/AppHeader.svelte';

	let { children } = $props();

	authStore.subscribe((user) => {
		if (user.isReady && !user.isAuth) goto(resolve('/auth/login'));
	});
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>

<div class="min-h-dvh bg-paper text-ink">
	<AppHeader />
	<main>{@render children()}</main>
</div>
