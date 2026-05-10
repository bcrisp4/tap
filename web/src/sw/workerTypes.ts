export type SWMessage =
  | { type: 'set-user'; userId: number }
  | { type: 'logout'; userId: number };
