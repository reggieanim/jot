<script lang="ts">
	import { createEventDispatcher, onMount, onDestroy, tick } from 'svelte';
	import { htmlFromBlockData, plainTextFromBlockData, sanitizeRichText } from '$lib/editor/richtext';
	import { copyTextToClipboard } from '$lib/utils/clipboard';

	export let id: string;
	export let type: string;
	export let data: any;
	export let apiUrl = 'http://localhost:8080';
	export let pageId = '';
	export let published = false;
	export let isDragging = false;
	export let viewerSessionId = '';
	export let lockOwner: { sessionId: string; userName: string } | null = null;

	let showShareToast = false;

	const dispatch = createEventDispatcher();

	let contentEl: HTMLElement;
	let imageInputEl: HTMLInputElement;
	let galleryInputEl: HTMLInputElement;
	let showSlashMenu = false;
	let slashMenuPosition = { x: 0, y: 0 };
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
		{ id: 'divider', label: 'Divider', icon: '—', description: 'Visual divider' },
		{ id: 'image', label: 'Image', icon: '🖼', description: 'Upload or embed image' },
		{ id: 'gallery', label: 'Gallery', icon: '▦', description: '2-4 image columns' },
		{ id: 'embed', label: 'Embed', icon: '◆', description: 'Embed external content' }
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
				slashMenuPosition = { x: rect.left, y: rect.bottom + 8 };
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

		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			e.stopPropagation();
			saveText();
			dispatch('addAfter', { id });
		}

		if (e.key === 'Backspace' && contentEl?.innerText === '') {
			e.preventDefault();
			e.stopPropagation();
			dispatch('delete', { id });
		}
	}

	function applyFormat(command: string) {
		if (isLocked || !contentEl) return;
		contentEl.focus();
		restoreSelection();
		document.execCommand(command, false);
		handleInput({ target: contentEl } as unknown as Event);
		refreshSelectionState();
		debouncedSave();
	}

	function applyLink() {
		if (isLocked || !contentEl) return;
		restoreSelection();
		const href = window.prompt('Paste link URL');
		if (!href) return;
		contentEl.focus();
		restoreSelection();
		document.execCommand('createLink', false, href.trim());
		handleInput({ target: contentEl } as unknown as Event);
		refreshSelectionState();
		debouncedSave();
	}

	function handleMouseUp() {
		refreshSelectionState();
	}

	function handleKeyUp() {
		refreshSelectionState();
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

	async function handleShare() {
		if (!pageId || !id) return;
		const origin = typeof window !== 'undefined' ? window.location.origin : '';
		const embedUrl = `${origin}/embed/${encodeURIComponent(pageId)}/${encodeURIComponent(id)}`;
		const copied = await copyTextToClipboard(embedUrl);
		if (copied) {
			showShareToast = true;
			setTimeout(() => (showShareToast = false), 2000);
		}
	}

	function triggerImageUpload() {
		if (isLocked) return;
		imageInputEl?.click();
	}

	async function uploadImageFile(file: File): Promise<string> {
		const formData = new FormData();
		formData.append('file', file);

		const response = await fetch(`${apiUrl}/v1/media/images`, {
			method: 'POST',
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
		if (localText === '') {
			focus();
		}
		const listener = () => refreshSelectionState();
		document.addEventListener('selectionchange', listener);
		return () => {
			document.removeEventListener('selectionchange', listener);
		};
	});

	onDestroy(() => {
		clearTimeout(saveTimeout);
	});
</script>

<div class="block" class:dragging={isDragging} class:locked={isLocked} data-block-id={id}>
	<button type="button" class="block-handle" draggable="true" aria-label="Drag block" on:dragstart={() => dispatch('dragstart', { id })}>
		<span class="handle-icon">⋮⋮</span>
	</button>

	<div class="block-content">
		{#if isRichTextType && isEditingText && hasRichSelection && !isLocked}
			<div class="rich-toolbar" role="toolbar" aria-label="Rich text toolbar">
				<button type="button" class="rich-btn" on:mousedown|preventDefault on:click={() => applyFormat('bold')}><strong>B</strong></button>
				<button type="button" class="rich-btn" on:mousedown|preventDefault on:click={() => applyFormat('italic')}><em>I</em></button>
				<button type="button" class="rich-btn" on:mousedown|preventDefault on:click={() => applyFormat('underline')}><u>U</u></button>
				<button type="button" class="rich-btn" on:mousedown|preventDefault on:click={() => applyFormat('strikeThrough')}><s>S</s></button>
				<button type="button" class="rich-btn" on:mousedown|preventDefault on:click={applyLink}>Link</button>
				<button type="button" class="rich-btn" on:mousedown|preventDefault on:click={() => applyFormat('removeFormat')}>Clear</button>
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
				<span class="number">{data?.number || '1'}.</span>
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
		{:else if type === 'divider'}
			<div class="divider-wrap">
				<hr class="divider" />
			</div>
		{:else if type === 'image'}
			{#if data?.url}
				<button
					type="button"
					class="image-hit"
					on:click={triggerImageUpload}
					on:dragover|preventDefault|stopPropagation={() => {}}
					on:drop={handleMediaDrop}
				>
					<img
						src={data.url}
						alt="block"
						class="block-image"
					/>
				</button>
			{:else}
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
					<button type="button" class="image-placeholder" on:click={triggerGalleryUpload}>
						<span>🖼</span>
						<span>Add images or drag text/image blocks here</span>
					</button>
				{:else}
					<div class="gallery-grid" style="--gallery-cols: {columns};">
						{#each items as item, i (item.id)}
							<div class="gallery-item" role="group" aria-label={`Gallery card ${i + 1}`} class:text-card={item.kind === 'text'} draggable="true" on:dragstart={(e) => handleGalleryItemDragStart(e, item)}>
								{#if item.kind === 'image'}
									<img src={item.value} alt={`gallery-${i}`} class="gallery-image" />
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
				<iframe src={data.url} class="embed-frame" title="Embedded content"></iframe>
			{:else}
				<div class="embed-placeholder">
					<input
						type="text"
						placeholder="Paste embed URL..."
						on:change={handleEmbedInputChange}
					/>
				</div>
			{/if}
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
		<button class="share-btn" title={showShareToast ? 'Copied!' : 'Copy embed link'} on:click={handleShare}>
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
	{/if}
	<button class="delete-btn" title="Delete" on:click={handleDelete}>✕</button>
</div>

{#if showSlashMenu}
	<div class="slash-menu" style="left: {slashMenuPosition.x}px; top: {slashMenuPosition.y}px;">
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
		display: inline-flex;
		gap: 6px;
		margin-bottom: 8px;
		padding: 4px;
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 8px;
		background: color-mix(in srgb, var(--note-surface, #ffffff) 92%, var(--note-accent, #7c5cff) 8%);
	}

	.rich-btn {
		border: 1px solid var(--note-border, #d1d5db);
		background: var(--note-surface, #ffffff);
		color: var(--note-text, #1f2328);
		border-radius: 6px;
		padding: 4px 8px;
		font-size: 12px;
		cursor: pointer;
	}

	.rich-btn:hover {
		background: color-mix(in srgb, var(--note-surface, #ffffff) 86%, var(--note-accent, #7c5cff) 14%);
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

	.divider {
		border: none;
		border-top: 1px solid var(--note-border, #e5e7eb);
		margin: 0;
	}

	.divider-wrap {
		display: flex;
		align-items: center;
		min-height: 26px;
		padding: 6px 0;
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
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 14px;
		box-shadow: 0 18px 46px rgba(15, 23, 42, 0.16);
		min-width: 280px;
		max-height: 360px;
		overflow-y: auto;
		padding: 8px 0;
	}

	.slash-menu-header {
		padding: 8px 14px;
		font-size: 11px;
		font-weight: 600;
		color: var(--note-muted, #9ca3af);
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.slash-item {
		display: flex;
		align-items: center;
		gap: 12px;
		width: 100%;
		padding: 8px 14px;
		background: transparent;
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background 0.1s;
	}

	.slash-item:hover,
	.slash-item.selected {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 10%, transparent);
	}

	.slash-icon {
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #e5e7eb);
		border-radius: 4px;
		font-size: 18px;
		flex-shrink: 0;
	}

	.slash-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.slash-label {
		font-size: 14px;
		font-weight: 500;
		color: var(--note-title, #1f2328);
	}

	.slash-desc {
		font-size: 12px;
		color: var(--note-muted, #6b7280);
	}
</style>
