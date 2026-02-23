import type { ApiBlock, GalleryItem } from './types';

export function normalizeGalleryItems(inputData: Record<string, any> | undefined): GalleryItem[] {
	if (Array.isArray(inputData?.items)) {
		return inputData.items
			.filter((item: any) => item && (item.kind === 'image' || item.kind === 'text' || item.kind === 'embed') && typeof item.value === 'string')
			.map((item: any, index: number) => ({
				id: typeof item.id === 'string' && item.id ? item.id : `item-${index}`,
				kind: item.kind,
				value: item.value
			}));
	}

	if (Array.isArray(inputData?.images)) {
		return inputData.images
			.filter((src: any) => typeof src === 'string' && src)
			.map((src: string, index: number) => ({ id: `legacy-img-${index}`, kind: 'image', value: src }));
	}

	return [];
}

export function toGalleryData(baseData: Record<string, any> | undefined, items: GalleryItem[], columns = 2) {
	return {
		...(baseData || {}),
		items,
		columns
	};
}

export function getBlockAsGalleryItems(block: ApiBlock, newItemId: () => string): GalleryItem[] {
	if (block.type === 'image') {
		return block.data?.url ? [{ id: newItemId(), kind: 'image', value: block.data.url }] : [];
	}

	if (block.type === 'embed') {
		return block.data?.url ? [{ id: newItemId(), kind: 'embed', value: block.data.url }] : [];
	}

	if (block.type === 'gallery') {
		return normalizeGalleryItems(block.data).map((item) => ({
			...item,
			id: item.id || newItemId()
		}));
	}

	const textValue = typeof block.data?.text === 'string' ? block.data.text.trim() : '';
	if (!textValue) return [];

	return [{ id: newItemId(), kind: 'text', value: textValue }];
}
