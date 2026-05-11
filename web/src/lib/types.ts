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
  extract: boolean;
  extract_selector: string;
  has_cookie: boolean;
  has_basic_auth: boolean;
  category_id: number | null;
};

export type Category = {
  id: number;
  name: string;
  unread: number;
  created_at: number;
  position: number;
};

export type DiscoverCandidate = {
  title: string;
  feed_url: string;
  site_url: string;
  type: string;
};

export type DiscoverResult = { candidates: DiscoverCandidate[] };

export type OPMLImportResult = { imported: number; skipped: number; errors: string[] };

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
  extract_failed: boolean;
};

export type EntryDetail = EntryListItem & {
  content: string;
};

export type ListResponse<T> = {
  data: T[];
  next_cursor?: string;
};

export type ApiError = {
  error: { code: string; message: string };
};

export type User = {
  id: number;
  username: string;
  role: 'admin' | 'user';
  has_totp: boolean;
  passkey_count: number;
};

export type SessionResponse = {
  user: User;
  csrf_token: string;
};

export type PasswordChangeResponse = {
  csrf_token: string;
};

// M7: session listing
export type Session = {
  id: number;
  created_at: number;
  last_seen_at: number;
  idle_expires_at: number;
  user_agent: string;
  address: string;
  current: boolean;
};

// M7: TOTP enrolment
export type TOTPEnrolmentBegin = {
  secret_uri: string;
  secret: string;
};

export type TOTPConfirmResponse = {
  recovery_codes: string[];
};

// M7: passkeys
export type Passkey = {
  id: number;
  label: string;
  created_at: number;
};

// M7: admin user management
export type AdminUser = {
  id: number;
  username: string;
  role: 'admin' | 'user';
  created_at: number;
  disabled_at: number | null;
  has_totp: boolean;
  passkey_count: number;
};

// M7: TOTP login
export type TOTPRequiredResponse = {
  totp_required: true;
  pending_token: string;
};

export type LoginResponse = SessionResponse | TOTPRequiredResponse;
