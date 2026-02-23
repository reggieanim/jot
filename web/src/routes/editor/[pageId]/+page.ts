import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params }) => {
	const pageId = encodeURIComponent(params.pageId);
	throw redirect(307, `/editor?pageId=${pageId}`);
};
