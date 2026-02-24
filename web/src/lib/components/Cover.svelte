<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { env } from '$env/dynamic/public';
	import type { ApiBlock } from '$lib/editor/types';

	export let cover: string | null;
	export let apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';
	export let pageId = '';
	export let shareToken = '';
	export let readonly = false;
	export let title = 'Untitled';
	export let blocks: ApiBlock[] = [];

	const dispatch = createEventDispatcher();

	let fileInput: HTMLInputElement;

	/* ── TOC: derive entries from heading blocks ── */
	type TocEntry = { id: string; label: string; type: string; icon: string; index: number; depth: number; isLast: boolean };

	$: tocEntries = buildToc(blocks);

	function buildToc(b: ApiBlock[]): TocEntry[] {
		const entries: TocEntry[] = [];
		let idx = 0;
		for (const block of b) {
			idx++;
			switch (block.type) {
				case 'heading':
				case 'heading2':
				case 'heading3': {
					const text = stripHtml(block.data?.text || '');
					const level = block.type === 'heading' ? 1 : block.type === 'heading2' ? 2 : 3;
					if (text) entries.push({ id: block.id || `toc-${idx}`, label: text, type: block.type, icon: `H${level}`, index: idx, depth: level - 1, isLast: false });
					break;
				}
				case 'code':
					entries.push({ id: block.id || `toc-${idx}`, label: block.data?.language || 'Code', type: 'code', icon: '⌘', index: idx, depth: 0, isLast: false });
					break;
				case 'canvas':
					entries.push({ id: block.id || `toc-${idx}`, label: block.data?.caption || 'Canvas', type: 'canvas', icon: '◆', index: idx, depth: 0, isLast: false });
					break;
				case 'image':
					if (block.data?.url) entries.push({ id: block.id || `toc-${idx}`, label: block.data?.caption || 'Image', type: 'image', icon: '▣', index: idx, depth: 0, isLast: false });
					break;
				case 'gallery':
					entries.push({ id: block.id || `toc-${idx}`, label: `Gallery · ${block.data?.items?.length || 0}`, type: 'gallery', icon: '⊞', index: idx, depth: 0, isLast: false });
					break;
				case 'embed':
					if (block.data?.url) entries.push({ id: block.id || `toc-${idx}`, label: block.data?.caption || 'Embed', type: 'embed', icon: '◈', index: idx, depth: 0, isLast: false });
					break;
				case 'quote':
				case 'callout': {
					const text = stripHtml(block.data?.text || '');
					if (text) entries.push({ id: block.id || `toc-${idx}`, label: text.slice(0, 40) + (text.length > 40 ? '…' : ''), type: block.type, icon: block.type === 'quote' ? '❝' : '!', index: idx, depth: 0, isLast: false });
					break;
				}
				default:
					break;
			}
		}
		// Mark last sibling at each depth for the tree connector
		for (let i = 0; i < entries.length; i++) {
			const depth = entries[i].depth;
			let isLast = true;
			for (let j = i + 1; j < entries.length; j++) {
				if (entries[j].depth <= depth) { isLast = entries[j].depth < depth; break; }
			}
			entries[i].isLast = isLast;
		}
		return entries;
	}

	function stripHtml(html: string): string {
		if (typeof document !== 'undefined') {
			const tmp = document.createElement('span');
			tmp.innerHTML = html;
			return tmp.textContent?.trim() || '';
		}
		return html.replace(/<[^>]*>/g, '').trim();
	}

	function scrollToBlock(blockId: string) {
		const el = document.querySelector(`[data-block-id="${blockId}"]`);
		if (!el) {
			/* fallback: find the block wrapper that contains a Block with this id */
			const all = document.querySelectorAll('.block-wrapper');
			const idx = blocks.findIndex((b) => b.id === blockId);
			if (idx >= 0 && all[idx]) {
				all[idx].scrollIntoView({ behavior: 'smooth', block: 'center' });
			}
			return;
		}
		el.scrollIntoView({ behavior: 'smooth', block: 'center' });
	}

	async function uploadImage(file: File): Promise<string> {
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
			throw new Error('cover upload failed');
		}

		const payload = await response.json();
		const url = payload?.url;
		if (typeof url !== 'string' || !url) {
			throw new Error('invalid upload response');
		}

		return url;
	}

	async function handleImageChange(e: Event) {
		const input = e.target as HTMLInputElement;
		if (input.files?.[0]) {
			try {
				const uploadedCover = await uploadImage(input.files[0]);
				dispatch('change', { cover: uploadedCover });
			} catch {
				// ignore upload failure here
			}
			input.value = '';
		}
	}

	function handleRemove() {
		if (readonly) return;
		dispatch('change', { cover: null });
	}

	function triggerUpload() {
		if (readonly) return;
		fileInput?.click();
	}
</script>

<div class="cover-area" class:has-cover={!!cover}>
	<!-- Expand hint arrow (visible when collapsed) -->
	<div class="expand-hint" aria-hidden="true">
		<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
			<polyline points="9 6 15 12 9 18"></polyline>
		</svg>
	</div>

	<!-- Background cover image (always behind TOC) -->
	{#if cover}
		<img src={cover} alt="page cover" class="cover-bg-image" />
		<div class="cover-overlay"></div>
	{/if}

	<!-- TOC content layer -->
	<div class="toc-layer">
		<div class="toc-header">
			<span class="toc-title-label">Contents</span>
			<div class="toc-page-title" title={title}>{title || 'Untitled'}</div>
		</div>

		{#if tocEntries.length > 0}
			<nav class="toc-nav" aria-label="Table of contents">
				{#each tocEntries as entry, i (entry.id)}
					<button
						class="toc-entry toc-{entry.type}"
						class:toc-last={entry.isLast}
						style="--depth:{entry.depth}"
						on:click={() => scrollToBlock(entry.id)}
						title={entry.label}
					>
						<!-- Tree connector lines -->
						{#if entry.depth > 0}
							<span class="toc-connector" aria-hidden="true">
								{#each Array(entry.depth) as _, d}
									<span class="toc-thread" class:toc-thread-last={d === entry.depth - 1 && entry.isLast}></span>
								{/each}
							</span>
						{:else}
							<span class="toc-root-spacer" aria-hidden="true"></span>
						{/if}
						<span class="toc-dot"></span>
						<span class="toc-label">{entry.label}</span>
						<span class="toc-type-badge">{entry.icon}</span>
					</button>
				{/each}
			</nav>
		{:else}
			{#if !readonly}
				<div class="toc-empty">
					<span class="toc-empty-icon">✎</span>
					<span class="toc-empty-text">Add headings, code, or media blocks to see your outline here</span>
				</div>
			{/if}
		{/if}

		<!-- Cover actions at the bottom -->
		<div class="toc-footer">
			{#if !readonly}
				<div class="cover-btns">
					<button class="cover-action-btn" on:click={triggerUpload}>
						<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
							<circle cx="8.5" cy="8.5" r="1.5"></circle>
							<polyline points="21 15 16 10 5 21"></polyline>
						</svg>
						{cover ? 'Change' : 'Cover'}
					</button>
					{#if cover}
						<button class="cover-action-btn" on:click={handleRemove}>
							<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<line x1="18" y1="6" x2="6" y2="18"></line>
								<line x1="6" y1="6" x2="18" y2="18"></line>
							</svg>
							Remove
						</button>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	{#if !readonly}
		<input
			bind:this={fileInput}
			type="file"
			accept="image/*"
			on:change={handleImageChange}
			class="hidden-input"
		/>
	{/if}
</div>

<style>
	.cover-area {
		position: relative;
		width: 100%;
		height: 100%;
		min-height: 100vh;
		background: var(--note-surface, #fafafa);
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	/* ── Expand hint arrow ── */

	.expand-hint {
		position: absolute;
		top: 50%;
		right: 2px;
		transform: translateY(-50%);
		z-index: 10;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 40px;
		border-radius: 4px;
		color: var(--note-muted, #9ca3af);
		opacity: 0.45;
		transition: opacity 0.3s ease, transform 0.3s ease, color 0.3s ease;
		pointer-events: none;
		animation: hint-breathe 2.4s ease-in-out infinite;
	}

	.has-cover .expand-hint {
		color: rgba(255, 255, 255, 0.5);
	}

	/* Hide when the rail is expanded (parent .cover-rail gets hovered or has-cover) */
	:global(.cover-rail:hover) .expand-hint,
	:global(.cover-rail.has-cover) .expand-hint {
		opacity: 0;
		transform: translateY(-50%) translateX(6px);
	}

	@keyframes hint-breathe {
		0%, 100% {
			transform: translateY(-50%) translateX(0px);
			opacity: 0.35;
		}
		50% {
			transform: translateY(-50%) translateX(3px);
			opacity: 0.65;
		}
	}

	/* ── Background image layer ── */

	.cover-bg-image {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
		z-index: 0;
	}

	.cover-overlay {
		position: absolute;
		inset: 0;
		z-index: 1;
		background: linear-gradient(
			180deg,
			rgba(0, 0, 0, 0.55) 0%,
			rgba(0, 0, 0, 0.35) 40%,
			rgba(0, 0, 0, 0.50) 100%
		);
		backdrop-filter: blur(1px);
	}

	/* ── TOC content layer ── */

	.toc-layer {
		position: relative;
		z-index: 2;
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 100vh;
		padding: 24px 16px 16px;
		box-sizing: border-box;
		gap: 12px;
	}

	/* ── Header ── */

	.toc-header {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding-bottom: 12px;
		border-bottom: 1px solid var(--note-border, rgba(255, 255, 255, 0.15));
	}

	.has-cover .toc-header {
		border-bottom-color: rgba(255, 255, 255, 0.18);
	}

	.toc-title-label {
		font-size: 10px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--note-muted, #6b7280);
	}

	.has-cover .toc-title-label {
		color: rgba(255, 255, 255, 0.55);
	}

	.toc-page-title {
		font-size: 15px;
		font-weight: 700;
		line-height: 1.3;
		color: var(--note-title, #111827);
		overflow: hidden;
		text-overflow: ellipsis;
		display: -webkit-box;
		-webkit-line-clamp: 3;
		-webkit-box-orient: vertical;
		word-break: break-word;
	}

	.has-cover .toc-page-title {
		color: #ffffff;
		text-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
	}

	/* ── TOC Navigation tree ── */

	.toc-nav {
		display: flex;
		flex-direction: column;
		gap: 0;
		flex: 1;
		overflow-y: auto;
		scrollbar-width: thin;
		scrollbar-color: rgba(128, 128, 128, 0.2) transparent;
		padding-left: 0;
		padding-top: 6px;
	}

	.toc-entry {
		display: flex;
		align-items: center;
		gap: 0;
		padding: 0 4px 0 0;
		border: none;
		background: transparent;
		color: var(--note-text, #374151);
		cursor: pointer;
		text-align: left;
		width: 100%;
		position: relative;
		min-height: 28px;
		border-radius: 5px;
		transition: background 0.12s;
	}

	.toc-entry:hover {
		background: rgba(0, 0, 0, 0.05);
	}

	.has-cover .toc-entry:hover {
		background: rgba(255, 255, 255, 0.1);
	}

	.has-cover .toc-entry {
		color: rgba(255, 255, 255, 0.8);
	}

	/* Root-level entry spacer (no connector, but aligns dot with children) */
	.toc-root-spacer {
		display: flex;
		width: 8px;
		flex-shrink: 0;
	}

	/* ── Tree connector wires ── */

	.toc-connector {
		display: flex;
		align-self: stretch;
		flex-shrink: 0;
		align-items: stretch;
	}

	.toc-thread {
		display: block;
		width: 16px;
		flex-shrink: 0;
		position: relative;
		align-self: stretch;
	}

	/* Vertical line running through the full thread cell */
	.toc-thread::before {
		content: '';
		position: absolute;
		left: 7px;
		top: 0;
		bottom: 0;
		width: 1.5px;
		background: var(--note-border, rgba(0,0,0,0.12));
		border-radius: 2px;
	}

	.has-cover .toc-thread::before {
		background: rgba(255,255,255,0.18);
	}

	/* The L-turn elbow on the last thread segment */
	.toc-thread.toc-thread-last::before {
		bottom: 50%;
	}

	.toc-thread.toc-thread-last::after {
		content: '';
		position: absolute;
		left: 7px;
		top: 50%;
		width: 8px;
		height: 1.5px;
		background: var(--note-border, rgba(0,0,0,0.12));
		border-radius: 2px;
	}

	.has-cover .toc-thread.toc-thread-last::after {
		background: rgba(255,255,255,0.18);
	}

	/* Continuing vertical + horizontal T connector (not last) */
	.toc-thread:not(.toc-thread-last)::after {
		content: '';
		position: absolute;
		left: 7px;
		top: 50%;
		width: 8px;
		height: 1.5px;
		background: var(--note-border, rgba(0,0,0,0.12));
		border-radius: 2px;
	}

	.has-cover .toc-thread:not(.toc-thread-last)::after {
		background: rgba(255,255,255,0.18);
	}

	/* ── Dot node ── */

	.toc-dot {
		width: 6px;
		height: 6px;
		flex-shrink: 0;
		border-radius: 50%;
		background: var(--note-border, rgba(0,0,0,0.2));
		margin-right: 8px;
		transition: background 0.15s, transform 0.15s;
	}

	.has-cover .toc-dot {
		background: rgba(255,255,255,0.3);
	}

	.toc-heading > .toc-dot {
		width: 8px;
		height: 8px;
		background: var(--note-title, #111827);
		box-shadow: 0 0 0 2px rgba(0,0,0,0.1);
	}

	.has-cover .toc-heading > .toc-dot {
		background: rgba(255,255,255,0.7);
		box-shadow: 0 0 0 2px rgba(255,255,255,0.15);
	}

	.toc-entry:hover .toc-dot {
		background: var(--note-title, #111827);
		transform: scale(1.3);
	}

	.has-cover .toc-entry:hover .toc-dot {
		background: #fff;
	}

	/* ── Label ── */

	.toc-label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
		font-weight: 500;
		line-height: 28px;
		transition: color 0.12s;
		padding: 0 4px 0 0;
	}

	.toc-heading > .toc-label {
		font-weight: 700;
		font-size: 12.5px;
		color: var(--note-title, #111827);
	}

	.has-cover .toc-heading > .toc-label {
		color: #fff;
	}

	.toc-entry:hover .toc-label {
		color: var(--note-title, #111827);
	}

	.has-cover .toc-entry:hover .toc-label {
		color: #fff;
	}

	/* ── Type badge (icon) ── */

	.toc-type-badge {
		flex-shrink: 0;
		font-size: 9px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--note-muted, #9ca3af);
		opacity: 0.6;
		padding-right: 4px;
		line-height: 1;
		transition: opacity 0.12s, color 0.12s;
	}

	.has-cover .toc-type-badge {
		color: rgba(255,255,255,0.4);
	}

	.toc-entry:hover .toc-type-badge {
		opacity: 1;
		color: var(--note-text, #374151);
	}

	.has-cover .toc-entry:hover .toc-type-badge {
		color: rgba(255,255,255,0.8);
	}

	/* ── Empty state ── */

	.toc-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding: 24px 8px;
		text-align: center;
	}

	.toc-empty-icon {
		font-size: 24px;
		opacity: 0.3;
		color: var(--note-muted, #6b7280);
	}

	.has-cover .toc-empty-icon {
		color: rgba(255, 255, 255, 0.4);
	}

	.toc-empty-text {
		font-size: 11px;
		line-height: 1.5;
		color: var(--note-muted, #6b7280);
		opacity: 0.7;
	}

	.has-cover .toc-empty-text {
		color: rgba(255, 255, 255, 0.45);
	}

	/* ── Footer (cover actions) ── */

	.toc-footer {
		margin-top: auto;
		padding-top: 12px;
		border-top: 1px solid var(--note-border, rgba(0, 0, 0, 0.06));
	}

	.has-cover .toc-footer {
		border-top-color: rgba(255, 255, 255, 0.15);
	}

	.cover-btns {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.cover-action-btn {
		display: flex;
		align-items: center;
		gap: 5px;
		padding: 5px 10px;
		border-radius: 5px;
		border: 1px solid var(--note-border, rgba(0, 0, 0, 0.1));
		background: var(--note-surface, rgba(255, 255, 255, 0.08));
		color: var(--note-muted, #6b7280);
		font-size: 11px;
		font-weight: 600;
		cursor: pointer;
		transition: background 0.12s, color 0.12s;
	}

	.has-cover .cover-action-btn {
		border-color: rgba(255, 255, 255, 0.2);
		background: rgba(0, 0, 0, 0.35);
		color: rgba(255, 255, 255, 0.7);
		backdrop-filter: blur(4px);
	}

	.cover-action-btn:hover {
		background: var(--note-accent, #7c5cff);
		color: #ffffff;
		border-color: var(--note-accent, #7c5cff);
	}

	.has-cover .cover-action-btn:hover {
		background: rgba(255, 255, 255, 0.22);
		color: #ffffff;
		border-color: rgba(255, 255, 255, 0.35);
	}

	.cover-action-btn svg {
		stroke: currentColor;
	}

	.hidden-input {
		display: none;
	}

	/* ── Mobile: horizontal compact mode ── */
	@media (max-width: 980px) {
		.cover-area {
			min-height: auto;
			height: auto;
		}

		.toc-layer {
			min-height: auto;
			padding: 14px 16px;
		}

		.toc-nav {
			max-height: 140px;
		}

		.toc-empty {
			padding: 12px 8px;
		}

		.expand-hint {
			display: none;
		}
	}
</style>
