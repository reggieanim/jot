<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { ApiBlock, ApiProofread, ApiProofreadAnnotation } from '$lib/editor/types';
	import { normalizeGalleryItems } from '$lib/editor/blocks';
	import { htmlFromBlockData } from '$lib/editor/richtext';
	import { copyTextToClipboard } from '$lib/utils/clipboard';

	export let blocks: ApiBlock[] = [];
	export let interactive = false;
	export let selectedBlockId = '';
	export let draftStates: Record<string, { kind: string; text: string }> = {};
	export let anchorPrefix = 'block-';
	export let pageId = '';
	export let proofreads: ApiProofread[] = [];

	const dispatch = createEventDispatcher<{ select: { blockId: string } }>();

	let shareToastBlockId = '';

	/* ── Proofread annotation map: blockId → enriched annotations ── */
	type EnrichedAnnotation = ApiProofreadAnnotation & { authorName: string; proofreadTitle: string; stance: string; proofreadId: string };

	$: annotationMap = buildAnnotationMap(proofreads);

	function buildAnnotationMap(prs: ApiProofread[]): Record<string, EnrichedAnnotation[]> {
		const map: Record<string, EnrichedAnnotation[]> = {};
		for (const pr of prs) {
			if (!pr.annotations) continue;
			for (const ann of pr.annotations) {
				if (!ann.block_id) continue;
				if (!map[ann.block_id]) map[ann.block_id] = [];
				map[ann.block_id].push({
					...ann,
					authorName: pr.author_name,
					proofreadTitle: pr.title,
					stance: pr.stance,
					proofreadId: pr.id
				});
			}
		}
		return map;
	}

	/* ── Popup slideshow state ── */
	let popupBlockId = '';
	let popupIndex = 0;
	$: popupAnnotations = popupBlockId ? (annotationMap[popupBlockId] || []) : [];
	$: popupCurrent = popupAnnotations[popupIndex] || null;

	function openPopup(blockId: string) {
		if (popupBlockId === blockId) {
			popupBlockId = '';
			return;
		}
		popupBlockId = blockId;
		popupIndex = 0;
	}

	function closePopup() {
		popupBlockId = '';
		popupIndex = 0;
	}

	function popupPrev() {
		if (popupIndex > 0) popupIndex--;
	}

	function popupNext() {
		if (popupIndex < popupAnnotations.length - 1) popupIndex++;
	}

	function handlePopupKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowLeft') popupPrev();
		else if (e.key === 'ArrowRight') popupNext();
		else if (e.key === 'Escape') closePopup();
	}

	function htmlOf(block: ApiBlock) {
		return htmlFromBlockData(block.data);
	}

	function blockIdOf(block: ApiBlock, index: number) {
		return block.id || `${block.type}-${index}`;
	}

	function isTextual(block: ApiBlock) {
		return ['paragraph', 'heading', 'heading2', 'heading3', 'bullet', 'numbered', 'quote'].includes(block.type);
	}

	function handleSelect(blockId: string) {
		if (!interactive) return;
		dispatch('select', { blockId });
	}

	let canvasRefs: Record<string, HTMLCanvasElement> = {};

	function bindCanvas(el: HTMLCanvasElement, blockId: string) {
		canvasRefs[blockId] = el;
		// Run the canvas code as soon as the element is bound
		const block = blocks.find((b, i) => blockIdOf(b, i) === blockId);
		if (block?.data?.code) {
			// Small delay to ensure the canvas is fully laid out
			requestAnimationFrame(() => runCanvasBlock(blockId, block.data.code));
		}
		return {
			destroy() {
				stopCanvasBlock(blockId);
				delete canvasRefs[blockId];
			}
		};
	}

	let canvasRafIds: Record<string, number> = {};

	function stopCanvasBlock(blockId: string) {
		if (canvasRafIds[blockId]) {
			cancelAnimationFrame(canvasRafIds[blockId]);
			delete canvasRafIds[blockId];
		}
	}

	function runCanvasBlock(blockId: string, code: string) {
		stopCanvasBlock(blockId);
		const el = canvasRefs[blockId];
		if (!el) return;
		const ctx = el.getContext('2d');
		if (!ctx) return;
		ctx.clearRect(0, 0, el.width, el.height);

		let loopFn: ((t: number) => void) | null = null;
		const loop = (fn: (t: number) => void) => { loopFn = fn; };

		try {
			const fn = new Function('canvas', 'ctx', 'loop', code);
			fn(el, ctx, loop);
		} catch { /* swallow in readonly */ }

		if (loopFn) {
			const userLoop = loopFn;
			let running = true;
			const tick = (t: number) => {
				if (!running) return;
				try { userLoop(t); } catch { running = false; return; }
				canvasRafIds[blockId] = requestAnimationFrame(tick);
			};
			canvasRafIds[blockId] = requestAnimationFrame(tick);
		}
	}

	onMount(() => {
		// Close popup on click outside
		function handleClickOutside(e: MouseEvent) {
			if (!popupBlockId) return;
			const target = e.target as HTMLElement;
			if (!target.closest('.annotation-popup') && !target.closest('.annotation-badge')) {
				closePopup();
			}
		}
		document.addEventListener('click', handleClickOutside);

		// Cleanup animation frames when component is destroyed
		return () => {
			document.removeEventListener('click', handleClickOutside);
			Object.keys(canvasRafIds).forEach(stopCanvasBlock);
		};
	});

	async function handleShare(blockId: string) {
		if (!pageId || !blockId) return;
		const origin = typeof window !== 'undefined' ? window.location.origin : '';
		const embedUrl = `${origin}/embed/${encodeURIComponent(pageId)}/${encodeURIComponent(blockId)}`;
		const copied = await copyTextToClipboard(embedUrl);
		if (copied) {
			shareToastBlockId = blockId;
			setTimeout(() => (shareToastBlockId = ''), 2000);
		}
	}
</script>

{#each blocks as block, index (blockIdOf(block, index))}
	{@const blockId = blockIdOf(block, index)}
	{@const hasDraft = !!draftStates[blockId]?.text?.trim()}
	{@const draftKind = draftStates[blockId]?.kind || 'note'}
	{@const dimOriginal = hasDraft && isTextual(block)}
	{@const listNumber = block.type === 'numbered' ? (() => { let n = 1; for (let i = index - 1; i >= 0; i--) { if (blocks[i].type === 'numbered') n++; else break; } return n; })() : 1}
	<div class="block-wrapper">
		<div
			id={`${anchorPrefix}${blockId}`}
			class="block"
			class:interactive={interactive}
			class:selected={interactive && selectedBlockId === blockId}
			class:proofread={hasDraft}
		>
			{#if interactive}
				<button
					type="button"
					class="select-hit"
					on:click={() => handleSelect(blockId)}
					aria-label={`Select block ${blockId} for proofread`}
				></button>
			{/if}
			<div class="block-content">
				{#if block.type === 'heading'}
					<h1 class="editable heading-1 readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</h1>
				{:else if block.type === 'heading2'}
					<h2 class="editable heading-2 readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</h2>
				{:else if block.type === 'heading3'}
					<h3 class="editable heading-3 readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</h3>
				{:else if block.type === 'bullet'}
					<div class="list-block">
						<span class="bullet">•</span>
						<div class="editable readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</div>
					</div>
				{:else if block.type === 'numbered'}
					<div class="list-block">
						<span class="number">{listNumber}.</span>
						<div class="editable readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</div>
					</div>
				{:else if block.type === 'quote'}
					<blockquote class="editable quote readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</blockquote>
				{:else if block.type === 'image'}
					{#if block.data?.url}
						<figure class="media-figure">
							<img src={block.data.url} alt={block.data.caption || 'block'} class="block-image" />
							{#if block.data.caption}
								<figcaption class="media-caption">{block.data.caption}</figcaption>
							{/if}
						</figure>
					{/if}
				{:else if block.type === 'gallery'}
					{@const items = normalizeGalleryItems(block.data)}
					{@const columns = Math.min(Math.max(Number(block.data?.columns || 2), 2), 4)}
					{#if items.length > 0}
						<div class="gallery-grid" style={`--gallery-cols: ${columns};`}>
							{#each items as item, i (item.id)}
								<div class="gallery-item" class:text-card={item.kind === 'text'}>
									{#if item.kind === 'image'}
										<img src={item.value} alt={`gallery-${i}`} class="gallery-image" />
									{:else if item.kind === 'embed'}
										<iframe src={item.value} title={`gallery-embed-${i}`} class="gallery-embed"></iframe>
									{:else}
										<div class="gallery-text">{item.value}</div>
									{/if}
								</div>
							{/each}
						</div>
				{/if}
			{:else if block.type === 'code'}
				<div class="code-block">
					<div class="code-toolbar">
						<span class="code-lang-badge">{block.data?.language || 'javascript'}</span>
						<span class="code-label">Code</span>
					</div>
					<pre class="code-readonly"><code>{block.data?.code || ''}</code></pre>
				</div>
			{:else if block.type === 'canvas'}
				{@const cBlockId = blockIdOf(block, index)}
				<figure class="media-figure">
					<div class="canvas-block canvas-clean">
						<div class="canvas-preview">
							<canvas
								use:bindCanvas={cBlockId}
								width={block.data?.width || 600}
								height={block.data?.height || 400}
								class="canvas-el"
							></canvas>
						</div>
					</div>
					{#if block.data?.caption}
						<figcaption class="media-caption">{block.data.caption}</figcaption>
					{/if}
				</figure>
			{:else if block.type === 'embed'}
				{#if block.data?.url}
					<figure class="media-figure">
						<iframe src={block.data.url} class="embed-frame" title="Embedded content"></iframe>
						{#if block.data.caption}
							<figcaption class="media-caption">{block.data.caption}</figcaption>
						{/if}
					</figure>
				{/if}
			{:else}
				<div class="editable readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</div>
			{/if}				{#if hasDraft}
					<div class="proofread-overlay" class:assert={draftKind === 'assert'} class:debunk={draftKind === 'debunk'} class:strike={draftKind === 'strike'}>
						<div class="proofread-kind">{draftKind}</div>
						<p>{draftStates[blockId].text}</p>
					</div>
				{/if}
			</div>

			{#if pageId}
				<button
					type="button"
					class="share-btn"
					title={shareToastBlockId === blockId ? 'Copied!' : 'Copy embed link'}
					on:click|stopPropagation={() => handleShare(blockId)}
				>
					{#if shareToastBlockId === blockId}
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

			<!-- Proofread annotation badge + popup -->
			{#if annotationMap[blockId]?.length}
				<button
					type="button"
					class="annotation-badge"
					class:active={popupBlockId === blockId}
					on:click|stopPropagation={() => openPopup(blockId)}
					title={`${annotationMap[blockId].length} proofread note${annotationMap[blockId].length > 1 ? 's' : ''}`}
				>
					<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>
					</svg>
					<span>{annotationMap[blockId].length}</span>
				</button>

				{#if popupBlockId === blockId && popupCurrent}
					<!-- svelte-ignore a11y-no-static-element-interactions -->
					<div class="annotation-popup" on:keydown={handlePopupKeydown}>
						<div class="popup-header">
							<div class="popup-author">
								<span class="popup-avatar">{popupCurrent.authorName.charAt(0).toUpperCase()}</span>
								<div class="popup-meta">
									<span class="popup-name">{popupCurrent.authorName}</span>
									<span class="popup-stance stance-{popupCurrent.stance}">{popupCurrent.stance}</span>
								</div>
							</div>
							<button class="popup-close" on:click|stopPropagation={closePopup} aria-label="Close">
								<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
							</button>
						</div>

						<div class="popup-kind kind-{popupCurrent.kind}">{popupCurrent.kind}</div>

						<div class="popup-body">
							<p>{popupCurrent.text}</p>
						</div>

						{#if popupAnnotations.length > 1}
							<div class="popup-nav">
								<button class="popup-arrow" disabled={popupIndex === 0} on:click|stopPropagation={popupPrev} aria-label="Previous">
									<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="15 18 9 12 15 6"/></svg>
								</button>
								<div class="popup-dots">
									{#each popupAnnotations as _, i}
										<button
											class="popup-dot"
											class:active={i === popupIndex}
											on:click|stopPropagation={() => (popupIndex = i)}
											aria-label={`Note ${i + 1}`}
										></button>
									{/each}
								</div>
								<button class="popup-arrow" disabled={popupIndex === popupAnnotations.length - 1} on:click|stopPropagation={popupNext} aria-label="Next">
									<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="9 6 15 12 9 18"/></svg>
								</button>
							</div>
						{/if}

						<a class="popup-link" href={`/proofread/${popupCurrent.proofreadId}`}>
							View full proofread →
						</a>
					</div>
				{/if}
			{/if}
		</div>
	</div>
{/each}

<style>
	.block-wrapper {
		position: relative;
		display: flex;
		align-items: flex-start;
		gap: 8px;
		margin: 0;
	}

	.block {
		position: relative;
		display: flex;
		align-items: flex-start;
		padding: 8px 10px;
		scroll-margin-top: 24px;
		border-radius: 10px;
		border: 1px solid transparent;
		transition: background 0.12s, border-color 0.12s, box-shadow 0.12s;
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.block:hover .share-btn {
		opacity: 1;
	}

	.share-btn {
		opacity: 0;
		position: absolute;
		right: -32px;
		top: 6px;
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
		z-index: 20;
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

	.block.interactive {
		cursor: pointer;
		transition: background 0.16s ease, box-shadow 0.16s ease;
	}

	.select-hit {
		position: absolute;
		inset: 0;
		z-index: 10;
		background: transparent;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}

	.block-content {
		position: relative;
		z-index: 3;
	}

	.block.interactive:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 10%, var(--note-surface, #ffffff));
		border-color: color-mix(in srgb, var(--note-border, #d1d5db) 85%, transparent);
		box-shadow: 0 10px 26px rgba(15, 23, 42, 0.08);
	}

	.block.selected {
		box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--note-accent, #7c5cff) 38%, transparent);
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 16%, var(--note-surface, #ffffff));
	}

	.block.proofread {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 6%, transparent);
	}

	.block-content {
		flex: 1;
		min-width: 0;
		width: 100%;
		position: relative;
	}

	.editable {
		outline: none;
		min-height: 1.5em;
		line-height: 1.6;
		word-break: break-word;
		color: var(--note-text, #1f2328);
	}

	.readonly-paragraph {
		white-space: pre-wrap;
	}

	.dimmed {
		opacity: 0.45;
		filter: saturate(0.65);
	}

	.strike {
		text-decoration: line-through;
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

	.bullet,
	.number {
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
	}

	/* ---- Media figure + caption (readonly) ---- */
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
		max-width: 480px;
		text-align: center;
		color: var(--note-muted, #6b7280);
		font-size: 13px;
		font-style: italic;
		font-family: inherit;
		line-height: 1.5;
		padding: 8px 8px 2px;
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

	.embed-frame {
		width: 100%;
		height: 400px;
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 12px;
		background: #000;
	}

	.proofread-overlay {
		margin-top: 8px;
		padding: 10px 12px;
		border-radius: 12px;
		border: 1px solid color-mix(in srgb, var(--note-border, #d1d5db) 90%, transparent);
		background: color-mix(in srgb, var(--note-surface, #ffffff) 84%, var(--note-accent, #7c5cff) 10%);
		box-shadow: 0 12px 30px rgba(15, 23, 42, 0.1);
	}

	.proofread-overlay.assert {
		border-color: #86efac;
		background: #f0fdf4;
	}

	.proofread-overlay.debunk {
		border-color: #fca5a5;
		background: #fef2f2;
	}

	.proofread-overlay.strike {
		border-color: #fcd34d;
		background: #fffbeb;
	}

	.proofread-kind {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 700;
		color: var(--note-muted, #6b7280);
		margin-bottom: 4px;
	}

	.proofread-overlay p {
		margin: 0;
		white-space: pre-wrap;
		line-height: 1.5;
	}

	/* ---- Code block (readonly) ---- */
	.code-block {
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 10px;
		overflow: hidden;
		background: #1e1e2e;
	}

	.code-toolbar {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 12px;
		background: #181825;
		border-bottom: 1px solid #313244;
	}

	.code-lang-badge {
		background: #313244;
		color: #cdd6f4;
		border-radius: 6px;
		padding: 3px 8px;
		font-size: 12px;
		font-weight: 600;
	}

	.code-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6c7086;
	}

	.code-readonly {
		margin: 0;
		padding: 14px 16px;
		background: #1e1e2e;
		color: #cdd6f4;
		font-family: 'JetBrains Mono', 'Fira Code', 'SF Mono', 'Consolas', monospace;
		font-size: 13px;
		line-height: 1.6;
		overflow-x: auto;
		white-space: pre;
		tab-size: 2;
	}

	.code-readonly code {
		font-family: inherit;
	}

	/* ---- Canvas block (readonly) ---- */
	.canvas-block {
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 10px;
		overflow: hidden;
		background: #1e1e2e;
	}

	.canvas-block.canvas-clean {
		border: none;
		background: transparent;
		border-radius: 0;
	}

	.canvas-block.canvas-clean .canvas-preview {
		padding: 0;
		background: transparent;
	}

	.canvas-block.canvas-clean .canvas-el {
		box-shadow: none;
		border-radius: 8px;
	}

	.canvas-toolbar-ro {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 12px;
		background: #181825;
		border-bottom: 1px solid #313244;
	}

	.canvas-label {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6c7086;
		margin-right: auto;
	}

	.canvas-dim-info {
		font-size: 11px;
		color: #585b70;
	}

	.canvas-source {
		border-bottom: 1px solid #313244;
	}

	.canvas-source summary {
		padding: 6px 14px;
		font-size: 12px;
		color: #6c7086;
		cursor: pointer;
		user-select: none;
	}

	.canvas-source summary:hover {
		color: #cdd6f4;
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

	/* ── Annotation badge (SoundCloud-style) ── */

	.annotation-badge {
		position: absolute;
		left: -34px;
		top: 8px;
		display: flex;
		align-items: center;
		gap: 3px;
		padding: 3px 7px;
		border-radius: 10px;
		border: 1.5px solid var(--note-border, #d1d5db);
		background: var(--note-surface, #ffffff);
		color: var(--note-accent, #7c5cff);
		font-size: 11px;
		font-weight: 700;
		cursor: pointer;
		opacity: 0;
		transition: opacity 0.15s, transform 0.15s, background 0.15s, border-color 0.15s;
		z-index: 20;
		transform: scale(0.92);
	}

	.block:hover .annotation-badge,
	.annotation-badge.active {
		opacity: 1;
		transform: scale(1);
	}

	.annotation-badge.active {
		background: var(--note-accent, #7c5cff);
		color: #ffffff;
		border-color: var(--note-accent, #7c5cff);
	}

	.annotation-badge svg {
		stroke: currentColor;
		flex-shrink: 0;
	}

	/* ── Annotation popup ── */

	.annotation-popup {
		position: absolute;
		left: 0;
		top: calc(100% + 6px);
		z-index: 50;
		width: min(360px, calc(100vw - 48px));
		background: var(--note-surface, #ffffff);
		border: 1.5px solid var(--note-border, #d1d5db);
		border-radius: 14px;
		box-shadow: 0 16px 48px rgba(0, 0, 0, 0.14), 0 4px 12px rgba(0, 0, 0, 0.08);
		padding: 0;
		overflow: hidden;
		animation: popup-enter 0.2s ease-out;
	}

	@keyframes popup-enter {
		from {
			opacity: 0;
			transform: translateY(-6px) scale(0.97);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	.popup-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 14px 8px;
		border-bottom: 1px solid color-mix(in srgb, var(--note-border, #e5e7eb) 50%, transparent);
	}

	.popup-author {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.popup-avatar {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 50%;
		background: var(--note-accent, #7c5cff);
		color: #ffffff;
		font-size: 12px;
		font-weight: 800;
		flex-shrink: 0;
	}

	.popup-meta {
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.popup-name {
		font-size: 13px;
		font-weight: 700;
		color: var(--note-title, #111827);
		line-height: 1.2;
	}

	.popup-stance {
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--note-muted, #6b7280);
	}

	.popup-stance.stance-assert {
		color: #16a34a;
	}

	.popup-stance.stance-debunk {
		color: #dc2626;
	}

	.popup-close {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--note-muted, #9ca3af);
		cursor: pointer;
		transition: background 0.12s, color 0.12s;
	}

	.popup-close:hover {
		background: color-mix(in srgb, var(--note-border, #e5e7eb) 60%, transparent);
		color: var(--note-title, #111827);
	}

	.popup-kind {
		margin: 8px 14px 0;
		display: inline-block;
		font-size: 10px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		padding: 2px 8px;
		border-radius: 4px;
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 12%, transparent);
		color: var(--note-accent, #7c5cff);
	}

	.popup-kind.kind-assert {
		background: #ecfdf5;
		color: #16a34a;
	}

	.popup-kind.kind-debunk {
		background: #fef2f2;
		color: #dc2626;
	}

	.popup-kind.kind-strike {
		background: #fffbeb;
		color: #d97706;
	}

	.popup-body {
		padding: 8px 14px 10px;
	}

	.popup-body p {
		margin: 0;
		font-size: 13.5px;
		line-height: 1.55;
		color: var(--note-text, #1f2328);
		white-space: pre-wrap;
		word-break: break-word;
	}

	/* ── Slideshow nav ── */

	.popup-nav {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 4px 14px 8px;
	}

	.popup-arrow {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border: 1px solid var(--note-border, #d1d5db);
		border-radius: 50%;
		background: var(--note-surface, #ffffff);
		color: var(--note-text, #374151);
		cursor: pointer;
		transition: background 0.12s, color 0.12s, border-color 0.12s;
		flex-shrink: 0;
	}

	.popup-arrow:hover:not(:disabled) {
		background: var(--note-accent, #7c5cff);
		color: #ffffff;
		border-color: var(--note-accent, #7c5cff);
	}

	.popup-arrow:disabled {
		opacity: 0.3;
		cursor: not-allowed;
	}

	.popup-dots {
		display: flex;
		align-items: center;
		gap: 5px;
	}

	.popup-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		border: none;
		padding: 0;
		background: var(--note-border, #d1d5db);
		cursor: pointer;
		transition: background 0.15s, transform 0.15s;
	}

	.popup-dot.active {
		background: var(--note-accent, #7c5cff);
		transform: scale(1.3);
	}

	.popup-link {
		display: block;
		padding: 8px 14px 10px;
		font-size: 11.5px;
		font-weight: 600;
		color: var(--note-accent, #7c5cff);
		text-decoration: none;
		border-top: 1px solid color-mix(in srgb, var(--note-border, #e5e7eb) 50%, transparent);
		transition: background 0.12s;
	}

	.popup-link:hover {
		background: color-mix(in srgb, var(--note-accent, #7c5cff) 6%, transparent);
	}

	/* ---- Mobile / responsive ---- */
	@media (max-width: 680px) {
		.block {
			padding: 6px 4px;
		}

		.heading-1 { font-size: 24px; margin: 8px 0 2px; }
		.heading-2 { font-size: 20px; margin: 6px 0 2px; }
		.heading-3 { font-size: 17px; margin: 4px 0 2px; }

		.editable {
			line-height: 1.5;
		}

		.gallery-grid {
			grid-template-columns: repeat(min(var(--gallery-cols, 2), 2), minmax(0, 1fr)) !important;
		}

		.gallery-image {
			height: 140px;
		}

		.code-editor {
			font-size: 12px;
			padding: 10px 12px;
		}

		.embed-frame {
			height: 260px;
			border-radius: 8px;
		}

		.media-caption {
			font-size: 12px;
			max-width: 100%;
			padding: 6px 4px 2px;
		}

		.annotation-badge {
			left: -4px;
			top: -6px;
			padding: 2px 5px;
			font-size: 10px;
		}

		.popup-card {
			width: calc(100vw - 48px);
			max-width: 300px;
		}
	}

	@media (max-width: 400px) {
		.gallery-grid {
			grid-template-columns: 1fr !important;
		}

		.heading-1 { font-size: 22px; }
		.heading-2 { font-size: 18px; }
		.heading-3 { font-size: 16px; }
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
