export interface AlbumSummary {
	name: string;
	count: number;
	cover_url?: string;
}

export interface Photo {
	id: string;
	file_name: string;
	url: string;
	// Absent for photos uploaded before thumbnails existed; fall back to url.
	thumb_url?: string;
	album_name: string;
	uploader: string;
	uploaded_at: string;
}
