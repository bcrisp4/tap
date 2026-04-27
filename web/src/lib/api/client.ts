// Typed fetch client for the Tap REST API.
//
// Unwraps the design.md §6 envelope:
//   - list endpoints  → `{ data, pagination }`  via getList()
//   - single objects  → bare object              via getJSON()
//   - error responses → `{ error: { code, message } }` → ApiError
//
// All requests are issued under /api/v1; in dev `vite.config.ts`
// proxies that prefix to the Go server, in production the SvelteKit
// build is served by the Go binary directly.

import type { ListResponse } from './types';

export class ApiError extends Error {
	constructor(
		public readonly status: number,
		public readonly code: string,
		message: string
	) {
		super(message);
		this.name = 'ApiError';
	}
}

const BASE = '/api/v1';

async function parse<T>(res: Response): Promise<T> {
	const text = await res.text();
	const body = text ? JSON.parse(text) : null;
	if (!res.ok) {
		const err = body?.error;
		throw new ApiError(res.status, err?.code ?? 'http_error', err?.message ?? res.statusText);
	}
	return body as T;
}

export async function getJSON<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(BASE + path, {
		...init,
		headers: { Accept: 'application/json', ...(init?.headers ?? {}) }
	});
	return parse<T>(res);
}

export async function getList<T>(path: string): Promise<ListResponse<T>> {
	return await getJSON<ListResponse<T>>(path);
}

export async function postJSON<T>(path: string, body: unknown): Promise<T> {
	return await getJSON<T>(path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function putJSON<T>(path: string, body: unknown): Promise<T> {
	return await getJSON<T>(path, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

export async function deleteResource(path: string): Promise<void> {
	const res = await fetch(BASE + path, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		await parse(res); // throws ApiError
	}
}
