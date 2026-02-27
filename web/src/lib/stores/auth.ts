import { writable } from 'svelte/store';
import { env } from '$env/dynamic/public';

const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

export type AuthUser = {
	id: string;
	email: string;
	username: string;
	display_name: string;
	bio: string;
	avatar_url: string;
	created_at: string;
	updated_at: string;
};

export const user = writable<AuthUser | null>(null);
export const authLoading = writable(true);

/** Called once on app mount — checks the cookie-based session */
export async function fetchMe(): Promise<AuthUser | null> {
	try {
		const res = await fetch(`${apiUrl}/v1/auth/me`, { credentials: 'include' });
		if (res.status === 204) {
			user.set(null);
			return null;
		}
		if (!res.ok) {
			user.set(null);
			return null;
		}
		const data: AuthUser = await res.json();
		user.set(data);
		return data;
	} catch {
		user.set(null);
		return null;
	} finally {
		authLoading.set(false);
	}
}

export async function login(email: string, password: string): Promise<AuthUser> {
	const res = await fetch(`${apiUrl}/v1/auth/login`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ email, password })
	});
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new Error(body?.error || 'Invalid email or password');
	}
	const data = await res.json();
	user.set(data.user);
	return data.user;
}

export async function signup(name: string, email: string, password: string): Promise<AuthUser> {
	const res = await fetch(`${apiUrl}/v1/auth/signup`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		credentials: 'include',
		body: JSON.stringify({ name, email, password })
	});
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new Error(body?.error || 'Could not create account');
	}
	const data = await res.json();
	user.set(data.user);
	return data.user;
}

export async function logout() {
	try {
		await fetch(`${apiUrl}/v1/auth/logout`, {
			method: 'POST',
			credentials: 'include'
		});
	} catch {
		// best-effort — clear UI state regardless
	}
	user.set(null);
}
