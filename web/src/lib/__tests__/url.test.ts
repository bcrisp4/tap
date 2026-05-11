import { describe, it, expect } from 'vitest';
import { originOf, displayUrl, formatAgo } from '../url';

describe('originOf', () => {
  it('returns the scheme + host for an absolute http(s) URL', () => {
    expect(originOf('https://jvns.ca/atom.xml')).toBe('https://jvns.ca');
    expect(originOf('http://example.com:8080/feed.xml')).toBe('http://example.com:8080');
  });
  it('does NOT double-prepend a scheme', () => {
    expect(originOf('https://jvns.ca/atom.xml')).not.toContain('https://https://');
  });
  it('returns the input unchanged on parse failure', () => {
    expect(originOf('not a url')).toBe('not a url');
  });
});

describe('displayUrl', () => {
  it('strips the scheme', () => {
    expect(displayUrl('https://jvns.ca/atom.xml')).toBe('jvns.ca/atom.xml');
  });
  it('trims trailing slash', () => {
    expect(displayUrl('https://jvns.ca/')).toBe('jvns.ca');
  });
  it('does NOT double-prepend a scheme — regression sentinel', () => {
    expect(displayUrl('https://jvns.ca/atom.xml')).not.toContain('https://https://');
  });
});

describe('formatAgo', () => {
  it.each([
    [0, '0s'], [59, '59s'], [60, '1m'], [120, '2m'], [3599, '59m'],
    [3600, '1h'], [86399, '23h'], [86400, '1d'], [Number.POSITIVE_INFINITY, '—'],
  ])('formatAgo(%i) === %s', (s, want) => expect(formatAgo(s)).toBe(want));
});
