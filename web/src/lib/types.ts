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
