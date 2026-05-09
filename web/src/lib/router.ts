// Stub — full implementation in Phase 9.
// Replaced by the real router when Phase 9 lands.
import { readable } from 'svelte/store';

export type Route =
  | { name: 'unread'; params: Record<string, never> }
  | { name: 'reader'; params: { id: string } };

export const route = readable<Route>({ name: 'unread', params: {} });
