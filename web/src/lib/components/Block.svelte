<script lang="ts">
	import { createEventDispatcher, onMount, onDestroy, tick } from 'svelte';
	import hljs from 'highlight.js/lib/common';
	import { htmlFromBlockData, plainTextFromBlockData, sanitizeRichText } from '$lib/editor/richtext';
	import { isLocalMediaRef, putLocalMediaBlob, resolveLocalMediaObjectURL } from '$lib/editor/localMedia';
	import { copyTextToClipboard } from '$lib/utils/clipboard';
	import MusicPlayer from '$lib/components/MusicPlayer.svelte';

	export let id: string;
	export let type: string;
	export let data: any;
	export let apiUrl = 'http://localhost:8080';
	export let pageId = '';
	export let shareToken = '';
	export let allowLocalMedia = false;
	export let published = false;
	export let isDragging = false;
	export let viewerSessionId = '';
	export let lockOwner: { sessionId: string; userName: string } | null = null;
	export let listNumber = 1;

	let showShareToast = false;
	let showShareMenu = false;
	let supportsNativeShare = false;
	let resolvedImageUrl = '';
	let galleryImageUrls: Record<string, string> = {};
	let resolveImageRun = 0;
	let resolveGalleryRun = 0;
	let shareMenuEl: HTMLDivElement;
	let shareBtnEl: HTMLButtonElement;

	const dispatch = createEventDispatcher();

	let contentEl: HTMLElement;
	let imageInputEl: HTMLInputElement;
	let galleryInputEl: HTMLInputElement;
	let showSlashMenu = false;
	let slashMenuPosition = { x: 0, y: 0 };
	let slashMenuPlacement: 'above' | 'below' = 'below';
	let selectedMenuIndex = 0;
	let localText = plainTextFromBlockData(data);
	let localHtml = htmlFromBlockData(data);
	let saveTimeout: ReturnType<typeof setTimeout>;
	let isEditingText = false;
	let hasRichSelection = false;
	let savedSelection: Range | null = null;
	$: isLocked = !!lockOwner && lockOwner.sessionId !== viewerSessionId;
	$: incomingText = plainTextFromBlockData(data);
	$: incomingHtml = htmlFromBlockData(data);
	$: if (!isEditingText && (incomingText !== localText || incomingHtml !== localHtml)) {
		localText = incomingText;
		localHtml = incomingHtml;
	}
	$: isRichTextType = ['paragraph', 'heading', 'heading2', 'heading3', 'bullet', 'numbered', 'quote'].includes(type);
	$: if (type === 'image') {
		void resolveCurrentImageUrl(String(data?.url || ''));
	}
	$: if (type === 'gallery') {
		void resolveCurrentGalleryUrls(normalizeGalleryItems(data));
	}

	type GalleryItem = {
		id: string;
		kind: 'image' | 'text' | 'embed';
		value: string;
	};

	const slashCommands = [
		{ id: 'paragraph', label: 'Text', icon: '¶', description: 'Plain text block' },
		{ id: 'heading', label: 'Heading', icon: 'H', description: 'Large section heading' },
		{ id: 'heading2', label: 'Heading 2', icon: 'H2', description: 'Medium heading' },
		{ id: 'heading3', label: 'Heading 3', icon: 'H3', description: 'Small heading' },
		{ id: 'bullet', label: 'Bullet List', icon: '•', description: 'Bulleted list item' },
		{ id: 'numbered', label: 'Numbered List', icon: '1.', description: 'Numbered list item' },
		{ id: 'quote', label: 'Quote', icon: '"', description: 'Capture a quote' },

		{ id: 'image', label: 'Image', icon: '🖼', description: 'Upload or embed image' },
		{ id: 'gallery', label: 'Gallery', icon: '▦', description: '2-4 image columns' },
		{ id: 'embed', label: 'Embed', icon: '◆', description: 'Embed external content' },
		{ id: 'code', label: 'Code', icon: '</>', description: 'Code block with syntax highlighting' },
		{ id: 'canvas', label: 'Canvas', icon: '🎨', description: 'JavaScript canvas playground' },
		{ id: 'music', label: 'Music', icon: '♫', description: 'Audio player with waveform' }
	];

	function saveText() {
		if (contentEl) {
			const html = sanitizeRichText(contentEl.innerHTML ?? '');
			const text = (contentEl.textContent ?? '').replace(/\u00a0/g, ' ').trimEnd();
			localHtml = html;
			localText = text;
			dispatch('update', { id, type, data: { ...data, text, html } });
		}
	}

	function selectionInContent() {
		if (!contentEl) return false;
		const selection = window.getSelection();
		if (!selection || selection.rangeCount === 0) return false;
		const { anchorNode, focusNode } = selection;
		if (!anchorNode || !focusNode) return false;
		return contentEl.contains(anchorNode) && contentEl.contains(focusNode);
	}

	function refreshSelectionState() {
		if (!isRichTextType || !isEditingText || !contentEl) {
			hasRichSelection = false;
			return;
		}
		const selection = window.getSelection();
		if (!selection || selection.rangeCount === 0 || !selectionInContent()) {
			hasRichSelection = false;
			return;
		}
		hasRichSelection = !selection.isCollapsed;
		savedSelection = selection.getRangeAt(0).cloneRange();
	}

	function restoreSelection() {
		if (!savedSelection) return;
		const selection = window.getSelection();
		if (!selection) return;
		selection.removeAllRanges();
		selection.addRange(savedSelection);
	}

	function debouncedSave() {
		clearTimeout(saveTimeout);
		saveTimeout = setTimeout(saveText, 500);
	}

	function handleInput(e: Event) {
		if (isLocked) return;
		const target = e.target as HTMLElement;
		const text = target.textContent ?? '';
		localHtml = target.innerHTML ?? '';
		localText = text;
		refreshSelectionState();
		dispatch('typing', { id, isTyping: true });

		// Check for slash command
		if (text.endsWith('/')) {
			const sel = window.getSelection();
			if (sel && sel.rangeCount > 0) {
				const rect = sel.getRangeAt(0).getBoundingClientRect();
				setSlashMenuPosition(rect);
				showSlashMenu = true;
				selectedMenuIndex = 0;
			}
		} else {
			showSlashMenu = false;
		}

		// Debounced save - don't update parent on every keystroke
		debouncedSave();
	}

	function handleBlur() {
		if (isLocked) return;
		isEditingText = false;
		hasRichSelection = false;
		clearTimeout(saveTimeout);
		dispatch('typing', { id, isTyping: false });
		saveText();
	}

	function handleFocus() {
		if (isLocked) return;
		isEditingText = true;
		refreshSelectionState();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (isLocked) {
			e.preventDefault();
			e.stopPropagation();
			return;
		}

		if (showSlashMenu) {
			if (e.key === 'ArrowDown') {
				e.preventDefault();
				selectedMenuIndex = (selectedMenuIndex + 1) % slashCommands.length;
			} else if (e.key === 'ArrowUp') {
				e.preventDefault();
				selectedMenuIndex = (selectedMenuIndex - 1 + slashCommands.length) % slashCommands.length;
			} else if (e.key === 'Enter') {
				e.preventDefault();
				e.stopPropagation();
				selectSlashCommand(slashCommands[selectedMenuIndex].id);
			} else if (e.key === 'Escape') {
				showSlashMenu = false;
			}
			return;
		}

		const mod = e.metaKey || e.ctrlKey;

		/* ---- Rich-text keyboard shortcuts ---- */
		if (mod && isRichTextType) {
			switch (e.key.toLowerCase()) {
				case 'b':
					e.preventDefault();
					applyFormat('bold');
					return;
				case 'i':
					e.preventDefault();
					applyFormat('italic');
					return;
				case 'u':
					e.preventDefault();
					applyFormat('underline');
					return;
				case 'k':
					e.preventDefault();
					applyLink();
					return;
				case 'e':
					e.preventDefault();
					toggleInlineTag('CODE');
					return;
				case 'h':
					if (e.shiftKey) {
						e.preventDefault();
						toggleInlineTag('MARK');
						return;
					}
					break;
			}
		}

		/* Markdown auto-convert on space */
		if (e.key === ' ' && type === 'paragraph' && isRichTextType) {
			if (tryMarkdownShortcut()) {
				e.preventDefault();
				return;
			}
		}

		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			e.stopPropagation();

			/* If a list item is empty on Enter, convert it to paragraph (exit list) */
			if ((type === 'bullet' || type === 'numbered') && contentEl?.innerText.trim() === '') {
				dispatch('transform', { id, newType: 'paragraph' });
				return;
			}

			saveText();
			dispatch('addAfter', { id, type });
		}

		if (e.key === 'Backspace' && contentEl?.innerText === '') {
			e.preventDefault();
			e.stopPropagation();
			/* If backspacing an empty list item, convert to paragraph instead of deleting */
			if (type === 'bullet' || type === 'numbered') {
				dispatch('transform', { id, newType: 'paragraph' });
			} else {
				dispatch('delete', { id });
			}
		}
	}

	/* ---- Format state detection ---- */
	let formatBold = false;
	let formatItalic = false;
	let formatUnderline = false;
	let formatStrike = false;
	let formatCode = false;
	let formatHighlight = false;
	let formatLink = false;

	let toolbarStyle = '';

	function detectFormats() {
		if (!isRichTextType || !isEditingText) return;
		formatBold = document.queryCommandState('bold');
		formatItalic = document.queryCommandState('italic');
		formatUnderline = document.queryCommandState('underline');
		formatStrike = document.queryCommandState('strikeThrough');
		// Code & highlight aren't execCommand-backed, detect via ancestor
		const sel = window.getSelection();
		if (sel && sel.rangeCount > 0) {
			const node = sel.anchorNode;
			formatCode = !!node && !!closestTag(node, 'CODE');
			formatHighlight = !!node && !!closestTag(node, 'MARK');
			formatLink = !!node && !!closestTag(node, 'A');
		}
	}

	function closestTag(node: Node, tagName: string): HTMLElement | null {
		let cur: Node | null = node;
		while (cur && cur !== contentEl) {
			if (cur.nodeType === Node.ELEMENT_NODE && (cur as HTMLElement).tagName === tagName) {
				return cur as HTMLElement;
			}
			cur = cur.parentNode;
		}
		return null;
	}

	function positionToolbar() {
		if (!hasRichSelection) { toolbarStyle = ''; return; }
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0) return;
		const range = sel.getRangeAt(0);
		const rect = range.getBoundingClientRect();
		const blockRect = contentEl?.closest('.block')?.getBoundingClientRect();
		if (!blockRect) return;
		const x = rect.left + rect.width / 2 - blockRect.left;
		const y = rect.top - blockRect.top - 46;
		toolbarStyle = `left:${x}px;top:${y}px;`;
	}

	function applyFormat(command: string) {
		if (isLocked || !contentEl) return;
		contentEl.focus();
		restoreSelection();
		document.execCommand(command, false);
		handleInput({ target: contentEl } as unknown as Event);
		refreshSelectionState();
		detectFormats();
		debouncedSave();
	}

	function toggleInlineTag(tagName: string) {
		if (isLocked || !contentEl) return;
		contentEl.focus();
		restoreSelection();

		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return;

		const range = sel.getRangeAt(0);
		const existing = closestTag(range.startContainer, tagName);

		if (existing) {
			// Unwrap: replace the tag with its children
			const parent = existing.parentNode;
			if (parent) {
				while (existing.firstChild) parent.insertBefore(existing.firstChild, existing);
				parent.removeChild(existing);
			}
		} else {
			// Wrap selection in the tag
			const wrapper = document.createElement(tagName);
			try {
				range.surroundContents(wrapper);
			} catch {
				// If surroundContents fails (partial overlap), extract and wrap
				const fragment = range.extractContents();
				wrapper.appendChild(fragment);
				range.insertNode(wrapper);
			}
		}

		handleInput({ target: contentEl } as unknown as Event);
		refreshSelectionState();
		detectFormats();
		debouncedSave();
	}

	function applyLink() {
		if (isLocked || !contentEl) return;
		restoreSelection();

		const sel = window.getSelection();
		const existingLink = sel && sel.anchorNode ? closestTag(sel.anchorNode, 'A') : null;

		if (existingLink) {
			// Edit existing link — prefill with current href
			const currentHref = existingLink.getAttribute('href') || '';
			const href = window.prompt('Edit link URL (clear to remove)', currentHref);
			if (href === null) return; // cancelled
			if (!href.trim()) {
				// Remove the link
				const parent = existingLink.parentNode;
				if (parent) {
					while (existingLink.firstChild) parent.insertBefore(existingLink.firstChild, existingLink);
					parent.removeChild(existingLink);
				}
			} else {
				existingLink.setAttribute('href', href.trim());
			}
		} else {
			const href = window.prompt('Paste link URL');
			if (!href) return;
			contentEl.focus();
			restoreSelection();
			document.execCommand('createLink', false, href.trim());
		}

		handleInput({ target: contentEl } as unknown as Event);
		refreshSelectionState();
		detectFormats();
		debouncedSave();
	}

	/* ---- Markdown auto-convert shortcuts ---- */
	function tryMarkdownShortcut(): boolean {
		if (!contentEl) return false;
		const text = contentEl.textContent || '';
		const patterns: [RegExp, string][] = [
			[/^#\s$/, 'heading'],
			[/^##\s$/, 'heading2'],
			[/^###\s$/, 'heading3'],
			[/^[-*]\s$/, 'bullet'],
			[/^1[.)]\s$/, 'numbered'],
			[/^>\s$/, 'quote'],
		];
		for (const [pattern, blockType] of patterns) {
			if (pattern.test(text)) {
				contentEl.innerHTML = '';
				dispatch('transform', { id, newType: blockType });
				return true;
			}
		}
		return false;
	}

	function handleMouseUp() {
		refreshSelectionState();
		detectFormats();
		positionToolbar();
	}

	function handleKeyUp() {
		refreshSelectionState();
		detectFormats();
		positionToolbar();
	}

	function selectSlashCommand(commandId: string) {
		showSlashMenu = false;
		if (contentEl) {
			contentEl.innerText = '';
		}
		dispatch('transform', { id, newType: commandId });
	}

	function handleDelete() {
		if (isLocked) return;
		dispatch('delete', { id });
	}

	function embedUrlForCurrentBlock(): string {
		if (!pageId || !id) return '';
		const origin = typeof window !== 'undefined' ? window.location.origin : '';
		return `${origin}/embed/${encodeURIComponent(pageId)}/${encodeURIComponent(id)}`;
	}

	function socialShareUrl(platform: 'x' | 'facebook' | 'linkedin', url: string): string {
		const encoded = encodeURIComponent(url);
		if (platform === 'x') return `https://twitter.com/intent/tweet?url=${encoded}`;
		if (platform === 'facebook') return `https://www.facebook.com/sharer/sharer.php?u=${encoded}`;
		return `https://www.linkedin.com/sharing/share-offsite/?url=${encoded}`;
	}

	function toggleShareMenu() {
		if (!pageId || !id) return;
		showShareMenu = !showShareMenu;
	}

	function closeShareMenu() {
		showShareMenu = false;
	}

	async function copyShareLink() {
		const embedUrl = embedUrlForCurrentBlock();
		if (!embedUrl) return;
		const copied = await copyTextToClipboard(embedUrl);
		if (copied) {
			showShareToast = true;
			setTimeout(() => (showShareToast = false), 2000);
			showShareMenu = false;
		}
	}

	async function shareNatively() {
		const embedUrl = embedUrlForCurrentBlock();
		if (!embedUrl || !supportsNativeShare) return;
		try {
			await navigator.share({ url: embedUrl });
			showShareMenu = false;
		} catch {
			// user cancelled or share failed
		}
	}

	function triggerImageUpload() {
		if (isLocked) return;
		imageInputEl?.click();
	}

	async function resolveCurrentImageUrl(url: string) {
		const runId = ++resolveImageRun;
		if (!url) {
			resolvedImageUrl = '';
			return;
		}
		if (!isLocalMediaRef(url)) {
			resolvedImageUrl = url;
			return;
		}
		const objectUrl = await resolveLocalMediaObjectURL(url);
		if (runId !== resolveImageRun) return;
		resolvedImageUrl = objectUrl || '';
	}

	async function resolveCurrentGalleryUrls(items: GalleryItem[]) {
		const runId = ++resolveGalleryRun;
		if (!items.length) {
			galleryImageUrls = {};
			return;
		}
		const entries = await Promise.all(
			items.map(async (item) => {
				if (item.kind !== 'image') return [item.id, item.value] as const;
				if (!isLocalMediaRef(item.value)) return [item.id, item.value] as const;
				const objectUrl = await resolveLocalMediaObjectURL(item.value);
				return [item.id, objectUrl || item.value] as const;
			})
		);
		if (runId !== resolveGalleryRun) return;
		galleryImageUrls = Object.fromEntries(entries);
	}

	async function uploadImageFile(file: File): Promise<string> {
		if (allowLocalMedia && !pageId && !shareToken) {
			return putLocalMediaBlob(file);
		}

		const formData = new FormData();
		formData.append('file', file);

		const encodedPageID = encodeURIComponent(pageId);
		const shareQuery = shareToken ? `?share=${encodeURIComponent(shareToken)}` : '';
		const endpoint = pageId ? `/v1/pages/${encodedPageID}/media/images${shareQuery}` : '/v1/media/images';

		const response = await fetch(`${apiUrl}${endpoint}`, {
			method: 'POST',
			credentials: 'include',
			body: formData
		});

		if (!response.ok) {
			throw new Error('image upload failed');
		}

		const payload = await response.json();
		const url = payload?.url;
		if (typeof url !== 'string' || !url) {
			throw new Error('invalid upload response');
		}

		return url;
	}

	async function handleImageUpload(e: Event) {
		if (isLocked) return;
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		try {
			const url = await uploadImageFile(file);
			dispatch('update', {
				id,
				type,
				data: { ...data, url }
			});
		} catch {
			// ignore upload failure here and keep block unchanged
		}
		input.value = '';
	}

	function triggerGalleryUpload() {
		if (isLocked) return;
		galleryInputEl?.click();
	}

	function makeGalleryItemId() {
		return `g-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	}

	function normalizeGalleryItems(inputData: any): GalleryItem[] {
		if (Array.isArray(inputData?.items)) {
			return inputData.items
				.filter((item: any) => item && (item.kind === 'image' || item.kind === 'text' || item.kind === 'embed') && typeof item.value === 'string')
				.map((item: any, index: number) => ({
					id: typeof item.id === 'string' && item.id ? item.id : `${id}-item-${index}`,
					kind: item.kind,
					value: item.value
				}));
		}

		if (Array.isArray(inputData?.images)) {
			return inputData.images
				.filter((src: any) => typeof src === 'string' && src)
				.map((src: string, index: number) => ({ id: `${id}-img-${index}`, kind: 'image', value: src }));
		}

		return [];
	}

	function updateGalleryItems(items: GalleryItem[], columns?: number) {
		dispatch('update', {
			id,
			type,
			data: {
				...data,
				items,
				columns: columns || data?.columns || 2
			}
		});
	}

	async function uploadImages(files: File[]) {
		const uploads = await Promise.allSettled(files.map((file) => uploadImageFile(file)));
		return uploads
			.filter((result): result is PromiseFulfilledResult<string> => result.status === 'fulfilled')
			.map((result) => result.value)
			.filter(Boolean);
	}

	async function handleGalleryUpload(e: Event) {
		if (isLocked) return;
		const input = e.target as HTMLInputElement;
		const files = Array.from(input.files || []);
		if (files.length === 0) return;

		try {
			const urls = await uploadImages(files);
			const existing = normalizeGalleryItems(data);
			const appended = urls.filter(Boolean).map((src) => ({
				id: makeGalleryItemId(),
				kind: 'image' as const,
				value: src
			}));
			updateGalleryItems([...existing, ...appended]);
		} finally {
			input.value = '';
		}
	}

	async function handleMediaDrop(e: DragEvent) {
		if (isLocked) return;
		e.preventDefault();
		e.stopPropagation();

		const files = Array.from(e.dataTransfer?.files || []).filter((file) => file.type.startsWith('image/'));
		if (files.length > 0) {
			const urls = await uploadImages(files);
			if (type === 'gallery') {
				const existing = normalizeGalleryItems(data);
				const appended = urls.filter(Boolean).map((src) => ({
					id: makeGalleryItemId(),
					kind: 'image' as const,
					value: src
				}));
				updateGalleryItems([...existing, ...appended]);
			} else {
				const existing = data?.url ? [data.url] : [];
				const items = [...existing, ...urls].map((src) => ({
					id: makeGalleryItemId(),
					kind: 'image' as const,
					value: src
				}));
				dispatch('update', {
					id,
					type: 'gallery',
					data: {
						items,
						columns: 2
					}
				});
			}
			return;
		}

		dispatch('mergeToGallery', { targetId: id });
	}

	function setGalleryColumns(columns: number) {
		if (isLocked) return;
		updateGalleryItems(normalizeGalleryItems(data), columns);
	}

	function removeGalleryItem(itemId: string) {
		if (isLocked) return;
		const items = normalizeGalleryItems(data);
		updateGalleryItems(items.filter((item) => item.id !== itemId));
	}

	function handleGalleryItemDragStart(e: DragEvent, item: GalleryItem) {
		if (isLocked) return;
		e.stopPropagation();
		e.dataTransfer?.setData(
			'application/x-jot-gallery-card',
			JSON.stringify({ sourceBlockId: id, itemId: item.id })
		);
		e.dataTransfer?.setData('text/plain', item.kind === 'text' ? item.value : '[image]');
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
		}
		dispatch('galleryCardDragStart');
	}

	function extractEmbedUrl(rawValue: string) {
		const value = (rawValue || '').trim();
		if (!value) return '';

		if (value.startsWith('<')) {
			try {
				const doc = new DOMParser().parseFromString(value, 'text/html');
				const iframe = doc.querySelector('iframe[src]');
				const iframeSrc = iframe?.getAttribute('src')?.trim();
				if (iframeSrc) return iframeSrc;
			} catch {
				// fall back to regex parsing below
			}

			const srcMatch = value.match(/<iframe[^>]*\bsrc\s*=\s*(['"]?)([^'"\s>]+)\1/i);
			if (srcMatch?.[2]) {
				return srcMatch[2].trim();
			}
		}

		return value;
	}

	function handleEmbedInputChange(e: Event) {
		if (isLocked) return;
		const target = e.currentTarget as HTMLInputElement;
		const url = extractEmbedUrl(target.value);
		dispatch('update', { id, type, data: { url } });
	}

	function addGalleryEmbed() {
		if (isLocked) return;
		const raw = window.prompt('Paste embed URL or iframe HTML');
		if (!raw) return;
		const url = extractEmbedUrl(raw);
		if (!url) return;

		const existing = normalizeGalleryItems(data);
		updateGalleryItems([
			...existing,
			{ id: makeGalleryItemId(), kind: 'embed', value: url }
		]);
	}

	/* ---- Code block ---- */
	let codeText = data?.code || '';
	let codeLang = data?.language || 'javascript';
	let highlightedCodeHtml = '';
	let codeTextareaEl: HTMLTextAreaElement;
	let codeHighlightEl: HTMLPreElement;
	const CODE_LANGUAGES = ['javascript', 'typescript', 'html', 'css', 'python', 'go', 'rust', 'json', 'sql', 'bash', 'markdown'];

	function normalizeHighlightLanguage(lang: string): string {
		const lower = String(lang || '').toLowerCase();
		return lower === 'bash' ? 'bash' : lower;
	}

	function escapeHtml(value: string): string {
		return value
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
	}

	function getHighlightedCodeHtml(code: string, lang: string): string {
		const safeCode = String(code || '');
		if (!safeCode.trim()) return ' ';

		try {
			const preferred = normalizeHighlightLanguage(lang);
			if (preferred && hljs.getLanguage(preferred)) {
				return hljs.highlight(safeCode, { language: preferred, ignoreIllegals: true }).value;
			}
			return hljs.highlightAuto(safeCode).value;
		} catch {
			return escapeHtml(safeCode);
		}
	}

	function syncCodeScroll() {
		if (!codeTextareaEl || !codeHighlightEl) return;
		codeHighlightEl.scrollTop = codeTextareaEl.scrollTop;
		codeHighlightEl.scrollLeft = codeTextareaEl.scrollLeft;
	}

	function handleCodeInput(e: Event) {
		if (isLocked) return;
		const target = e.target as HTMLTextAreaElement;
		codeText = target.value;
		dispatch('update', { id, type, data: { ...data, code: codeText, language: codeLang } });
	}

	function handleCodeLang(e: Event) {
		if (isLocked) return;
		codeLang = (e.target as HTMLSelectElement).value;
		dispatch('update', { id, type, data: { ...data, code: codeText, language: codeLang } });
	}

	function handleCodeKeydown(e: KeyboardEvent) {
		if (e.key === 'Tab') {
			e.preventDefault();
			const ta = e.target as HTMLTextAreaElement;
			const start = ta.selectionStart;
			const end = ta.selectionEnd;
			codeText = codeText.substring(0, start) + '  ' + codeText.substring(end);
			ta.value = codeText;
			ta.selectionStart = ta.selectionEnd = start + 2;
			dispatch('update', { id, type, data: { ...data, code: codeText, language: codeLang } });
		}
	}

	/* ---- Canvas block ---- */
	let canvasCode = data?.code || 'const ctx = canvas.getContext("2d");\nctx.fillStyle = "#7c5cff";\nctx.fillRect(10, 10, 100, 80);';
	let canvasEl: HTMLCanvasElement;
	let canvasError = '';
	let canvasWidth = Number(data?.width) || 600;
	let canvasHeight = Number(data?.height) || 400;
	let canvasRunning = false;
	let canvasRafId = 0;

	function stopCanvas() {
		if (canvasRafId) cancelAnimationFrame(canvasRafId);
		canvasRafId = 0;
		canvasRunning = false;
	}

	function runCanvas() {
		stopCanvas();
		canvasError = '';
		if (!canvasEl) return;
		const ctx = canvasEl.getContext('2d');
		if (!ctx) return;
		ctx.clearRect(0, 0, canvasEl.width, canvasEl.height);

		// Build a sandboxed loop: user code gets `loop(fn)` to register a per-frame callback
		let loopFn: ((t: number) => void) | null = null;
		const loop = (fn: (t: number) => void) => { loopFn = fn; };

		try {
			const fn = new Function('canvas', 'ctx', 'loop', canvasCode);
			fn(canvasEl, ctx, loop);
		} catch (err: any) {
			canvasError = err?.message || String(err);
			return;
		}

		if (loopFn) {
			canvasRunning = true;
			const userLoop: (t: number) => void = loopFn;
			const tick = (t: number) => {
				if (!canvasRunning) return;
				try {
					userLoop(t);
				} catch (err: any) {
					canvasError = err?.message || String(err);
					stopCanvas();
					return;
				}
				canvasRafId = requestAnimationFrame(tick);
			};
			canvasRafId = requestAnimationFrame(tick);
		}
	}

	function handleCanvasCodeInput(e: Event) {
		if (isLocked) return;
		const target = e.target as HTMLTextAreaElement;
		canvasCode = target.value;
		dispatch('update', { id, type, data: { ...data, code: canvasCode, width: canvasWidth, height: canvasHeight } });
	}

	function handleCanvasCodeKeydown(e: KeyboardEvent) {
		if (e.key === 'Tab') {
			e.preventDefault();
			const ta = e.target as HTMLTextAreaElement;
			const start = ta.selectionStart;
			const end = ta.selectionEnd;
			canvasCode = canvasCode.substring(0, start) + '  ' + canvasCode.substring(end);
			ta.value = canvasCode;
			ta.selectionStart = ta.selectionEnd = start + 2;
			dispatch('update', { id, type, data: { ...data, code: canvasCode, width: canvasWidth, height: canvasHeight } });
		}
	}

	function handleCanvasResize(dimension: 'width' | 'height', e: Event) {
		if (isLocked) return;
		const val = Math.max(100, Math.min(2000, Number((e.target as HTMLInputElement).value) || 600));
		if (dimension === 'width') canvasWidth = val;
		else canvasHeight = val;
		dispatch('update', { id, type, data: { ...data, code: canvasCode, width: canvasWidth, height: canvasHeight } });
	}

	function handleCaptionInput(e: Event) {
		if (isLocked) return;
		const caption = (e.target as HTMLInputElement).value;
		dispatch('update', { id, type, data: { ...data, caption } });
	}

	$: if (type === 'code') {
		codeText = data?.code ?? codeText;
		codeLang = data?.language ?? codeLang;
	}
	$: highlightedCodeHtml = type === 'code' ? getHighlightedCodeHtml(codeText, codeLang) : '';
	$: if (type === 'canvas') {
		canvasCode = data?.code ?? canvasCode;
		canvasWidth = Number(data?.width) || canvasWidth;
		canvasHeight = Number(data?.height) || canvasHeight;
	}

	export function focus() {
		tick().then(() => {
			contentEl?.focus();
			// Place cursor at end
			if (contentEl) {
				const range = document.createRange();
				const sel = window.getSelection();
				range.selectNodeContents(contentEl);
				range.collapse(false);
				sel?.removeAllRanges();
				sel?.addRange(range);
			}
		});
	}

	onMount(() => {
		supportsNativeShare = typeof navigator !== 'undefined' && typeof navigator.share === 'function';
		if (localText === '') {
			focus();
		}
		const listener = () => refreshSelectionState();
		document.addEventListener('selectionchange', listener);
		const handleDocumentClick = (event: MouseEvent) => {
			if (!showShareMenu) return;
			const target = event.target as Node;
			if (shareMenuEl?.contains(target)) return;
			if (shareBtnEl?.contains(target)) return;
			showShareMenu = false;
		};
		document.addEventListener('mousedown', handleDocumentClick);
		return () => {
			document.removeEventListener('selectionchange', listener);
			document.removeEventListener('mousedown', handleDocumentClick);
		};
	});

	onDestroy(() => {
		clearTimeout(saveTimeout);
		stopCanvas();
	});

	function handleHandleClick(e: MouseEvent) {
		if (isLocked) return;
		const btn = e.currentTarget as HTMLElement;
		const rect = btn.getBoundingClientRect();
		setSlashMenuPosition(rect);
		showSlashMenu = !showSlashMenu;
		selectedMenuIndex = 0;
	}

	function setSlashMenuPosition(anchorRect: DOMRect) {
		const margin = 12;
		const gap = 8;
		const estimatedMenuHeight = 320;
		const estimatedMenuWidth = Math.min(320, window.innerWidth - margin * 2);

		const canOpenBelow = anchorRect.bottom + gap + estimatedMenuHeight <= window.innerHeight - margin;
		const canOpenAbove = anchorRect.top - gap - estimatedMenuHeight >= margin;
		slashMenuPlacement = !canOpenBelow && canOpenAbove ? 'above' : 'below';

		const rawX = anchorRect.left;
		const maxX = Math.max(margin, window.innerWidth - estimatedMenuWidth - margin);
		const clampedX = Math.min(Math.max(rawX, margin), maxX);

		const y = slashMenuPlacement === 'above'
			? Math.max(margin, anchorRect.top - gap)
			: Math.min(window.innerHeight - margin, anchorRect.bottom + gap);

		slashMenuPosition = { x: clampedX, y };
	}
</script>

<div class="block" class:dragging={isDragging} class:locked={isLocked} data-block-id={id}>
	<button type="button" class="block-handle" draggable="true" aria-label="Drag or click for menu" on:dragstart={() => dispatch('dragstart', { id })} on:click={handleHandleClick}>
		<span class="handle-icon">⋮⋮</span>
	</button>

	<div class="block-content">
		{#if isRichTextType && isEditingText && hasRichSelection && !isLocked}
			<div class="rich-toolbar" role="toolbar" aria-label="Rich text toolbar" style={toolbarStyle}>
				<button type="button" class="rich-btn" class:active={formatBold} title="Bold (⌘B)" on:mousedown|preventDefault on:click={() => applyFormat('bold')}><strong>B</strong></button>
				<button type="button" class="rich-btn" class:active={formatItalic} title="Italic (⌘I)" on:mousedown|preventDefault on:click={() => applyFormat('italic')}><em>I</em></button>
				<button type="button" class="rich-btn" class:active={formatUnderline} title="Underline (⌘U)" on:mousedown|preventDefault on:click={() => applyFormat('underline')}><u>U</u></button>
				<button type="button" class="rich-btn" class:active={formatStrike} title="Strikethrough" on:mousedown|preventDefault on:click={() => applyFormat('strikeThrough')}><s>S</s></button>
				<span class="rich-sep"></span>
				<button type="button" class="rich-btn mono" class:active={formatCode} title="Inline code (⌘E)" on:mousedown|preventDefault on:click={() => toggleInlineTag('CODE')}>⟨⟩</button>
				<button type="button" class="rich-btn" class:active={formatHighlight} title="Highlight (⌘⇧H)" on:mousedown|preventDefault on:click={() => toggleInlineTag('MARK')}>
					<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4z"/></svg>
				</button>
				<span class="rich-sep"></span>
				<button type="button" class="rich-btn" class:active={formatLink} title="Link (⌘K)" on:mousedown|preventDefault on:click={applyLink}>
					<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg>
				</button>
				<button type="button" class="rich-btn" title="Clear formatting" on:mousedown|preventDefault on:click={() => applyFormat('removeFormat')}>
					<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
				</button>
			</div>
		{/if}
		{#if isLocked && lockOwner}
			<div class="typing-lock">{lockOwner.userName} is typing…</div>
			<div class="lock-overlay" aria-hidden="true"></div>
		{/if}
		{#if type === 'heading'}
			<h1
				bind:this={contentEl}
				bind:innerHTML={localHtml}
				class="editable heading-1"
				contenteditable="true"
				on:input={handleInput}
				on:focus={handleFocus}
				on:mouseup={handleMouseUp}
				on:keyup={handleKeyUp}
				on:keydown={handleKeydown}
				on:blur={handleBlur}
				data-placeholder="Heading 1"
			></h1>
		{:else if type === 'heading2'}
			<h2
				bind:this={contentEl}
				bind:innerHTML={localHtml}
				class="editable heading-2"
				contenteditable="true"
				on:input={handleInput}
				on:focus={handleFocus}
				on:mouseup={handleMouseUp}
				on:keyup={handleKeyUp}
				on:keydown={handleKeydown}
				on:blur={handleBlur}
				data-placeholder="Heading 2"
			></h2>
		{:else if type === 'heading3'}
			<h3
				bind:this={contentEl}
				bind:innerHTML={localHtml}
				class="editable heading-3"
				contenteditable="true"
				on:input={handleInput}
				on:focus={handleFocus}
				on:mouseup={handleMouseUp}
				on:keyup={handleKeyUp}
				on:keydown={handleKeydown}
				on:blur={handleBlur}
				data-placeholder="Heading 3"
			></h3>
		{:else if type === 'bullet'}
			<div class="list-block">
				<span class="bullet">•</span>
				<div
					bind:this={contentEl}
					bind:innerHTML={localHtml}
					class="editable"
					contenteditable="true"
					role="textbox"
					aria-multiline="true"
					tabindex="0"
					on:input={handleInput}
					on:focus={handleFocus}
					on:mouseup={handleMouseUp}
					on:keyup={handleKeyUp}
					on:keydown={handleKeydown}
					on:blur={handleBlur}
					data-placeholder="List item"
				></div>
			</div>
		{:else if type === 'numbered'}
			<div class="list-block">
				<span class="number">{listNumber}.</span>
				<div
					bind:this={contentEl}
					bind:innerHTML={localHtml}
					class="editable"
					contenteditable="true"
					role="textbox"
					aria-multiline="true"
					tabindex="0"
					on:input={handleInput}
					on:focus={handleFocus}
					on:mouseup={handleMouseUp}
					on:keyup={handleKeyUp}
					on:keydown={handleKeydown}
					on:blur={handleBlur}
					data-placeholder="List item"
				></div>
			</div>
		{:else if type === 'quote'}
			<blockquote
				bind:this={contentEl}
				bind:innerHTML={localHtml}
				class="editable quote"
				contenteditable="true"
				on:input={handleInput}
				on:focus={handleFocus}
				on:mouseup={handleMouseUp}
				on:keyup={handleKeyUp}
				on:keydown={handleKeydown}
				on:blur={handleBlur}
				data-placeholder="Quote"
			></blockquote>
		{:else if type === 'image'}
			{#if data?.url}
				<figure class="media-figure">
					<button
						type="button"
						class="image-hit"
						on:click={triggerImageUpload}
						on:dragover|preventDefault|stopPropagation={() => {}}
						on:drop={handleMediaDrop}
					>
						<img
							src={resolvedImageUrl || (isLocalMediaRef(data?.url) ? '' : data.url)}
							alt={data.caption || 'block'}
							class="block-image"
						/>
					</button>
					<figcaption class="media-caption">
						<input
							type="text"
							class="caption-input"
							placeholder="Add a caption…"
							value={data.caption || ''}
							on:input={handleCaptionInput}
							on:keydown|stopPropagation
						/>
					</figcaption>
				</figure>
			{:else}
				<div>
					<button
						type="button"
						class="image-placeholder"
						on:click={triggerImageUpload}
						on:dragover|preventDefault|stopPropagation={() => {}}
						on:drop={handleMediaDrop}
					>
						<span>🖼</span>
						<span>Click to add image</span>
					</button>
					{#if allowLocalMedia && !pageId && !shareToken}
						<div class="local-media-hint">Stored locally until publish</div>
					{/if}
				</div>
			{/if}
		{:else if type === 'gallery'}
			{@const items = normalizeGalleryItems(data)}
			{@const columns = Math.min(Math.max(Number(data?.columns || 2), 2), 4)}
			<div class="gallery-block" role="region" aria-label="Gallery block" on:dragover|preventDefault|stopPropagation={() => {}} on:drop={handleMediaDrop}>
				<div class="gallery-toolbar">
					<div class="gallery-actions">
						<button class="gallery-btn" on:click={triggerGalleryUpload}>Add images</button>
						<button class="gallery-btn" on:click={addGalleryEmbed}>Add embed</button>
					</div>
					<div class="gallery-columns">
						{#each [2, 3, 4] as col}
							<button class="gallery-btn" class:selected={columns === col} on:click={() => setGalleryColumns(col)}>{col} cols</button>
						{/each}
					</div>
				</div>

				{#if items.length === 0}
					<div>
						<button type="button" class="image-placeholder" on:click={triggerGalleryUpload}>
							<span>🖼</span>
							<span>Add images or drag text/image blocks here</span>
						</button>
						{#if allowLocalMedia && !pageId && !shareToken}
							<div class="local-media-hint">Stored locally until publish</div>
						{/if}
					</div>
				{:else}
					<div class="gallery-grid" style="--gallery-cols: {columns};">
						{#each items as item, i (item.id)}
							<div class="gallery-item" role="group" aria-label={`Gallery card ${i + 1}`} class:text-card={item.kind === 'text'} draggable="true" on:dragstart={(e) => handleGalleryItemDragStart(e, item)}>
								{#if item.kind === 'image'}
										<img src={galleryImageUrls[item.id] || (isLocalMediaRef(item.value) ? '' : item.value)} alt={`gallery-${i}`} class="gallery-image" />
								{:else if item.kind === 'embed'}
									<iframe src={item.value} title={`gallery-embed-${i}`} class="gallery-embed"></iframe>
								{:else}
									<div class="gallery-text">{item.value}</div>
								{/if}
								<button class="gallery-remove" on:click={() => removeGalleryItem(item.id)}>✕</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{:else if type === 'embed'}
			{#if data?.url}
				<figure class="media-figure" on:dragover|preventDefault|stopPropagation={() => {}} on:drop={handleMediaDrop}>
					<iframe src={data.url} class="embed-frame" title="Embedded content"></iframe>
					<figcaption class="media-caption">
						<input
							type="text"
							class="caption-input"
							placeholder="Add a caption…"
							value={data.caption || ''}
							on:input={handleCaptionInput}
							on:keydown|stopPropagation
						/>
					</figcaption>
				</figure>
			{:else}
				<div class="embed-placeholder">
					<input
						type="text"
						placeholder="Paste embed URL..."
						on:change={handleEmbedInputChange}
					/>
				</div>
			{/if}
		{:else if type === 'code'}
			<div class="code-block">
				<div class="code-toolbar">
					<select class="code-lang-select" value={codeLang} on:change={handleCodeLang}>
						{#each CODE_LANGUAGES as lang}
							<option value={lang}>{lang}</option>
						{/each}
					</select>
					<span class="code-label">Code</span>
				</div>
				<div class="code-editor-shell">
					<pre bind:this={codeHighlightEl} class="code-highlight-layer" aria-hidden="true"><code class={`hljs language-${codeLang}`}>{@html highlightedCodeHtml}</code></pre>
					<textarea
						bind:this={codeTextareaEl}
						class="code-editor code-editor-overlay"
						spellcheck="false"
						autocomplete="off"
						autocapitalize="off"
						wrap="off"
						value={codeText}
						placeholder="Write your code here..."
						on:input={handleCodeInput}
						on:keydown={handleCodeKeydown}
						on:scroll={syncCodeScroll}
					></textarea>
				</div>
			</div>
		{:else if type === 'canvas'}
			<div class="canvas-block">
				<div class="canvas-toolbar">
					<span class="canvas-label">Canvas JS</span>
					<div class="canvas-dims">
						<input type="number" class="canvas-dim-input" value={canvasWidth} min="100" max="2000" on:change={(e) => handleCanvasResize('width', e)} title="Width" />
						<span class="canvas-dim-x">×</span>
						<input type="number" class="canvas-dim-input" value={canvasHeight} min="100" max="2000" on:change={(e) => handleCanvasResize('height', e)} title="Height" />
					</div>
					{#if canvasRunning}
						<button class="canvas-stop-btn" on:click={stopCanvas}>■ Stop</button>
					{:else}
						<button class="canvas-run-btn" on:click={runCanvas}>▶ Run</button>
					{/if}
				</div>
				<textarea
					class="code-editor canvas-code"
					spellcheck="false"
					autocomplete="off"
					autocapitalize="off"
					wrap="off"
					value={canvasCode}
					placeholder={'const ctx = canvas.getContext("2d");\nctx.fillStyle = "#7c5cff";\nctx.fillRect(10, 10, 100, 80);'}
					on:input={handleCanvasCodeInput}
					on:keydown={handleCanvasCodeKeydown}
				></textarea>
				{#if canvasError}
					<div class="canvas-error">{canvasError}</div>
				{/if}
				<div class="canvas-preview">
					<canvas bind:this={canvasEl} width={canvasWidth} height={canvasHeight} class="canvas-el"></canvas>
				</div>
				<div class="media-caption canvas-caption">
					<input
						type="text"
						class="caption-input"
						placeholder="Add a caption…"
						value={data.caption || ''}
						on:input={handleCaptionInput}
						on:keydown|stopPropagation
					/>
				</div>
			</div>
		{:else if type === 'music'}
			<div class="music-block-wrap">
				<MusicPlayer
					url={data?.url || ''}
					title={data?.title || ''}
					artist={data?.artist || ''}
					coverUrl={data?.coverUrl || ''}
					{apiUrl}
					{pageId}
					{shareToken}
					{allowLocalMedia}
					on:change={(e) => dispatch('update', { id, type, data: e.detail })}
				/>
			</div>
		{:else}
			<div
				bind:this={contentEl}
				bind:innerHTML={localHtml}
				class="editable"
				contenteditable="true"
				role="textbox"
				aria-multiline="true"
				tabindex="0"
				on:input={handleInput}
				on:focus={handleFocus}
				on:mouseup={handleMouseUp}
				on:keyup={handleKeyUp}
				on:keydown={handleKeydown}
				on:blur={handleBlur}
				data-placeholder="Type '/' for commands..."
			></div>
		{/if}
	</div>

	<input bind:this={imageInputEl} type="file" accept="image/*" class="image-input" on:change={handleImageUpload} />
	<input bind:this={galleryInputEl} type="file" accept="image/*" class="image-input" multiple on:change={handleGalleryUpload} />

	{#if published && pageId}
		<button bind:this={shareBtnEl} class="share-btn" title={showShareToast ? 'Copied!' : 'Share block'} on:click|stopPropagation={toggleShareMenu}>
			{#if showShareToast}
				<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>
			{:else}
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M4 12v8a2 2 0 002 2h12a2 2 0 002-2v-8"/>
					<polyline points="16 6 12 2 8 6"/>
					<line x1="12" y1="2" x2="12" y2="15"/>
				</svg>
			{/if}
		</button>
		{#if showShareMenu}
			<div bind:this={shareMenuEl} class="share-menu" role="menu" aria-label="Share block menu">
				<button type="button" class="share-menu-item" on:click={copyShareLink}>Copy link</button>
				{#if supportsNativeShare}
					<button type="button" class="share-menu-item" on:click={shareNatively}>Share…</button>
				{/if}
				{#if embedUrlForCurrentBlock()}
					<a class="share-menu-item" href={socialShareUrl('x', embedUrlForCurrentBlock())} target="_blank" rel="noreferrer">Share to X</a>
					<a class="share-menu-item" href={socialShareUrl('facebook', embedUrlForCurrentBlock())} target="_blank" rel="noreferrer">Share to Facebook</a>
					<a class="share-menu-item" href={socialShareUrl('linkedin', embedUrlForCurrentBlock())} target="_blank" rel="noreferrer">Share to LinkedIn</a>
				{/if}
			</div>
		{/if}
	{/if}
	<button class="delete-btn" title="Delete" on:click={handleDelete}>✕</button>
</div>

{#if showSlashMenu}
	<div class="slash-menu" class:above={slashMenuPlacement === 'above'} style="left: {slashMenuPosition.x}px; top: {slashMenuPosition.y}px;">
		<div class="slash-menu-header">Basic blocks</div>
		{#each slashCommands as cmd, i (cmd.id)}
			<button
				class="slash-item"
				class:selected={i === selectedMenuIndex}
				on:click={() => selectSlashCommand(cmd.id)}
				on:mouseenter={() => (selectedMenuIndex = i)}
			>
				<span class="slash-icon">{cmd.icon}</span>
				<div class="slash-info">
					<span class="slash-label">{cmd.label}</span>
					<span class="slash-desc">{cmd.description}</span>
				</div>
			</button>
		{/each}
	</div>
{/if}

<style>
	.block {
		position: relative;
		display: flex;
		align-items: flex-start;
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
		padding: 8px 10px;
		border-radius: 10px;
		border: 1px solid transparent;
		transition: background 0.12s, border-color 0.12s, box-shadow 0.12s;
	}

	.block:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 10%, var(--note-surface, #ffffff));
		border-color: color-mix(in srgb, var(--note-border, #d1d5db) 85%, transparent);
		box-shadow: 0 10px 26px rgba(15, 23, 42, 0.08);
	}

	.block:hover .block-handle,
	.block:hover .delete-btn,
	.block:hover .share-btn {
		opacity: 1;
	}

	.block.dragging {
		opacity: 0.4;
	}

	.block-handle {
		opacity: 0;
		cursor: grab;
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #d1d5db);
		padding: 4px;
		border-radius: 8px;
		color: var(--note-muted, #9ca3af);
		transition: opacity 0.15s, background 0.15s;
		user-select: none;
		position: absolute;
		left: -28px;
		top: 4px;
		flex-shrink: 0;
	}

	.block-handle:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 24%, #ffffff);
		color: var(--note-text, #6b7280);
	}

	.block-handle:active {
		cursor: grabbing;
	}

	.handle-icon {
		font-size: 14px;
		letter-spacing: -2px;
	}

	.block-content {
		flex: 1;
		min-width: 0;
		width: 100%;
		position: relative;
	}

	.rich-toolbar {
		position: absolute;
		z-index: 50;
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 4px 6px;
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 10px;
		background: var(--note-surface, #ffffff);
		box-shadow: 0 8px 30px rgba(15, 23, 42, 0.18), 0 1px 3px rgba(0,0,0,0.06);
		transform: translateX(-50%);
		white-space: nowrap;
		animation: toolbar-in 0.12s ease-out;
	}

	@keyframes toolbar-in {
		from { opacity: 0; transform: translateX(-50%) translateY(4px) scale(0.96); }
		to   { opacity: 1; transform: translateX(-50%) translateY(0) scale(1); }
	}

	.rich-sep {
		width: 1px;
		height: 18px;
		background: var(--note-border, #e5e7eb);
		margin: 0 2px;
		flex-shrink: 0;
	}

	.rich-btn {
		border: none;
		background: transparent;
		color: var(--note-text, #1f2328);
		border-radius: 6px;
		padding: 4px 7px;
		font-size: 13px;
		cursor: pointer;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 28px;
		height: 28px;
		transition: background 0.1s, color 0.1s;
		line-height: 1;
	}

	.rich-btn:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 12%, transparent);
	}

	.rich-btn.active {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 20%, transparent);
		color: var(--note-accent, #7c5cff);
	}

	.rich-btn.mono {
		font-family: 'JetBrains Mono', 'Fira Code', 'SF Mono', monospace;
		font-size: 14px;
		font-weight: 600;
		letter-spacing: -1px;
	}

	.rich-btn svg {
		flex-shrink: 0;
	}

	.editable {
		outline: none;
		min-height: 1.5em;
		line-height: 1.6;
		word-break: break-word;
		color: var(--note-text, #1f2328);
	}

	.editable:empty::before {
		content: attr(data-placeholder);
		color: color-mix(in srgb, var(--note-muted, #6b7280) 70%, #ffffff);
		pointer-events: none;
	}

	.heading-1 {
		font-size: 30px;
		font-weight: 700;
		line-height: 1.3;
		margin: 12px 0 4px;
		color: var(--note-title, #111827);
	}

	.heading-2 {
		font-size: 24px;
		font-weight: 600;
		line-height: 1.3;
		margin: 10px 0 4px;
		color: var(--note-title, #111827);
	}

	.heading-3 {
		font-size: 20px;
		font-weight: 600;
		line-height: 1.3;
		margin: 8px 0 4px;
		color: var(--note-title, #111827);
	}

	.list-block {
		display: flex;
		align-items: flex-start;
		gap: 8px;
	}

	.bullet, .number {
		color: var(--note-muted, #6b7280);
		flex-shrink: 0;
		padding-top: 2px;
	}

	.quote {
		border-left: 3px solid var(--note-accent, #7c5cff);
		padding-left: 16px;
		margin: 4px 0;
		color: var(--note-text, #1f2328);
		font-style: italic;
	}



	.block-image {
		max-width: 100%;
		border-radius: 4px;
		margin: 8px 0;
		cursor: pointer;
	}

	.image-hit {
		background: transparent;
		border: none;
		padding: 0;
		width: 100%;
		text-align: left;
	}

	.image-input {
		display: none;
	}

	.local-media-hint {
		margin-top: 8px;
		font-size: 10px;
		font-weight: 600;
		letter-spacing: 0.04em;
		opacity: 0.7;
		color: var(--note-muted, #6b7280);
	}

	/* ---- Media figure + caption ---- */
	.media-figure {
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 100%;
	}

	.media-caption {
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 6px 0 2px;
		position: relative;
	}

	.media-caption::before {
		content: '';
		position: absolute;
		top: 0;
		left: 50%;
		transform: translateX(-50%);
		width: 32px;
		height: 2px;
		border-radius: 1px;
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 24%, transparent);
		transition: width 0.2s ease;
	}

	.media-caption:focus-within::before {
		width: 64px;
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 50%, transparent);
	}

	.caption-input {
		width: 100%;
		max-width: 480px;
		text-align: center;
		border: none;
		background: transparent;
		color: var(--note-muted, #6b7280);
		font-size: 13px;
		font-style: italic;
		font-family: inherit;
		line-height: 1.5;
		padding: 4px 8px;
		outline: none;
		transition: color 0.15s, border-color 0.15s;
	}

	.caption-input:focus {
		color: var(--note-text, #1f2328);
	}

	.caption-input::placeholder {
		color: color-mix(in srgb, var(--note-muted, #9ca3af) 60%, transparent);
		font-style: italic;
	}

	.canvas-caption {
		padding: 8px 12px 6px;
		background: #1e1e2e;
		border-top: 1px solid #313244;
	}

	.canvas-caption::before {
		background: color-mix(in srgb, #cba6f7 30%, transparent);
	}

	.canvas-caption:focus-within::before {
		background: color-mix(in srgb, #cba6f7 60%, transparent);
	}

	.canvas-caption .caption-input {
		color: #6c7086;
		font-size: 12px;
	}

	.canvas-caption .caption-input:focus {
		color: #cdd6f4;
	}

	.canvas-caption .caption-input::placeholder {
		color: #45475a;
	}

	.gallery-block {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.gallery-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
	}

	.gallery-columns {
		display: flex;
		gap: 6px;
	}

	.gallery-actions {
		display: flex;
		gap: 6px;
	}

	.gallery-btn {
		background: transparent;
		border: none;
		border-radius: 0;
		padding: 0 2px 2px;
		font-size: 14px;
		cursor: pointer;
		color: var(--note-text, #1f2328);
		font-weight: 700;
		text-decoration: underline;
		text-decoration-thickness: 2px;
		text-underline-offset: 6px;
	}

	.gallery-btn.selected {
		color: color-mix(in srgb, var(--note-accent, #7c5cff) 70%, var(--note-text, #1f2328));
	}

	.gallery-grid {
		display: grid;
		grid-template-columns: repeat(var(--gallery-cols, 2), minmax(0, 1fr));
		gap: 8px;
	}

	.gallery-item {
		position: relative;
		overflow: hidden;
		border-radius: 6px;
		background: #f3f4f6;
		border: 1px solid color-mix(in srgb, var(--note-border, #d1d5db) 85%, transparent);
		cursor: grab;
	}

	.gallery-item:active {
		cursor: grabbing;
	}

	.gallery-item.text-card {
		background: color-mix(in srgb, var(--note-surface, #ffffff) 88%, var(--note-accent, #7c5cff) 10%);
	}

	.gallery-image {
		width: 100%;
		height: 180px;
		object-fit: cover;
		display: block;
	}

	.gallery-embed {
		width: 100%;
		height: 180px;
		border: none;
		display: block;
		background: #000;
	}

	.gallery-text {
		min-height: 120px;
		padding: 14px;
		line-height: 1.45;
		font-size: 14px;
		color: var(--note-text, #1f2328);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.gallery-remove {
		position: absolute;
		top: 6px;
		right: 6px;
		width: 22px;
		height: 22px;
		border: none;
		border-radius: 999px;
		background: rgba(0, 0, 0, 0.55);
		color: #fff;
		cursor: pointer;
	}

	.image-placeholder,
	.embed-placeholder {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 16px;
		background: color-mix(in srgb, var(--note-surface, #ffffff) 82%, var(--note-accent, #7c5cff) 18%);
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 12px;
		color: var(--note-text, #1f2328);
		cursor: pointer;
		transition: background 0.15s;
	}

	.image-placeholder:hover,
	.embed-placeholder:hover {
		background: color-mix(in srgb, var(--note-surface, #ffffff) 78%, var(--note-accent, #7c5cff) 10%);
	}

	.embed-placeholder input {
		flex: 1;
		border: none;
		background: transparent;
		color: var(--note-text, #1f2328);
		font-size: 14px;
		outline: none;
	}

	.embed-placeholder input::placeholder {
		color: var(--note-muted, #6b7280);
	}

	.embed-frame {
		width: 100%;
		height: 400px;
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 12px;
		background: #000;
	}

	.delete-btn {
		opacity: 0;
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #d1d5db);
		padding: 4px 8px;
		border-radius: 8px;
		color: var(--note-muted, #9ca3af);
		cursor: pointer;
		transition: opacity 0.15s, background 0.15s, color 0.15s;
	}

	.delete-btn:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 14%, transparent);
		color: var(--note-title, #ef4444);
	}

	.share-btn {
		opacity: 0;
		position: absolute;
		right: -28px;
		top: 28px;
		width: 26px;
		height: 26px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 8px;
		color: var(--note-muted, #9ca3af);
		cursor: pointer;
		transition: opacity 0.15s, background 0.15s, color 0.15s;
		padding: 0;
	}

	.share-btn svg {
		width: 13px;
		height: 13px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.share-btn:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 18%, var(--note-surface, #ffffff));
		color: var(--note-accent, #7c5cff);
		border-color: var(--note-accent, #7c5cff);
	}

	.share-menu {
		position: absolute;
		right: -32px;
		top: 56px;
		z-index: 40;
		min-width: 168px;
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		box-shadow: 6px 6px 0 #1a1a1a;
		overflow: hidden;
	}

	.share-menu-item {
		display: block;
		width: 100%;
		padding: 8px 10px;
		text-align: left;
		font: inherit;
		font-size: 12px;
		font-weight: 700;
		color: #1a1a1a;
		text-decoration: none;
		background: #fff;
		border: 0;
		border-bottom: 1px solid #ececec;
		cursor: pointer;
	}

	.share-menu-item:last-child {
		border-bottom: 0;
	}

	.share-menu-item:hover {
		background: #f5f5f3;
	}

	.block.locked {
		outline: 1px solid color-mix(in srgb, var(--note-accent, #7c5cff) 28%, transparent);
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 8%, transparent);
	}

	.typing-lock {
		margin: 0 0 8px;
		display: inline-flex;
		align-items: center;
		padding: 4px 10px;
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		color: var(--note-text, #1f2328);
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 18%, var(--note-surface, #ffffff));
	}

	.lock-overlay {
		position: absolute;
		inset: 0;
		z-index: 3;
		border-radius: 6px;
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 6%, transparent);
		cursor: not-allowed;
	}

	/* Slash Menu */
	.slash-menu {
		position: fixed;
		z-index: 1000;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		box-shadow: 6px 6px 0 #1a1a1a;
		min-width: 252px;
		max-width: min(280px, calc(100vw - 24px));
		max-height: 300px;
		overflow-y: auto;
		padding: 4px;
		font-family: inherit;
	}

	.slash-menu.above {
		transform: translateY(calc(-100% - 2px));
	}

	.slash-menu-header {
		padding: 6px 8px;
		font-size: 10px;
		font-weight: 800;
		color: #666;
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}

	.slash-item {
		display: flex;
		align-items: center;
		gap: 9px;
		width: 100%;
		padding: 7px 8px;
		background: #fff;
		border: 1.5px solid transparent;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		transition: background 0.12s, border-color 0.12s, transform 0.12s;
	}

	.slash-item:hover,
	.slash-item.selected {
		background: #f5f5f3;
		border-color: #1a1a1a;
		transform: translateY(-1px);
	}

	.slash-icon {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: #fff;
		border: 1.5px solid #1a1a1a;
		border-radius: 6px;
		font-size: 13px;
		font-weight: 700;
		color: #1a1a1a;
		flex-shrink: 0;
	}

	.slash-info {
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.slash-label {
		display: inline-block;
		font-size: 9px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: #1a1a1a;
		border-bottom: 2px solid #1a1a1a;
		padding-bottom: 1px;
		line-height: 1.1;
	}

	.slash-desc {
		font-size: 10px;
		color: #666;
		line-height: 1.2;
		margin-top: 1px;
	}

	/* ---- Code block ---- */
	.code-block {
		border: 1px solid #e7e5e4;
		border-radius: 8px;
		overflow: hidden;
		background: #f7f7f5;
	}

	.code-toolbar {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 12px;
		background: #f1f1ef;
		border-bottom: 1px solid #e7e5e4;
	}

	.code-lang-select {
		background: #ffffff;
		color: #37352f;
		border: 1px solid #d6d3d1;
		border-radius: 6px;
		padding: 4px 8px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		outline: none;
	}

	.code-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #78716c;
	}

	.code-editor-shell {
		position: relative;
	}

	.code-highlight-layer {
		margin: 0;
		padding: 14px 16px;
		min-height: 120px;
		max-height: 600px;
		overflow: hidden;
		pointer-events: none;
		font-family: 'JetBrains Mono', 'Fira Code', 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
		font-size: 13px;
		line-height: 1.6;
		tab-size: 2;
		white-space: pre;
		box-sizing: border-box;
	}

	.code-highlight-layer :global(code.hljs) {
		display: block;
		background: transparent;
		padding: 0;
		margin: 0;
		font-family: inherit;
		font-size: inherit;
		line-height: inherit;
		white-space: pre;
	}

	.code-block .code-editor {
		width: 100%;
		min-height: 120px;
		max-height: 600px;
		resize: vertical;
		padding: 14px 16px;
		background: #f7f7f5;
		color: #2f3437;
		border: none;
		outline: none;
		font-family: 'JetBrains Mono', 'Fira Code', 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
		font-size: 13px;
		line-height: 1.6;
		tab-size: 2;
		white-space: pre;
		overflow: auto;
		box-sizing: border-box;
	}

	.code-editor-overlay {
		position: absolute;
		inset: 0;
		background: transparent !important;
		color: transparent !important;
		caret-color: #2f3437;
		z-index: 1;
	}

	.code-editor-overlay::selection {
		background: rgba(59, 130, 246, 0.28);
	}

	.code-block .code-editor::placeholder {
		color: #9b9a97;
	}

	.code-highlight-layer :global(.hljs-comment),
	.code-highlight-layer :global(.hljs-quote) {
		color: #9b9a97;
	}

	.code-highlight-layer :global(.hljs-keyword),
	.code-highlight-layer :global(.hljs-selector-tag),
	.code-highlight-layer :global(.hljs-literal),
	.code-highlight-layer :global(.hljs-title),
	.code-highlight-layer :global(.hljs-section),
	.code-highlight-layer :global(.hljs-doctag),
	.code-highlight-layer :global(.hljs-type) {
		color: #9a3412;
	}

	.code-highlight-layer :global(.hljs-string),
	.code-highlight-layer :global(.hljs-regexp),
	.code-highlight-layer :global(.hljs-meta .hljs-string) {
		color: #166534;
	}

	.code-highlight-layer :global(.hljs-number),
	.code-highlight-layer :global(.hljs-symbol),
	.code-highlight-layer :global(.hljs-bullet),
	.code-highlight-layer :global(.hljs-variable),
	.code-highlight-layer :global(.hljs-template-variable) {
		color: #1d4ed8;
	}

	.code-highlight-layer :global(.hljs-function .hljs-title),
	.code-highlight-layer :global(.hljs-title.function_) {
		color: #7c2d12;
	}

	:global(.editor-main.dark) .code-block {
		background: #191919;
		border-color: #2f2f2f;
	}

	:global(.editor-main.dark) .code-toolbar {
		background: #222222;
		border-bottom-color: #2f2f2f;
	}

	:global(.editor-main.dark) .code-lang-select {
		background: #2b2b2b;
		color: #e8e8e8;
		border-color: #3a3a3a;
	}

	:global(.editor-main.dark) .code-label {
		color: #a3a3a3;
	}

	:global(.editor-main.dark) .code-block .code-editor {
		background: #191919;
		color: #e8e8e8;
	}

	:global(.editor-main.dark) .code-editor-overlay {
		caret-color: #e8e8e8;
	}

	:global(.editor-main.dark) .code-block .code-editor::placeholder {
		color: #8a8a8a;
	}

	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-comment),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-quote) {
		color: #7f7f7f;
	}

	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-keyword),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-selector-tag),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-literal),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-title),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-section),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-doctag),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-type) {
		color: #f59e0b;
	}

	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-string),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-regexp),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-meta .hljs-string) {
		color: #86efac;
	}

	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-number),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-symbol),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-bullet),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-variable),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-template-variable) {
		color: #93c5fd;
	}

	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-function .hljs-title),
	:global(.editor-main.dark) .code-highlight-layer :global(.hljs-title.function_) {
		color: #fda4af;
	}

	/* ---- Canvas block ---- */
	.music-block-wrap {
		width: 100%;
	}

	.canvas-block {
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 10px;
		overflow: hidden;
		background: #1e1e2e;
	}

	.canvas-toolbar {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 12px;
		background: #181825;
		border-bottom: 1px solid #313244;
		flex-wrap: wrap;
	}

	.canvas-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6c7086;
		margin-right: auto;
	}

	.canvas-dims {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.canvas-dim-input {
		width: 60px;
		background: #313244;
		color: #cdd6f4;
		border: 1px solid #45475a;
		border-radius: 6px;
		padding: 3px 6px;
		font-size: 12px;
		text-align: center;
		outline: none;
	}

	.canvas-dim-x {
		color: #6c7086;
		font-size: 12px;
	}

	.canvas-run-btn {
		background: #a6e3a1;
		color: #1e1e2e;
		border: none;
		border-radius: 6px;
		padding: 4px 12px;
		font-size: 12px;
		font-weight: 700;
		cursor: pointer;
		transition: background 0.12s;
	}

	.canvas-run-btn:hover {
		background: #94e2d5;
	}

	.canvas-stop-btn {
		background: #f38ba8;
		color: #1e1e2e;
		border: none;
		border-radius: 6px;
		padding: 4px 12px;
		font-size: 12px;
		font-weight: 700;
		cursor: pointer;
		transition: background 0.12s;
	}

	.canvas-stop-btn:hover {
		background: #eba0ac;
	}

	.canvas-code {
		border-bottom: 1px solid #313244;
	}

	.canvas-error {
		padding: 8px 14px;
		background: #45273a;
		color: #f38ba8;
		font-size: 12px;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		border-bottom: 1px solid #313244;
	}

	.canvas-preview {
		background: #ffffff;
		padding: 8px;
		overflow: auto;
		display: flex;
		justify-content: center;
	}

	.canvas-el {
		max-width: 100%;
		height: auto;
		border-radius: 4px;
		box-shadow: 0 0 0 1px rgba(0,0,0,0.06);
	}

	/* ---- Mobile / responsive ---- */
	@media (max-width: 680px) {
		.block {
			padding: 10px 8px;
			border-radius: 8px;
		}

		/* Move handle & delete inside the block on mobile */
		.block-handle {
			position: relative;
			left: 0;
			top: 0;
			opacity: 0.5;
			padding: 6px;
			margin-right: 4px;
			flex-shrink: 0;
		}

		.delete-btn {
			opacity: 0.5;
			padding: 6px 8px;
			flex-shrink: 0;
		}

		.share-btn {
			position: relative;
			right: 0;
			top: 0;
			opacity: 0.5;
			margin-left: 4px;
		}

		.share-menu {
			right: 0;
			top: 34px;
			box-shadow: 4px 4px 0 #1a1a1a;
		}

		/* Floating toolbar becomes full-width anchored */
		.rich-toolbar {
			position: relative;
			top: auto !important;
			left: auto !important;
			transform: none;
			width: 100%;
			justify-content: center;
			margin-bottom: 8px;
			box-shadow: 0 4px 16px rgba(15, 23, 42, 0.12);
			overflow-x: auto;
		}

		@keyframes toolbar-in {
			from { opacity: 0; transform: translateY(4px); }
			to   { opacity: 1; transform: translateY(0); }
		}

		/* Headings scale down */
		.heading-1 { font-size: 24px; margin: 8px 0 2px; }
		.heading-2 { font-size: 20px; margin: 6px 0 2px; }
		.heading-3 { font-size: 17px; margin: 4px 0 2px; }

		/* Gallery collapses to fewer columns */
		.gallery-grid {
			grid-template-columns: repeat(min(var(--gallery-cols, 2), 2), minmax(0, 1fr)) !important;
		}

		.gallery-image {
			height: 140px;
		}

		.gallery-toolbar {
			flex-wrap: wrap;
			gap: 6px;
		}

		/* Code/canvas responsive */
		.code-editor {
			min-height: 80px;
			font-size: 12px;
			padding: 10px 12px;
		}

		.canvas-toolbar {
			gap: 6px;
			padding: 6px 8px;
		}

		.canvas-dims {
			order: 10;
			width: 100%;
			justify-content: center;
		}

		.canvas-dim-input {
			width: 50px;
		}

		/* Embed responsive */
		.embed-frame {
			height: 260px;
			border-radius: 8px;
		}

		/* Image placeholder compact */
		.image-placeholder,
		.embed-placeholder {
			padding: 12px;
			gap: 8px;
			border-radius: 8px;
			font-size: 13px;
		}

		/* Caption responsive */
		.caption-input {
			font-size: 12px;
			max-width: 100%;
		}

		/* Slash menu responsive */
		.slash-menu {
			min-width: 240px;
			max-width: calc(100vw - 32px);
			border-radius: 10px;
		}

		.slash-icon {
			width: 32px;
			height: 32px;
			font-size: 15px;
		}

		.slash-item {
			padding: 6px 12px;
			gap: 10px;
		}

		.slash-label { font-size: 13px; }
		.slash-desc { font-size: 11px; }
	}

	@media (max-width: 400px) {
		.gallery-grid {
			grid-template-columns: 1fr !important;
		}

		.heading-1 { font-size: 22px; }
		.heading-2 { font-size: 18px; }
		.heading-3 { font-size: 16px; }

		.rich-toolbar {
			gap: 1px;
			padding: 3px 4px;
		}

		.rich-btn {
			min-width: 24px;
			height: 26px;
			padding: 3px 5px;
			font-size: 12px;
		}
	}

	/* ---- Inline rich-text element styles ---- */
	.editable :global(code) {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 10%, var(--note-surface, #f6f6f7));
		color: var(--note-accent, #7c5cff);
		padding: 1px 5px;
		border-radius: 4px;
		font-family: 'JetBrains Mono', 'Fira Code', 'SF Mono', monospace;
		font-size: 0.88em;
		font-weight: 500;
		border: 1px solid color-mix(in srgb, var(--note-accent, #7c5cff) 14%, transparent);
	}

	.editable :global(mark) {
		background: color-mix(in srgb, #facc15 38%, transparent);
		color: inherit;
		padding: 1px 2px;
		border-radius: 3px;
	}

	.editable :global(a) {
		color: var(--note-accent, #7c5cff);
		text-decoration: underline;
		text-decoration-color: color-mix(in srgb, var(--note-accent, #7c5cff) 40%, transparent);
		text-underline-offset: 2px;
		transition: text-decoration-color 0.15s;
	}

	.editable :global(a:hover) {
		text-decoration-color: var(--note-accent, #7c5cff);
	}
</style>
