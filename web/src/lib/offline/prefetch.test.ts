import { describe, it, expect } from 'vitest';
import { extractProxyURLs } from './prefetch';

describe('extractProxyURLs', () => {
	it('returns [] for null / empty content', () => {
		expect(extractProxyURLs(null)).toEqual([]);
		expect(extractProxyURLs(undefined)).toEqual([]);
		expect(extractProxyURLs('')).toEqual([]);
	});

	it('finds /api/v1/proxy/* URLs inside an entry HTML body', () => {
		const html =
			'<p>hi</p><img src="/api/v1/proxy/abc123_def-456">' +
			'<img src="https://elsewhere.example.com/x.png"/>' +
			'<p>bye</p>';
		expect(extractProxyURLs(html)).toEqual(['/api/v1/proxy/abc123_def-456']);
	});

	it('caps the result at MAX_URLS_PER_ENTRY', () => {
		// The function does not dedupe inside one entry — that's the
		// caller's job (it stuffs them into a Set across entries). What
		// we DO test is the cap.
		const url = '/api/v1/proxy/AAAA';
		const html = Array(100).fill(`<img src="${url}">`).join('');
		const out = extractProxyURLs(html);
		expect(out.length).toBe(32);
		expect(out.every((u) => u === url)).toBe(true);
	});

	it('matches the base64url + dot character class', () => {
		const html =
			'<img src="/api/v1/proxy/aZ09_-=.token">' +
			'<img src="/api/v1/proxy/should~not~match">';
		const out = extractProxyURLs(html);
		expect(out).toContain('/api/v1/proxy/aZ09_-=.token');
		// Tokens that contain disallowed characters with no terminating
		// boundary right after the run are skipped entirely (rather than
		// truncated into a bogus partial URL that the server would 404).
		expect(out).not.toContain('/api/v1/proxy/should');
	});

	it('still matches valid tokens at end-of-string with no terminator', () => {
		const html = '/api/v1/proxy/standalone';
		expect(extractProxyURLs(html)).toEqual(['/api/v1/proxy/standalone']);
	});
});
