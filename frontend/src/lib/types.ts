export interface AlbumSummary {
	name: string;
	count: number;
	cover_url?: string;
}

export interface Photo {
	id: string;
	file_name: string;
	url: string;
	album_name: string;
	uploader: string;
	uploaded_at: string;
}
