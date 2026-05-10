/// <reference types="vite/client" />

declare module 'virtual:pwa-register/svelte' {
  import type { Writable } from 'svelte/store';
  export function useRegisterSW(options?: { onNeedRefresh?: () => void }): {
    needRefresh: Writable<boolean>;
    updateServiceWorker: (reloadPage?: boolean) => Promise<void>;
  };
}
