import { writable } from 'svelte/store';

export function useRegisterSW(_options?: { onNeedRefresh?: () => void }) {
  return {
    needRefresh: writable(false),
    updateServiceWorker: async (_reloadPage?: boolean) => {},
  };
}
