import { env } from '$env/dynamic/public';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => {
	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';
	const { pageId, blockId } = params;

	const res = await fetch(
		`${apiUrl}/v1/public/pages/${encodeURIComponent(pageId)}/blocks/${encodeURIComponent(blockId)}`
	);

	if (!res.ok) {
		throw error(404, 'Block not found');
	}

	const payload = await res.json();

	return {
		block: payload.block,
		page: payload.page
	};
};
