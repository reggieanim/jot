const URL_PROTOCOL_RE = /^(https?:|mailto:|tel:)/i;

export function escapeHtml(input: string): string {
	return input
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/\"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

function sanitizeHref(value: string | null): string {
	const href = (value || '').trim();
	if (!href) return '';
	if (href.startsWith('#') || href.startsWith('/')) return href;
	return URL_PROTOCOL_RE.test(href) ? href : '';
}

function basicSanitize(html: string): string {
	return html
		.replace(/<\s*script[^>]*>[\s\S]*?<\s*\/\s*script>/gi, '')
		.replace(/<\s*style[^>]*>[\s\S]*?<\s*\/\s*style>/gi, '')
		.replace(/\son\w+\s*=\s*(['"]).*?\1/gi, '')
		.replace(/\sjavascript:/gi, ' ');
}

export function sanitizeRichText(html: string): string {
	const raw = (html || '').trim();
	if (!raw) return '';

	if (typeof window === 'undefined' || typeof document === 'undefined') {
		return basicSanitize(raw);
	}

	const template = document.createElement('template');
	template.innerHTML = raw;
	const allowedTags = new Set([
		'B',
		'STRONG',
		'I',
		'EM',
		'U',
		'S',
		'STRIKE',
		'BR',
		'P',
		'DIV',
		'SPAN',
		'UL',
		'OL',
		'LI',
		'CODE',
		'A'
	]);

	const walk = (node: Node) => {
		if (node.nodeType === Node.ELEMENT_NODE) {
			const el = node as HTMLElement;
			const tagName = el.tagName.toUpperCase();

			if (!allowedTags.has(tagName)) {
				const parent = el.parentNode;
				if (!parent) return;
				while (el.firstChild) parent.insertBefore(el.firstChild, el);
				parent.removeChild(el);
				return;
			}

			for (const attr of [...el.attributes]) {
				const name = attr.name.toLowerCase();
				if (name.startsWith('on')) {
					el.removeAttribute(attr.name);
					continue;
				}
				if (tagName !== 'A') {
					el.removeAttribute(attr.name);
					continue;
				}
				if (name !== 'href') {
					el.removeAttribute(attr.name);
				}
			}

			if (tagName === 'A') {
				const cleanHref = sanitizeHref(el.getAttribute('href'));
				if (!cleanHref) {
					el.removeAttribute('href');
				} else {
					el.setAttribute('href', cleanHref);
					el.setAttribute('target', '_blank');
					el.setAttribute('rel', 'noopener noreferrer nofollow');
				}
			}
		}

		for (const child of [...node.childNodes]) {
			walk(child);
		}
	};

	walk(template.content);
	return template.innerHTML.trim();
}

export function plainTextFromHtml(html: string): string {
	const raw = (html || '').trim();
	if (!raw) return '';

	if (typeof window === 'undefined' || typeof document === 'undefined') {
		return raw
			.replace(/<br\s*\/?\s*>/gi, '\n')
			.replace(/<\/(p|div|li|h1|h2|h3|blockquote)>/gi, '\n')
			.replace(/<[^>]+>/g, '')
			.replace(/\n{3,}/g, '\n\n')
			.trim();
	}

	const div = document.createElement('div');
	div.innerHTML = raw;
	return (div.textContent || '').replace(/\u00a0/g, ' ').trim();
}

export function htmlFromBlockData(data: Record<string, any> | undefined): string {
	if (typeof data?.html === 'string' && data.html.trim()) {
		return sanitizeRichText(data.html);
	}
	if (typeof data?.text === 'string' && data.text.length > 0) {
		return escapeHtml(data.text).replace(/\n/g, '<br>');
	}
	return '';
}

export function plainTextFromBlockData(data: Record<string, any> | undefined): string {
	if (typeof data?.text === 'string') return data.text;
	if (typeof data?.html === 'string') return plainTextFromHtml(data.html);
	return '';
}
