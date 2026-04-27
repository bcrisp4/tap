import { describe, it, expect, vi } from 'vitest';
import { ApiError, getJSON, getList } from './client';

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

describe('client', () => {
	it('throws ApiError on error envelope', async () => {
		vi.stubGlobal('fetch', async () =>
			jsonResponse({ error: { code: 'not_found', message: 'no' } }, 404)
		);
		await expect(getJSON('/feeds/9999')).rejects.toMatchObject({
			name: 'ApiError',
			status: 404,
			code: 'not_found'
		});
	});

	it('returns the parsed body on success', async () => {
		vi.stubGlobal('fetch', async () => jsonResponse({ id: 1, title: 'a' }));
		await expect(getJSON('/feeds/1')).resolves.toEqual({ id: 1, title: 'a' });
	});

	it('passes list envelope through', async () => {
		vi.stubGlobal('fetch', async () =>
			jsonResponse({
				data: [{ id: 1 }, { id: 2 }],
				pagination: { limit: 25, offset: 0, total: 2 }
			})
		);
		const out = await getList<{ id: number }>('/feeds');
		expect(out.data).toHaveLength(2);
		expect(out.pagination.total).toBe(2);
	});

	it('falls back to http_error code when body lacks an error envelope', async () => {
		vi.stubGlobal(
			'fetch',
			async () => new Response('', { status: 502, statusText: 'Bad Gateway' })
		);
		await expect(getJSON('/x')).rejects.toMatchObject({
			name: 'ApiError',
			status: 502,
			code: 'http_error'
		});
	});

	it('surfaces non-JSON error bodies as ApiError, not SyntaxError', async () => {
		// e.g. an upstream nginx returning an HTML 502 page.
		vi.stubGlobal(
			'fetch',
			async () =>
				new Response('<html><body>Bad Gateway</body></html>', {
					status: 502,
					statusText: 'Bad Gateway',
					headers: { 'Content-Type': 'text/html' }
				})
		);
		await expect(getJSON('/x')).rejects.toMatchObject({
			name: 'ApiError',
			status: 502,
			code: 'http_error'
		});
	});

	it('flags non-JSON 200 responses as bad_response', async () => {
		vi.stubGlobal(
			'fetch',
			async () => new Response('not json', { status: 200, headers: { 'Content-Type': 'text/plain' } })
		);
		await expect(getJSON('/x')).rejects.toMatchObject({
			name: 'ApiError',
			status: 200,
			code: 'bad_response'
		});
	});
});
