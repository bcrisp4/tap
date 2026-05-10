import { writable, type Readable } from 'svelte/store';

type RouteState =
  | { name: 'unread' }
  | { name: 'reader'; params: { id: number } }
  | { name: 'saved' }
  | { name: 'search' }
  | { name: 'settings' }
  | { name: 'admin' }
  | { name: 'category'; params: { id: number } };

function parse(pathname: string): RouteState {
  const m = pathname.match(/^\/entry\/(\d+)$/);
  if (m) return { name: 'reader', params: { id: Number(m[1]) } };
  const mc = pathname.match(/^\/categories\/(\d+)$/);
  if (mc) return { name: 'category', params: { id: Number(mc[1]) } };
  if (pathname === '/saved')    return { name: 'saved' };
  if (pathname === '/search')   return { name: 'search' };
  if (pathname === '/settings') return { name: 'settings' };
  if (pathname === '/admin')    return { name: 'admin' };
  return { name: 'unread' };
}

const internal = writable<RouteState>(parse(window.location.pathname));

window.addEventListener('popstate', () => internal.set(parse(window.location.pathname)));

export const route: Readable<RouteState> = { subscribe: internal.subscribe };

export function navigate(to: string) {
  if (window.location.pathname === to) return;
  window.history.pushState({}, '', to);
  internal.set(parse(to));
}
