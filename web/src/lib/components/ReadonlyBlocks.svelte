<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import type { ApiBlock } from '$lib/editor/types';
	import { normalizeGalleryItems } from '$lib/editor/blocks';
	import { htmlFromBlockData } from '$lib/editor/richtext';
	import { copyTextToClipboard } from '$lib/utils/clipboard';

	export let blocks: ApiBlock[] = [];
	export let interactive = false;
	export let selectedBlockId = '';
	export let draftStates: Record<string, { kind: string; text: string }> = {};
	export let anchorPrefix = 'block-';
	export let pageId = '';

	const dispatch = createEventDispatcher<{ select: { blockId: string } }>();

	let shareToastBlockId = '';

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
						<span class="number">{block.data?.number || index + 1}.</span>
						<div class="editable readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</div>
					</div>
				{:else if block.type === 'quote'}
					<blockquote class="editable quote readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</blockquote>
				{:else if block.type === 'divider'}
					<div class="divider-wrap">
						<hr class="divider" />
					</div>
				{:else if block.type === 'image'}
					{#if block.data?.url}
						<img src={block.data.url} alt="block" class="block-image" />
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
				{:else if block.type === 'embed'}
					{#if block.data?.url}
						<iframe src={block.data.url} class="embed-frame" title="Embedded content"></iframe>
					{/if}
				{:else}
					<div class="editable readonly-paragraph" class:dimmed={dimOriginal} class:strike={dimOriginal && draftKind === 'strike'}>{@html htmlOf(block)}</div>
				{/if}

				{#if hasDraft}
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
</style>
