// SPA mode: no SSR / no prerender. The Go binary serves a single
// index.html that boots the client-side app for every route.
export const prerender = false;
export const ssr = false;
export const csr = true;
