import type { ApiBlock } from '$lib/editor/types';
import * as mammoth from 'mammoth/mammoth.browser';

function makeId(): string {
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
		return `tmp-${crypto.randomUUID()}`;
	}
	return `tmp-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function makeBlock(type: string, position: number, data: Record<string, any>): ApiBlock {
	return { id: makeId(), type, position, data };
}

function normalizeText(text: string): string {
	return text.replace(/\r\n?/g, '\n').replace(/\u0000/g, '');
}

function cleanInlineMarkdown(text: string): string {
	return text
		.replace(/\*\*(.*?)\*\*/g, '$1')
		.replace(/__(.*?)__/g, '$1')
		.replace(/\*(.*?)\*/g, '$1')
		.replace(/_(.*?)_/g, '$1')
		.replace(/`([^`]+)`/g, '$1')
		.replace(/\[(.*?)\]\((.*?)\)/g, '$1')
		.trim();
}

export function parseMarkdownToBlocks(markdown: string): { title?: string; blocks: ApiBlock[] } {
	const source = normalizeText(markdown).trim();
	if (!source) return { blocks: [makeBlock('paragraph', 0, { text: '' })] };

	const lines = source.split('\n');
	const blocks: ApiBlock[] = [];
	let paragraphLines: string[] = [];
	let inCodeFence = false;
	let codeLang = 'text';
	let codeLines: string[] = [];
	let title: string | undefined;

	const flushParagraph = () => {
		if (paragraphLines.length === 0) return;
		const text = cleanInlineMarkdown(paragraphLines.join('\n').trim());
		if (text) blocks.push(makeBlock('paragraph', blocks.length, { text }));
		paragraphLines = [];
	};

	const flushCode = () => {
		const text = codeLines.join('\n').trimEnd();
		blocks.push(makeBlock('code', blocks.length, { code: text, language: codeLang || 'text' }));
		codeLines = [];
		codeLang = 'text';
	};

	for (const rawLine of lines) {
		const line = rawLine.replace(/\t/g, '    ');
		const trimmed = line.trim();

		if (inCodeFence) {
			if (trimmed.startsWith('```')) {
				flushCode();
				inCodeFence = false;
				continue;
			}
			codeLines.push(rawLine);
			continue;
		}

		if (trimmed.startsWith('```')) {
			flushParagraph();
			inCodeFence = true;
			codeLang = trimmed.slice(3).trim() || 'text';
			continue;
		}

		if (trimmed === '') {
			flushParagraph();
			continue;
		}

		if (/^---+$/.test(trimmed) || /^\*\*\*+$/.test(trimmed) || /^___+$/.test(trimmed)) {
			flushParagraph();
			blocks.push(makeBlock('divider', blocks.length, {}));
			continue;
		}

		const headingMatch = /^(#{1,3})\s+(.*)$/.exec(trimmed);
		if (headingMatch) {
			flushParagraph();
			const level = headingMatch[1].length;
			const headingText = cleanInlineMarkdown(headingMatch[2]);
			if (!title && level === 1) title = headingText;
			const type = level === 1 ? 'heading' : level === 2 ? 'heading2' : 'heading3';
			blocks.push(makeBlock(type, blocks.length, { text: headingText }));
			continue;
		}

		const quoteMatch = /^>\s?(.*)$/.exec(trimmed);
		if (quoteMatch) {
			flushParagraph();
			blocks.push(makeBlock('quote', blocks.length, { text: cleanInlineMarkdown(quoteMatch[1]) }));
			continue;
		}

		const numberedMatch = /^\d+[.)]\s+(.*)$/.exec(trimmed);
		if (numberedMatch) {
			flushParagraph();
			blocks.push(makeBlock('numbered', blocks.length, { text: cleanInlineMarkdown(numberedMatch[1]) }));
			continue;
		}

		const bulletMatch = /^[-*+]\s+(.*)$/.exec(trimmed);
		if (bulletMatch) {
			flushParagraph();
			blocks.push(makeBlock('bullet', blocks.length, { text: cleanInlineMarkdown(bulletMatch[1]) }));
			continue;
		}

		const imageMatch = /^!\[[^\]]*\]\((https?:\/\/[^)\s]+)\)/.exec(trimmed);
		if (imageMatch) {
			flushParagraph();
			blocks.push(makeBlock('image', blocks.length, { url: imageMatch[1] }));
			continue;
		}

		if (/^https?:\/\/\S+$/.test(trimmed)) {
			flushParagraph();
			blocks.push(makeBlock('embed', blocks.length, { url: trimmed }));
			continue;
		}

		paragraphLines.push(rawLine);
	}

	if (inCodeFence) flushCode();
	flushParagraph();

	if (blocks.length === 0) blocks.push(makeBlock('paragraph', 0, { text: '' }));
	blocks.forEach((block, index) => (block.position = index));

	return { title, blocks };
}

export function parsePlainTextToBlocks(text: string): { title?: string; blocks: ApiBlock[] } {
	const source = normalizeText(text).trim();
	if (!source) return { blocks: [makeBlock('paragraph', 0, { text: '' })] };

	const chunks = source
		.split(/\n\s*\n/g)
		.map((chunk) => chunk.trim())
		.filter(Boolean);

	const blocks = chunks.map((chunk, index) => makeBlock('paragraph', index, { text: chunk }));
	const title = chunks[0]?.split('\n')[0]?.trim();
	return { title, blocks };
}

function parseDocxHtmlToBlocks(html: string): { title?: string; blocks: ApiBlock[] } {
	const source = html.trim();
	if (!source) return { blocks: [makeBlock('paragraph', 0, { text: '' })] };

	const doc = new DOMParser().parseFromString(source, 'text/html');
	const body = doc.body;
	const blocks: ApiBlock[] = [];
	let title: string | undefined;

	const addTextBlock = (type: string, text: string) => {
		const clean = normalizeText(text).replace(/\s+/g, ' ').trim();
		if (!clean) return;
		if (!title && type === 'heading') title = clean;
		blocks.push(makeBlock(type, blocks.length, { text: clean }));
	};

	const nodes = Array.from(body.children);
	for (const node of nodes) {
		const tag = node.tagName.toUpperCase();
		if (tag === 'H1') {
			addTextBlock('heading', node.textContent || '');
			continue;
		}
		if (tag === 'H2') {
			addTextBlock('heading2', node.textContent || '');
			continue;
		}
		if (tag === 'H3' || tag === 'H4' || tag === 'H5' || tag === 'H6') {
			addTextBlock('heading3', node.textContent || '');
			continue;
		}

		if (tag === 'UL' || tag === 'OL') {
			const listType = tag === 'OL' ? 'numbered' : 'bullet';
			const items = Array.from(node.querySelectorAll(':scope > li'));
			for (const li of items) {
				addTextBlock(listType, li.textContent || '');
			}
			continue;
		}

		if (tag === 'BLOCKQUOTE') {
			addTextBlock('quote', node.textContent || '');
			continue;
		}

		if (tag === 'HR') {
			blocks.push(makeBlock('divider', blocks.length, {}));
			continue;
		}

		if (tag === 'PRE') {
			const codeEl = node.querySelector('code');
			const code = (codeEl?.textContent || node.textContent || '').replace(/\s+$/g, '');
			if (code.trim()) blocks.push(makeBlock('code', blocks.length, { code, language: 'text' }));
			continue;
		}

		if (tag === 'IMG') {
			const url = (node as HTMLImageElement).src?.trim();
			if (url) blocks.push(makeBlock('image', blocks.length, { url }));
			continue;
		}

		if (tag === 'P' || tag === 'DIV') {
			const text = node.textContent || '';
			const links = Array.from(node.querySelectorAll('a[href]'));
			if (links.length === 1 && normalizeText(text).trim() === normalizeText(links[0].textContent || '').trim()) {
				const href = links[0].getAttribute('href')?.trim() || '';
				if (/^https?:\/\//i.test(href)) {
					blocks.push(makeBlock('embed', blocks.length, { url: href }));
					continue;
				}
			}
			addTextBlock('paragraph', text);
			continue;
		}

		addTextBlock('paragraph', node.textContent || '');
	}

	if (blocks.length === 0) {
		const raw = normalizeText(body.textContent || '').trim();
		if (raw) {
			return parsePlainTextToBlocks(raw);
		}
		return { blocks: [makeBlock('paragraph', 0, { text: '' })] };
	}

	blocks.forEach((block, index) => (block.position = index));
	return { title, blocks };
}

export async function parseImportedDocument(content: string | ArrayBuffer, fileName: string): Promise<{ title?: string; blocks: ApiBlock[] }> {
	const lowerName = fileName.toLowerCase();
	if (lowerName.endsWith('.docx')) {
		const arrayBuffer = typeof content === 'string' ? new TextEncoder().encode(content).buffer : content;
		const htmlResult = await mammoth.convertToHtml({ arrayBuffer });
		const parsed = parseDocxHtmlToBlocks(htmlResult.value || '');
		if (parsed.blocks.length > 0) return parsed;
		const rawTextResult = await mammoth.extractRawText({ arrayBuffer });
		return parsePlainTextToBlocks(rawTextResult.value || '');
	}
	if (lowerName.endsWith('.md') || lowerName.endsWith('.markdown')) {
		return parseMarkdownToBlocks(typeof content === 'string' ? content : new TextDecoder().decode(content));
	}
	return parsePlainTextToBlocks(typeof content === 'string' ? content : new TextDecoder().decode(content));
}
