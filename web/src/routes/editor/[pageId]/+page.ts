import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params, url }) => {
	const pageId = encodeURIComponent(params.pageId);
	const share = url.searchParams.get('share');
	const shareQuery = share ? `&share=${encodeURIComponent(share)}` : '';
	throw redirect(307, `/editor?pageId=${pageId}${shareQuery}`);
};
