import { getIdToken } from './authentication';

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL ?? 'http://localhost:3000';

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
	const token = await getIdToken();
	const headers = new Headers(init.headers);
	// Leave Content-Type unset for FormData uploads — the browser must set its
	// own multipart boundary, which we'd clobber by forcing application/json.
	if (!(init.body instanceof FormData)) {
		headers.set('Content-Type', 'application/json');
	}
	if (token) headers.set('Authorization', `Bearer ${token}`);

	return fetch(`${BACKEND_URL}${path}`, { ...init, headers });
}
