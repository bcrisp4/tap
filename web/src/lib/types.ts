export type Subscription = {
  id: number;
  title: string;
  feed_url: string;
  site_url?: string;
  next_poll_at: number;
  last_poll_at?: number;
  error_count: number;
  last_error?: string;
  created_at: number;
  // M5 backfill — present on the wire since M5; the type was missing them.
  extract: boolean;
  extract_selector: string;
  // M6.
  has_cookie: boolean;
  has_basic_auth: boolean;
};

export type EntryListItem = {
  id: number;
  subscription_id: number;
  title: string;
  author?: string;
  url: string;
  published_at: number;
  fetched_at: number;
  read: boolean;
  saved: boolean;
  // M5 backfill.
  extract_failed: boolean;
};

export type EntryDetail = EntryListItem & {
  content: string;
};

export type ListResponse<T> = {
  data: T[];
  // Opaque cursor string ("<published_at>_<id>"). Pass back to the next request
  // as ?cursor=. Absent when there are no more pages.
  next_cursor?: string;
};

export type ApiError = {
  error: { code: string; message: string };
};

// M6: auth types.
export type User = {
  id: number;
  username: string;
  role: 'admin' | 'user';
};

export type SessionResponse = {
  user: User;
  csrf_token: string;
};

export type PasswordChangeResponse = {
  csrf_token: string;
};
