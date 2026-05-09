const PALETTE = [
  '#3a4a5a', '#7a4a3a', '#4a6a4a', '#5a4a6a',
  '#6a5a3a', '#3a5a6a', '#5a3a4a', '#4a5a3a',
];

export function colorForFeed(feedURL: string): string {
  let hash = 0;
  for (let i = 0; i < feedURL.length; i++) {
    hash = (hash * 31 + feedURL.charCodeAt(i)) | 0;
  }
  return PALETTE[Math.abs(hash) % PALETTE.length];
}
