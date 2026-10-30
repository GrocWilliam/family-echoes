import { writable } from 'svelte/store';
import {
	isSignInWithEmailLink,
	onAuthStateChanged,
	sendSignInLinkToEmail,
	signInWithEmailLink,
	signOut as firebaseSignOut
} from 'firebase/auth';
import { BACKEND_URL } from './config';
import { auth } from './firebase';

export interface User {
	isAuth: boolean;
	email: string;
	// Firebase restores a persisted session asynchronously, so callers must wait
	// for this to be true before treating `isAuth: false` as "logged out" rather
	// than "not checked yet".
	isReady: boolean;
	// Only used to decide whether to show upload UI. The real access control for
	// uploads is enforced server-side (RequireAdmin middleware), not here.
	isAdmin: boolean;
}

const EMAIL_STORAGE_KEY = 'echoes:emailForSignIn';

export const authStore = writable<User>({
	email: '',
	isAuth: false,
	isReady: false,
	isAdmin: false
});

onAuthStateChanged(auth, async (user) => {
	if (!user) {
		authStore.set({ email: '', isAuth: false, isReady: true, isAdmin: false });
		return;
	}

	authStore.set({ email: user.email ?? '', isAuth: true, isReady: true, isAdmin: false });

	const isAdmin = await fetchIsAdmin(await user.getIdToken());
	authStore.update((current) => ({ ...current, isAdmin }));
});

async function fetchIsAdmin(idToken: string): Promise<boolean> {
	try {
		const response = await fetch(`${BACKEND_URL}/api/me`, {
			headers: { Authorization: `Bearer ${idToken}` }
		});
		if (!response.ok) return false;
		const data = await response.json();
		return Boolean(data.is_admin);
	} catch {
		return false;
	}
}

// Demande d'envoi du lien de connexion : le backend vérifie d'abord que
// l'email fait partie de la famille invitée, puis Firebase envoie le lien.
// Les erreurs affichées sont toujours en français et disent quoi faire : les
// messages bruts (backend, Firebase, réseau) sont en anglais et techniques.
export async function requestMagicLink(email: string): Promise<void> {
	const sendFailed =
		"Le lien n'a pas pu être envoyé. Vérifiez votre connexion internet, puis réessayez.";

	let response: Response;
	try {
		response = await fetch(`${BACKEND_URL}/api/auth/magic-link`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email })
		});
	} catch {
		throw new Error(sendFailed);
	}

	if (response.status === 403) {
		throw new Error(
			"Cette adresse n'est pas invitée. Vérifiez qu'elle est bien écrite, ou demandez à un membre de la famille de vous ajouter."
		);
	}
	if (!response.ok) throw new Error(sendFailed);

	try {
		await sendSignInLinkToEmail(auth, email, {
			url: `${window.location.origin}/auth/callback`,
			handleCodeInApp: true
		});
	} catch {
		throw new Error(sendFailed);
	}

	window.localStorage.setItem(EMAIL_STORAGE_KEY, email);
}

// À appeler sur la page de callback pour terminer la connexion.
export async function completeSignInFromLink(url: string): Promise<void> {
	if (!isSignInWithEmailLink(auth, url)) {
		throw new Error("Ce lien de connexion n'est pas valide. Demandez-en un nouveau.");
	}

	// Absent quand le lien est ouvert sur un autre appareil ou navigateur que
	// celui qui l'a demandé.
	let email = window.localStorage.getItem(EMAIL_STORAGE_KEY);
	if (!email) {
		email = window.prompt(
			"Vous avez ouvert le lien sur un autre appareil que celui où vous l'avez demandé. Pour terminer, écrivez votre adresse email :"
		);
	}
	if (!email) {
		throw new Error(
			'Votre adresse email est nécessaire pour terminer la connexion. Ouvrez à nouveau le lien reçu, ou demandez-en un nouveau.'
		);
	}

	try {
		await signInWithEmailLink(auth, email.trim(), url);
	} catch {
		throw new Error(
			"Ce lien ne fonctionne plus : il a déjà servi, il a expiré, ou l'adresse écrite ne correspond pas. Demandez un nouveau lien."
		);
	}
	window.localStorage.removeItem(EMAIL_STORAGE_KEY);
}

export async function getIdToken(): Promise<string | null> {
	// On a fresh page load, Firebase restores the persisted session
	// asynchronously — without this, a request fired right after mount can race
	// ahead of that restoration and see currentUser as null, dropping the
	// Authorization header even though the user is actually signed in.
	await auth.authStateReady();
	return auth.currentUser ? auth.currentUser.getIdToken() : null;
}

export async function signOut(): Promise<void> {
	await firebaseSignOut(auth);
}
