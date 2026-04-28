// Wire types for the Tap REST API. Mirrors design.md §6 envelope:
// list endpoints return `{ data, pagination }`; singles return bare
// objects; errors return `{ error: { code, message } }`.

export interface Pagination {
	limit: number;
	offset: number;
	total: number;
}

export interface ListResponse<T> {
	data: T[];
	pagination: Pagination;
}

export interface Feed {
	id: number;
	title: string;
	feed_url: string;
	site_url?: string | null;
	description?: string | null;
	category_id?: number | null;
	icon_id?: number | null;
	// SHA-256 hex of the cached favicon bytes; the SPA fetches the
	// image at /api/v1/icons/<hash>. Optional: set after the poller's
	// favicon scrape lands. Plan 17.
	icon_hash?: string | null;
	last_polled_at?: number | null;
	next_poll_at?: number | null;
	poll_interval?: number;
	error_count: number;
	last_error?: string | null;
	crawler: boolean;
	scraper_rules?: string | null;
	disabled: boolean;
	ignore_entry_updates?: boolean;
	user_agent?: string | null;
	weekly_entry_count: number;
}

export interface Category {
	id: number;
	user_id: number;
	name: string;
}

export interface Enclosure {
	id: number;
	entry_id: number;
	url: string;
	mime_type: string;
	size: number;
}

export interface Entry {
	id: number;
	feed_id: number;
	user_id: number;
	hash: string;
	title: string;
	url?: string | null;
	author?: string | null;
	summary?: string | null;
	content?: string | null;
	published_at?: number | null;
	reading_time: number;
	read: boolean;
	saved: boolean;
	extraction_failed: boolean;
	created_at: number;
	enclosures?: Enclosure[];
}

export interface SystemStatus {
	version: string;
	uptime_seconds: number;
	run_state: {
		active_polls: number;
		last_poll_at: number;
		recent_errors?: string[];
	};
}

// FeedPatch encodes credential-redaction discipline (Plan 08): the
// API stores cookie/username/password/proxy_url but redacts them on
// read (json:"-"). Sending an empty string would clobber the stored
// value; the edit form must therefore *omit* the key when the input
// is empty. Modelling those fields as optional (and only ever
// assigning when a non-empty string is present) makes the contract
// self-enforcing on the SPA side.
export interface FeedPatch {
	title?: string;
	feed_url?: string;
	site_url?: string | null;
	category_id?: number | null;
	crawler?: boolean;
	disabled?: boolean;
	scraper_rules?: string | null;
	user_agent?: string | null;
	cookie?: string;
	username?: string;
	password?: string;
	proxy_url?: string;
}

// DiscoverCandidate / DiscoverResult mirror the
// POST /api/v1/feeds/discover wire shape. The endpoint returns
// `{candidates: [{href, title?, type?}]}` (NOT the design.md §6 list
// envelope — discover predates that).
export interface DiscoverCandidate {
	href: string;
	title?: string;
	type?: string;
}

export interface DiscoverResult {
	candidates: DiscoverCandidate[];
}
