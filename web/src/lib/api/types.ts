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
	category_id?: number | null;
	icon_id?: number | null;
	error_count: number;
	last_error?: string | null;
	crawler: boolean;
	disabled: boolean;
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
