<script module lang="ts">
	const PREFIX = 'echoes:tip:';

	// Brings back every tip the viewer dismissed (from the help page).
	export function showAllTips() {
		try {
			for (const key of Object.keys(localStorage)) {
				if (key.startsWith(PREFIX)) localStorage.removeItem(key);
			}
		} catch {
			// Storage blocked: tips were never hidden in the first place.
		}
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

	let { id, children }: { id: string; children: Snippet } = $props();

	let dismissed = $state(false);
	let hidden = $derived(dismissed || isHidden(id));

	function isHidden(tipId: string): boolean {
		try {
			return localStorage.getItem(PREFIX + tipId) === 'hidden';
		} catch {
			return false;
		}
	}

	function dismiss() {
		dismissed = true;
		try {
			localStorage.setItem(PREFIX + id, 'hidden');
		} catch {
			// Stays hidden until the page is reloaded.
		}
	}
</script>

{#if !hidden}
	<aside
		class="mb-5 flex gap-3 rounded-xl bg-paper-dark p-4 text-base text-ink ring-1 ring-black/5"
	>
		<span aria-hidden="true" class="text-xl leading-6">💡</span>
		<div class="min-w-0 flex-1">
			<div class="space-y-1.5 leading-snug">{@render children()}</div>
			<button
				type="button"
				onclick={dismiss}
				class="mt-2 -ml-2 min-h-11 rounded-full px-2 text-sm font-medium text-ink/70 underline active:bg-ink/5"
			>
				J'ai compris, masquer ce conseil
			</button>
		</div>
	</aside>
{/if}
