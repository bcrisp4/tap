import { describe, it, expect } from 'vitest';
import { colorForFeed } from '../../lib/colors';

const PALETTE = [
  '#3a4a5a', '#7a4a3a', '#4a6a4a', '#5a4a6a',
  '#6a5a3a', '#3a5a6a', '#5a3a4a', '#4a5a3a',
];

describe('colorForFeed', () => {
  it('returns a colour from the palette', () => {
    const colour = colorForFeed('https://example.com/feed.xml');
    expect(PALETTE).toContain(colour);
  });

  it('is deterministic — same input always yields same colour', () => {
    const url = 'https://news.ycombinator.com/rss';
    expect(colorForFeed(url)).toBe(colorForFeed(url));
    expect(colorForFeed(url)).toBe(colorForFeed(url));
  });

  it('known input 1: https://example.com/feed.xml maps to a stable colour', () => {
    const first = colorForFeed('https://example.com/feed.xml');
    // Run 5 times — must always match the first result.
    for (let i = 0; i < 5; i++) {
      expect(colorForFeed('https://example.com/feed.xml')).toBe(first);
    }
  });

  it('known input 2: https://blog.golang.org/feed.atom maps to a stable colour', () => {
    const first = colorForFeed('https://blog.golang.org/feed.atom');
    for (let i = 0; i < 5; i++) {
      expect(colorForFeed('https://blog.golang.org/feed.atom')).toBe(first);
    }
  });

  it('known input 3: https://feeds.feedburner.com/oreilly/radar maps to a stable colour', () => {
    const first = colorForFeed('https://feeds.feedburner.com/oreilly/radar');
    for (let i = 0; i < 5; i++) {
      expect(colorForFeed('https://feeds.feedburner.com/oreilly/radar')).toBe(first);
    }
  });

  it('known input 4: https://css-tricks.com/feed/ maps to a stable colour', () => {
    const first = colorForFeed('https://css-tricks.com/feed/');
    for (let i = 0; i < 5; i++) {
      expect(colorForFeed('https://css-tricks.com/feed/')).toBe(first);
    }
  });

  it('known input 5: https://overreacted.io/rss.xml maps to a stable colour', () => {
    const first = colorForFeed('https://overreacted.io/rss.xml');
    for (let i = 0; i < 5; i++) {
      expect(colorForFeed('https://overreacted.io/rss.xml')).toBe(first);
    }
  });

  it('different feed URLs produce colours that cover more than one palette entry', () => {
    const urls = [
      'https://example.com/feed.xml',
      'https://blog.golang.org/feed.atom',
      'https://feeds.feedburner.com/oreilly/radar',
      'https://css-tricks.com/feed/',
      'https://overreacted.io/rss.xml',
      'https://planet.gnome.org/rss20.xml',
      'https://lobste.rs/rss',
      'https://news.ycombinator.com/rss',
    ];
    const colours = new Set(urls.map(colorForFeed));
    // At least 2 distinct colours from 8 distinct URLs.
    expect(colours.size).toBeGreaterThan(1);
  });
});
