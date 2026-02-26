<script lang="ts">
	import { page } from '$app/stores';
	import { plainTextFromBlockData, htmlFromBlockData } from '$lib/editor/richtext';
	import { normalizeGalleryItems } from '$lib/editor/blocks';
	import { buildThemeStyle, DEFAULT_THEME, extractPaletteFromImage } from '$lib/editor/theme';
	import { copyTextToClipboard } from '$lib/utils/clipboard';
	import MusicPlayer from '$lib/components/MusicPlayer.svelte';
	import type { ApiBlock, Rgb } from '$lib/editor/types';
	import { onMount } from 'svelte';

	let { data }: {
		data: {
			block: ApiBlock;
			page: { id: string; title: string; cover?: string; dark_mode?: boolean; cinematic?: boolean; mood?: number; bg_color?: string; owner_username?: string; owner_display_name?: string; owner_avatar_url?: string };
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
	let authorUsername = $derived(data.page?.owner_username || '');
	let authorDisplayName = $derived(data.page?.owner_display_name || '');
	let authorAvatarUrl = $derived(data.page?.owner_avatar_url || '');
	let authorInitial = $derived((authorDisplayName || authorUsername || '?').charAt(0).toUpperCase());

	let blockText = $derived(block ? plainTextFromBlockData(block.data) : '');
	let blockHtml = $derived(block ? htmlFromBlockData(block.data) : '');
	let ogDescription = $derived(blockText ? blockText.slice(0, 200) : `A block from "${pageTitle}"`);
	let ogTitle = $derived(blockText ? `${pageTitle} — "${blockText.slice(0, 60)}${blockText.length > 60 ? '…' : ''}"` : `${pageTitle} — Block embed`);
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

	async function copyEmbedLink() {
		const copiedOk = await copyTextToClipboard(currentUrl);
		if (copiedOk) {
			copied = true;
			setTimeout(() => (copied = false), 2000);
		}
	}
</script>

<svelte:head>
	<title>{ogTitle}</title>
	<meta property="og:title" content={ogTitle} />
	<meta property="og:description" content={ogDescription} />
	<meta property="og:type" content="article" />
	{#if ogImage}
		<meta property="og:image" content={ogImage} />
		<meta name="twitter:card" content="summary_large_image" />
	{:else}
		<meta name="twitter:card" content="summary" />
	{/if}
	<meta name="twitter:title" content={ogTitle} />
	<meta name="twitter:description" content={ogDescription} />
	{#if ogImage}
		<meta name="twitter:image" content={ogImage} />
	{/if}
</svelte:head>

<div class="embed-shell" class:dark={darkMode} style="{themeStyle}{bgColor ? `--note-user-bg:${bgColor};` : ''}">
	{#if block}
		<main class="embed-main" class:has-bg-color={!!bgColor}>
			<div class="embed-card" class:dark={darkMode}>
				<div class="card-visual">
					{#if block.type === 'image' && block.data?.url}
						<img src={block.data.url} alt="block" />
					{:else if block.type === 'music' && block.data?.coverUrl}
						<img src={block.data.coverUrl} alt={block.data.title || 'Music'} />
					{:else if pageCover}
						<img src={pageCover} alt={pageTitle} />
					{:else}
						<div class="card-default-icon">✦</div>
					{/if}
				</div>

				<div class="card-body">
					<span class="card-tag">{pageTitle ? pageTitle.split(' ').slice(0, 3).join(' ').toUpperCase() : 'EMBED'}</span>

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
							{/if}					{:else if block.type === 'music'}
						{#if block.data?.url}
							<div class="music-embed-wrap">
								<MusicPlayer
									url={block.data.url}
									title={block.data.title || ''}
									artist={block.data.artist || ''}
									coverUrl={block.data.coverUrl || ''}
									readonly={true}
									pageId={pageId}
								/>
							</div>
						{/if}						{:else}
							<div class="editable">{@html blockHtml}</div>
						{/if}
					</div>

					<div class="card-meta">
						<a href="/user/{authorUsername}" class="card-author">
							{#if authorAvatarUrl}
								<img class="card-author-avatar" src={authorAvatarUrl} alt={authorDisplayName || authorUsername} />
							{:else}
								<span class="card-author-letter">{authorInitial}</span>
							{/if}
							<span class="card-author-name">{authorDisplayName || authorUsername || 'Anonymous'}</span>
						</a>
						<button class="copy-btn" onclick={copyEmbedLink} title={copied ? 'Copied!' : 'Copy link'}>
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

					<div class="card-read-more">
						<a href={publicPageUrl} class="view-full">View full page</a>
						<span class="card-arrow">→</span>
					</div>
				</div>
			</div>
		</main>
	{/if}
</div>

<style>
	:global(body) {
		margin: 0;
		font-family: 'Moderat', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
		background: #f5f5f3;
		color: #1a1a1a;
		overflow-x: hidden;
	}

	.embed-shell {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
		box-sizing: border-box;
		background: #f5f5f3;
		transition: background 0.2s;
		overflow: hidden;
	}

	.embed-shell.dark {
		background: #111;
	}

	.embed-main {
		width: 100%;
		max-width: 420px;
		box-sizing: border-box;
	}

	/* ━━ CARD ━━ */
	.embed-card {
		display: flex;
		flex-direction: column;
		width: 100%;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		overflow: hidden;
		box-sizing: border-box;
		transition: transform 0.14s ease, box-shadow 0.14s ease;
	}

	.embed-card:hover {
		transform: translateY(-4px);
		box-shadow: 6px 6px 0 #1a1a1a;
	}

	/* ━━ CARD VISUAL ━━ */
	.card-visual {
		width: 100%;
		min-height: 160px;
		background: #e8e8e4;
		overflow: hidden;
		display: flex;
		align-items: center;
		justify-content: center;
		position: relative;
	}

	.card-visual img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
		position: absolute;
		inset: 0;
	}

	.card-default-icon {
		font-size: 40px;
		opacity: 0.2;
		color: #1a1a1a;
		user-select: none;
	}

	/* ━━ CARD BODY ━━ */
	.card-body {
		padding: 14px 16px 12px;
		display: flex;
		flex-direction: column;
		gap: 5px;
	}

	.card-tag {
		display: inline-block;
		font-size: 9px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: #1a1a1a;
		align-self: flex-start;
		border-bottom: 2px solid #1a1a1a;
		padding-bottom: 2px;
	}

	/* ━━ BLOCK BODY ━━ */
	.block-body {
		min-height: 24px;
		overflow: hidden;
		margin-top: 4px;
	}

	.editable {
		outline: none;
		min-height: 1.5em;
		line-height: 1.6;
		word-break: break-word;
		color: #1a1a1a;
		white-space: pre-wrap;
		font-size: 14px;
	}

	.heading-1 {
		font-size: 15px;
		font-weight: 800;
		line-height: 1.35;
		margin: 3px 0 0;
		color: #1a1a1a;
		letter-spacing: -0.02em;
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.heading-2 {
		font-size: 14px;
		font-weight: 800;
		line-height: 1.35;
		margin: 3px 0 0;
		color: #1a1a1a;
		letter-spacing: -0.02em;
	}

	.heading-3 {
		font-size: 13px;
		font-weight: 700;
		line-height: 1.3;
		margin: 3px 0 0;
		color: #1a1a1a;
	}

	.list-block {
		display: flex;
		align-items: flex-start;
		gap: 6px;
	}

	.bullet, .number {
		color: #999;
		flex-shrink: 0;
		padding-top: 2px;
		font-size: 13px;
	}

	.quote {
		border-left: 3px solid #374151;
		padding-left: 12px;
		margin: 0;
		color: #374151;
		font-style: italic;
		font-weight: 500;
		font-size: 13px;
	}

	.divider-wrap {
		display: flex;
		align-items: center;
		min-height: 20px;
		padding: 4px 0;
	}

	.divider {
		border: none;
		border-top: 1px solid #e0dfdc;
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
		gap: 6px;
	}

	.gallery-item {
		overflow: hidden;
		border-radius: 6px;
		background: #f3f4f6;
	}

	.gallery-item.text-card {
		background: #f0eef6;
	}

	.gallery-image {
		width: 100%;
		height: 120px;
		object-fit: cover;
		display: block;
	}

	.gallery-embed {
		width: 100%;
		height: 120px;
		border: none;
		display: block;
		background: #000;
	}

	.gallery-text {
		min-height: 80px;
		padding: 10px;
		line-height: 1.45;
		font-size: 12px;
		color: #1a1a1a;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.embed-frame {
		width: 100%;
		height: 200px;
		border: none;
		border-radius: 6px;
		background: #000;
	}

	/* ━━ CARD META / AUTHOR ━━ */
	.card-meta {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-top: 6px;
	}

	.card-author {
		display: flex;
		align-items: center;
		gap: 6px;
		text-decoration: none;
		min-width: 0;
	}

	.card-author-avatar {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		object-fit: cover;
		border: 1.5px solid #1a1a1a;
		flex-shrink: 0;
	}

	.card-author-letter {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: #1a1a1a;
		color: #fff;
		font-size: 10px;
		font-weight: 800;
		flex-shrink: 0;
	}

	.card-author-name {
		font-size: 11px;
		font-weight: 700;
		color: #1a1a1a;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 160px;
		transition: opacity 0.15s;
	}

	.card-author:hover .card-author-name {
		opacity: 0.5;
	}

	.copy-btn {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		color: #999;
		transition: color 0.12s;
		padding: 0;
		flex-shrink: 0;
	}

	.copy-btn:hover {
		color: #1a1a1a;
	}

	.music-embed-wrap {
		width: 100%;
		border-radius: 12px;
		overflow: hidden;
	}

	.copy-btn svg {
		width: 13px;
		height: 13px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	/* ━━ READ MORE BAR ━━ */
	.card-read-more {
		margin-top: 8px;
		padding-top: 10px;
		border-top: 1px solid #e0dfdc;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.view-full {
		font-size: 10px;
		font-weight: 800;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: #1a1a1a;
		text-decoration: none;
		transition: opacity 0.12s;
	}

	.view-full:hover {
		opacity: 0.5;
	}

	.card-arrow {
		font-size: 14px;
		color: #1a1a1a;
		transition: transform 0.15s;
	}

	.embed-card:hover .card-arrow {
		transform: translateX(3px);
	}

	/* ━━ DARK MODE ━━ */
	.embed-card.dark {
		background: #0a0a0a;
		border-color: #333;
		color: #e8e8e8;
	}

	.embed-card.dark:hover {
		box-shadow: 6px 6px 0 #444;
	}

	.embed-card.dark .card-visual {
		background: #111;
	}

	.embed-card.dark .card-body {
		background: #0a0a0a;
	}

	.embed-card.dark .card-tag {
		color: #ccc;
		border-bottom-color: #555;
	}

	.embed-card.dark .heading-1,
	.embed-card.dark .heading-2,
	.embed-card.dark .heading-3 {
		color: #fff;
	}

	.embed-card.dark .editable {
		color: #e0e0e0;
	}

	.embed-card.dark .card-author-name {
		color: #ccc;
	}

	.embed-card.dark .card-author-letter {
		background: #444;
		color: #fff;
	}

	.embed-card.dark .card-author-avatar {
		border-color: #555;
	}

	.embed-card.dark .card-read-more {
		border-top-color: #2a2a2a;
	}

	.embed-card.dark .view-full,
	.embed-card.dark .card-arrow {
		color: #ccc;
	}

	.embed-card.dark .copy-btn {
		color: #666;
	}

	.embed-card.dark .copy-btn:hover {
		color: #ccc;
	}

	.embed-card.dark .card-default-icon {
		color: #fff;
	}

	.embed-card.dark .quote {
		border-left-color: #555;
		color: #aaa;
	}

	.embed-card.dark .bullet,
	.embed-card.dark .number {
		color: #666;
	}

	.embed-card.dark .divider {
		border-top-color: #333;
	}

	/* ━━ Mobile ━━ */
	@media (max-width: 680px) {
		.embed-shell {
			padding: 12px;
		}

		.embed-main {
			max-width: 100%;
		}

		.card-visual {
			min-height: 120px;
		}

		.card-body {
			padding: 12px 12px 10px;
		}

		.gallery-grid {
			grid-template-columns: repeat(min(var(--gallery-cols, 2), 2), minmax(0, 1fr)) !important;
		}

		.gallery-image,
		.gallery-embed {
			height: 100px;
		}

		.embed-frame {
			height: 160px;
		}
	}

	@media (max-width: 400px) {
		.embed-shell {
			padding: 6px;
		}

		.card-body {
			padding: 10px 10px 8px;
		}

		.gallery-grid {
			grid-template-columns: 1fr !important;
		}

		.embed-frame {
			height: 140px;
		}
	}
</style>
