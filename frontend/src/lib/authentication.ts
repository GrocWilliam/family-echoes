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
export async function requestMagicLink(email: string): Promise<void> {
	const response = await fetch(`${BACKEND_URL}/api/auth/magic-link`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});

	if (!response.ok) {
		const body = await response.json().catch(() => ({}));
		throw new Error(body.error ?? "Cet email n'est pas invité à l'album de famille.");
	}

	await sendSignInLinkToEmail(auth, email, {
		url: `${window.location.origin}/auth/callback`,
		handleCodeInApp: true
	});

	window.localStorage.setItem(EMAIL_STORAGE_KEY, email);
}

// À appeler sur la page de callback pour terminer la connexion.
export async function completeSignInFromLink(url: string): Promise<void> {
	if (!isSignInWithEmailLink(auth, url)) {
		throw new Error('Ce lien de connexion est invalide ou a expiré.');
	}

	let email = window.localStorage.getItem(EMAIL_STORAGE_KEY);
	if (!email) {
		email = window.prompt('Merci de confirmer votre email pour terminer la connexion :');
	}
	if (!email) {
		throw new Error('Email requis pour terminer la connexion.');
	}

	await signInWithEmailLink(auth, email, url);
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
