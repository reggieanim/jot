export type ApiBlock = {
	id?: string;
	type: string;
	position: number;
	data: Record<string, any>;
};

export type ApiPage = {
	id: string;
	title: string;
	cover?: string;
	published?: boolean;
	published_at?: string;
	dark_mode?: boolean;
	cinematic?: boolean;
	mood?: number;
	bg_color?: string;
	blocks?: ApiBlock[];
	proofread_count?: number;
	block_count?: number;
	updated_at?: string;
	deleted_at?: string;
};

export type ApiProofreadAnnotation = {
	id: string;
	block_id: string;
	kind: string;
	quote: string;
	text: string;
};

export type ApiProofread = {
	id: string;
	page_id: string;
	author_name: string;
	title: string;
	summary: string;
	stance: string;
	annotations: ApiProofreadAnnotation[];
	created_at: string;
	updated_at: string;
};

export type GalleryItem = {
	id: string;
	kind: 'image' | 'text' | 'embed';
	value: string;
};

export type Rgb = [number, number, number];
