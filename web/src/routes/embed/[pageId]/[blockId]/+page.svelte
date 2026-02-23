<script lang="ts">
	import { page } from '$app/stores';
	import { plainTextFromBlockData, htmlFromBlockData } from '$lib/editor/richtext';
	import { normalizeGalleryItems } from '$lib/editor/blocks';
	import { buildThemeStyle, DEFAULT_THEME, extractPaletteFromImage } from '$lib/editor/theme';
	import type { ApiBlock, Rgb } from '$lib/editor/types';
	import { onMount } from 'svelte';

	let { data }: {
		data: {
			block: ApiBlock;
			page: { id: string; title: string; cover?: string; dark_mode?: boolean; cinematic?: boolean; mood?: number; bg_color?: string };
		};
	} = $props();

	const FALLBACK_BASE: Rgb = [205, 207, 214];
	const FALLBACK_ACCENT: Rgb = [124, 92, 255];
	let themeStyle = $state(DEFAULT_THEME);
	let copied = $state(false);

	let block = $derived(data.block);
	let pageTitle = $derived(data.page?.title || 'Untitled');
	let pageId = $derived(data.page?.id || $page.params.pageId);
	let pageCover = $derived(data.page?.cover || null);
	let darkMode = $derived(!!data.page?.dark_mode);
	let cinematicEnabled = $derived(data.page?.cinematic !== false);
	let moodStrength = $derived(Number(data.page?.mood ?? 65));
	let bgColor = $derived(data.page?.bg_color || '');

	let blockText = $derived(block ? plainTextFromBlockData(block.data) : '');
	let blockHtml = $derived(block ? htmlFromBlockData(block.data) : '');
	let ogDescription = $derived(blockText ? blockText.slice(0, 200) : `A block from "${pageTitle}"`);
	let ogImage = $derived(block?.data?.url && (block.type === 'image') ? block.data.url : pageCover);
	let publicPageUrl = $derived(`/public/${pageId}`);
	let currentUrl = $derived(typeof window !== 'undefined' ? window.location.href : '');

	async function applyCoverPalette(imageSrc: string | null) {
		const palette = await extractPaletteFromImage(imageSrc, FALLBACK_BASE, FALLBACK_ACCENT);
		themeStyle = buildThemeStyle(palette.base, palette.accent, { darkMode, cinematicEnabled, moodStrength });
	}

	onMount(() => {
		void applyCoverPalette(pageCover);
	});

	$effect(() => {
		if (typeof window !== 'undefined') {
			void applyCoverPalette(pageCover);
		}
	});

	function copyEmbedLink() {
		if (typeof navigator !== 'undefined' && navigator.clipboard) {
			navigator.clipboard.writeText(currentUrl);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		}
	}
</script>

<svelte:head>
	<title>{pageTitle} — Block embed</title>
	<meta property="og:title" content={pageTitle} />
	<meta property="og:description" content={ogDescription} />
	<meta property="og:type" content="article" />
	{#if ogImage}
		<meta property="og:image" content={ogImage} />
		<meta name="twitter:card" content="summary_large_image" />
	{:else}
		<meta name="twitter:card" content="summary" />
	{/if}
	<meta name="twitter:title" content={pageTitle} />
	<meta name="twitter:description" content={ogDescription} />
	{#if ogImage}
		<meta name="twitter:image" content={ogImage} />
	{/if}
</svelte:head>

<div class="embed-shell" class:dark={darkMode} style="{themeStyle}{bgColor ? `--note-user-bg:${bgColor};` : ''}">
	{#if block}
		<main class="embed-main" class:has-bg-color={!!bgColor}>
			<div class="embed-card">
				<div class="embed-header">
					<a href={publicPageUrl} class="page-link" title="View full page">
						<svg viewBox="0 0 24 24" class="page-icon" aria-hidden="true">
							<path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
							<polyline points="14 2 14 8 20 8" />
						</svg>
						<span class="page-title">{pageTitle}</span>
					</a>
					<button class="copy-btn" on:click={copyEmbedLink} title="Copy embed link">
						{#if copied}
							<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>
						{:else}
							<svg viewBox="0 0 24 24" aria-hidden="true">
								<rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
								<path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
							</svg>
						{/if}
					</button>
				</div>

				<div class="block-body">
					{#if block.type === 'heading'}
						<h1 class="editable heading-1">{@html blockHtml}</h1>
					{:else if block.type === 'heading2'}
						<h2 class="editable heading-2">{@html blockHtml}</h2>
					{:else if block.type === 'heading3'}
						<h3 class="editable heading-3">{@html blockHtml}</h3>
					{:else if block.type === 'bullet'}
						<div class="list-block">
							<span class="bullet">•</span>
							<div class="editable">{@html blockHtml}</div>
						</div>
					{:else if block.type === 'numbered'}
						<div class="list-block">
							<span class="number">{block.data?.number || 1}.</span>
							<div class="editable">{@html blockHtml}</div>
						</div>
					{:else if block.type === 'quote'}
						<blockquote class="editable quote">{@html blockHtml}</blockquote>
					{:else if block.type === 'divider'}
						<div class="divider-wrap"><hr class="divider" /></div>
					{:else if block.type === 'image'}
						{#if block.data?.url}
							<img src={block.data.url} alt="block" class="block-image" />
						{/if}
					{:else if block.type === 'gallery'}
						{@const items = normalizeGalleryItems(block.data)}
						{@const columns = Math.min(Math.max(Number(block.data?.columns || 2), 2), 4)}
						{#if items.length > 0}
							<div class="gallery-grid" style="--gallery-cols: {columns};">
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
						<div class="editable">{@html blockHtml}</div>
					{/if}
				</div>

				<div class="embed-footer">
					<a href={publicPageUrl} class="view-full">View full page →</a>
				</div>
			</div>
		</main>
	{/if}
</div>

<style>
	:global(body) {
		margin: 0;
		font-family: 'Moderat', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
		background: var(--note-bg, #ffffff);
		color: var(--note-text, #1f2328);
		transition: background 0.2s, color 0.2s;
	}

	.embed-shell {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
		box-sizing: border-box;
		background: var(--note-bg, #ffffff);
		transition: background 0.2s;
	}

	.embed-main {
		width: 100%;
		max-width: 640px;
		background: var(--note-bg, #ffffff);
		border-radius: 8px;
		overflow: hidden;
	}

	.embed-main.has-bg-color {
		--note-bg: var(--note-user-bg);
		--note-surface: var(--note-user-bg);
	}

	.embed-card {
		border: 2px solid var(--note-title, #1a1a1a);
		border-radius: 8px;
		background: var(--note-surface, #ffffff);
		padding: 24px 28px;
		box-shadow: 6px 6px 0 var(--note-title, #1a1a1a);
		transition: transform 0.12s ease, box-shadow 0.12s ease;
	}

	.embed-card:hover {
		transform: translateY(-3px);
		box-shadow: 8px 8px 0 var(--note-title, #1a1a1a);
	}

	.embed-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 18px;
	}

	.page-link {
		display: flex;
		align-items: center;
		gap: 8px;
		text-decoration: none;
		color: var(--note-title, #1a1a1a);
		transition: transform 0.12s;
		min-width: 0;
	}

	.page-link:hover {
		color: var(--note-title, #1a1a1a);
		transform: translateX(2px);
	}

	.page-icon {
		width: 16px;
		height: 16px;
		flex-shrink: 0;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.page-title {
		font-size: 13px;
		font-weight: 800;
		letter-spacing: 0.03em;
		text-transform: uppercase;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.copy-btn {
		flex-shrink: 0;
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		color: var(--note-muted, #888);
		transition: color 0.12s;
	}

	.copy-btn:hover {
		color: var(--note-title, #1a1a1a);
	}

	.copy-btn svg {
		width: 14px;
		height: 14px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.block-body {
		min-height: 24px;
	}

	.editable {
		outline: none;
		min-height: 1.5em;
		line-height: 1.6;
		word-break: break-word;
		color: var(--note-text, #1f2328);
		white-space: pre-wrap;
	}

	.heading-1 {
		font-size: 28px;
		font-weight: 800;
		line-height: 1.25;
		margin: 0;
		color: var(--note-title, #1a1a1a);
		letter-spacing: -0.01em;
	}

	.heading-2 {
		font-size: 22px;
		font-weight: 800;
		line-height: 1.25;
		margin: 0;
		color: var(--note-title, #1a1a1a);
		letter-spacing: -0.01em;
	}

	.heading-3 {
		font-size: 18px;
		font-weight: 700;
		line-height: 1.3;
		margin: 0;
		color: var(--note-title, #1a1a1a);
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
		border-left: 3px solid var(--note-quote, #374151);
		padding-left: 16px;
		margin: 0;
		color: var(--note-quote, #374151);
		font-style: italic;
		font-weight: 500;
	}

	.divider-wrap {
		display: flex;
		align-items: center;
		min-height: 26px;
		padding: 6px 0;
	}

	.divider {
		border: none;
		border-top: 1px solid var(--note-border, #d1d5db);
		width: 100%;
		margin: 0;
	}

	.block-image {
		max-width: 100%;
		border-radius: 6px;
	}

	.gallery-grid {
		display: grid;
		grid-template-columns: repeat(var(--gallery-cols, 2), minmax(0, 1fr));
		gap: 8px;
	}

	.gallery-item {
		overflow: hidden;
		border-radius: 6px;
		background: var(--note-surface, #f3f4f6);
	}

	.gallery-item.text-card {
		background: color-mix(in srgb, var(--note-surface, #ffffff) 88%, var(--note-accent, #7c5cff) 10%);
	}

	.gallery-image {
		width: 100%;
		height: 160px;
		object-fit: cover;
		display: block;
	}

	.gallery-embed {
		width: 100%;
		height: 160px;
		border: none;
		display: block;
		background: #000;
	}

	.gallery-text {
		min-height: 100px;
		padding: 12px;
		line-height: 1.45;
		font-size: 13px;
		color: var(--note-text, #1f2328);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.embed-frame {
		width: 100%;
		height: 320px;
		border: none;
		border-radius: 6px;
		background: #000;
	}

	.embed-footer {
		margin-top: 20px;
		display: flex;
		justify-content: flex-end;
	}

	.view-full {
		font-size: 11px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--note-muted, #888);
		text-decoration: none;
		transition: color 0.12s;
	}

	.view-full:hover {
		color: var(--note-title, #1a1a1a);
	}

</style>
