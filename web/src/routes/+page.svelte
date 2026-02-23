<script lang="ts">
	import { goto } from '$app/navigation';
	import { env } from '$env/dynamic/public';
	import { onMount } from 'svelte';
	import type { ApiPage } from '$lib/editor/types';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let pages: ApiPage[] = [];
	let loading = true;
	let error = '';
	let pageIdInput = '';

	/** Per-card cinematic tint extracted from cover image */
	let cardTints: Record<string, { bg: string; border: string; shadow: string; muted: string }> = {};

	/* beautiful default cover patterns – each card gets a deterministic one */
	const defaultPatterns = [
		'repeating-linear-gradient(45deg, #e8e8e4 0px, #e8e8e4 10px, #f5f5f3 10px, #f5f5f3 20px)',
		'repeating-linear-gradient(-45deg, #e8e8e4 0px, #e8e8e4 10px, #f5f5f3 10px, #f5f5f3 20px)',
		'repeating-linear-gradient(90deg, #e8e8e4 0px, #e8e8e4 8px, #f5f5f3 8px, #f5f5f3 16px)',
		'radial-gradient(circle at 20% 30%, #e0e0dc 2px, transparent 2px), radial-gradient(circle at 70% 60%, #e0e0dc 2px, transparent 2px), radial-gradient(circle at 40% 80%, #e0e0dc 2px, transparent 2px)',
		'repeating-conic-gradient(#e8e8e4 0% 25%, #f5f5f3 0% 50%) 0 0 / 20px 20px',
		'linear-gradient(135deg, #e8e8e4 25%, transparent 25%) -10px 0, linear-gradient(225deg, #e8e8e4 25%, transparent 25%) -10px 0, linear-gradient(315deg, #e8e8e4 25%, transparent 25%), linear-gradient(45deg, #e8e8e4 25%, transparent 25%)',
	];

	function patternFor(page: ApiPage): string {
		let hash = 0;
		const id = page.id || '';
		for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) | 0;
		return defaultPatterns[Math.abs(hash) % defaultPatterns.length];
	}

	/** Quick dominant color extraction from an image for cinematic card tinting */
	function extractQuickTint(imgSrc: string, pageId: string) {
		const img = new Image();
		img.crossOrigin = 'anonymous';
		img.onload = () => {
			try {
				const canvas = document.createElement('canvas');
				const ctx = canvas.getContext('2d', { willReadFrequently: true });
				if (!ctx) return;
				canvas.width = 16;
				canvas.height = 16;
				ctx.drawImage(img, 0, 0, 16, 16);
				const { data } = ctx.getImageData(0, 0, 16, 16);
				let r = 0, g = 0, b = 0, n = 0;
				for (let i = 0; i < data.length; i += 4) {
					if (data[i + 3] < 150) continue;
					r += data[i]; g += data[i + 1]; b += data[i + 2]; n++;
				}
				if (n === 0) return;
				r = Math.round(r / n); g = Math.round(g / n); b = Math.round(b / n);
				const hex = (v: number) => Math.max(0, Math.min(255, v)).toString(16).padStart(2, '0');
				/* mix dominant color into card surfaces at low opacity for subtle tinting */
				const bgR = Math.round(r * 0.08 + 250 * 0.92);
				const bgG = Math.round(g * 0.08 + 249 * 0.92);
				const bgB = Math.round(b * 0.08 + 247 * 0.92);
				const brR = Math.round(r * 0.3 + 180 * 0.7);
				const brG = Math.round(g * 0.3 + 175 * 0.7);
				const brB = Math.round(b * 0.3 + 168 * 0.7);
				const shR = Math.round(r * 0.25 + 140 * 0.75);
				const shG = Math.round(g * 0.25 + 135 * 0.75);
				const shB = Math.round(b * 0.25 + 128 * 0.75);
				const muR = Math.round(r * 0.15 + 110 * 0.85);
				const muG = Math.round(g * 0.15 + 105 * 0.85);
				const muB = Math.round(b * 0.15 + 100 * 0.85);
				cardTints = { ...cardTints, [pageId]: {
					bg: `#${hex(bgR)}${hex(bgG)}${hex(bgB)}`,
					border: `#${hex(brR)}${hex(brG)}${hex(brB)}`,
					shadow: `#${hex(shR)}${hex(shG)}${hex(shB)}`,
					muted: `#${hex(muR)}${hex(muG)}${hex(muB)}`,
				}};
			} catch { /* ignore cross-origin errors */ }
		};
		img.src = imgSrc;
	}

	/** Build inline style vars for cinematic cards + bg_color */
	function cinematicStyle(page: ApiPage): string {
		const parts: string[] = [];
		const t = cardTints[page.id];
		if (t) {
			parts.push(`--card-bg:${t.bg}`, `--card-border:${t.border}`, `--card-shadow:${t.shadow}`, `--card-muted:${t.muted}`);
		}
		if (page.bg_color) {
			parts.push(`--card-user-bg:${page.bg_color}`);
		}
		return parts.join(';');
	}

	onMount(async () => {
		try {
			const res = await fetch(`${apiUrl}/v1/pages`);
			if (!res.ok) throw new Error('Failed to load pages');
			const payload = await res.json();
			pages = payload?.items ?? [];

			/* extract cover tints for cinematic pages */
			for (const page of pages) {
				if (!page.cinematic) continue;
				const img = imageFor(page);
				if (img) extractQuickTint(img, page.id);
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load pages';
		} finally {
			loading = false;
		}
	});

	async function openPageById(e: SubmitEvent) {
		e.preventDefault();
		const id = pageIdInput.trim();
		if (!id) return;
		await goto(`/editor/${encodeURIComponent(id)}`);
	}

	function formatDate(iso?: string) {
		if (!iso) return '';
		return new Date(iso).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	/** Extract image url from page: cover → image block → gallery item → null */
	function imageFor(page: ApiPage): string | null {
		if (page.cover) return page.cover;
		if (page.blocks) {
			for (const b of page.blocks) {
				if (b.type === 'image' && b.data?.url) return b.data.url;
				if (b.type === 'gallery' && Array.isArray(b.data?.items)) {
					const img = b.data.items.find((i: any) => i.kind === 'image' && i.value);
					if (img) return img.value;
				}
			}
		}
		return null;
	}

	/** Extract embed url from page blocks */
	function embedFor(page: ApiPage): string | null {
		if (page.blocks) {
			for (const b of page.blocks) {
				if (b.type === 'embed' && b.data?.url) return b.data.url;
				if (b.type === 'gallery' && Array.isArray(b.data?.items)) {
					const emb = b.data.items.find((i: any) => i.kind === 'embed' && i.value);
					if (emb) return emb.value;
				}
			}
		}
		return null;
	}

	$: published = pages.filter((p) => p.published);
	$: drafts = pages.filter((p) => !p.published);
</script>

<div class="dashboard">
	<!-- NAV -->
	<header class="nav">
		<a href="/" class="brand">Jot.</a>
		<nav class="nav-links">
			<a href="/">Home.</a>
			<a href="/editor">Editor.</a>
		</nav>
		<a class="nav-cta" href="/editor">+ New page</a>
	</header>

	<!-- HERO -->
	<section class="hero">
		<h1>Your Pages</h1>
		<form class="open-form" on:submit={openPageById}>
			<input type="text" placeholder="Open page by ID…" bind:value={pageIdInput} />
			<button type="submit">Open →</button>
		</form>
	</section>

	{#if loading}
		<div class="status">
			<div class="spinner"></div>
			<span>Loading pages…</span>
		</div>
	{:else if error}
		<div class="status error-text">{error}</div>
	{:else if pages.length === 0}
		<div class="empty">
			<div class="empty-icon">✎</div>
			<p>No pages yet.</p>
			<a href="/editor" class="empty-cta">Create your first page →</a>
		</div>
	{:else}
		{#if published.length > 0}
			<section class="section">
				<h2 class="section-title">Published</h2>
				<div class="masonry">
					{#each published as page, idx (page.id)}
						{@const img = imageFor(page)}
						{@const emb = embedFor(page)}
						<a class="card" href={`/public/${page.id}`} class:tall={idx % 3 === 0} class:dark={page.dark_mode} class:cinematic={page.cinematic} class:has-user-bg={!!page.bg_color} style={cinematicStyle(page)}>
							<div class="card-visual" style={!img && !emb ? `background:${patternFor(page)}` : ''}>
								{#if img}
									<img src={img} alt={page.title || 'Page image'} />
								{:else if emb}
									<iframe src={emb} title="Embedded content" loading="lazy" sandbox="allow-scripts allow-same-origin"></iframe>
								{:else}
									<div class="card-default-icon">✦</div>
								{/if}
							</div>
							<div class="card-body">
								<div class="card-top-row">
									<span class="card-tag">Published</span>
									<button type="button" class="card-edit" on:click|preventDefault|stopPropagation={() => goto(`/editor/${page.id}`)}>✎ Edit</button>
								</div>
								<h3 class="card-title">{page.title || 'Untitled'}</h3>
								<div class="card-stats">
									<span><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" /><polyline points="10 9 9 9 8 9" /></svg> {page.block_count ?? 0} notes</span>
									<span><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 20h4l10-10-4-4L4 16v4z" /><path d="M12 6l4 4" /></svg> {page.proofread_count ?? 0} proofreads</span>
								</div>
								<div class="card-meta"><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><rect x="3" y="4" width="18" height="18" rx="2" ry="2" /><line x1="16" y1="2" x2="16" y2="6" /><line x1="8" y1="2" x2="8" y2="6" /><line x1="3" y1="10" x2="21" y2="10" /></svg> {formatDate(page.published_at || page.updated_at)}</div>
							</div>
						</a>
					{/each}
				</div>
			</section>
		{/if}

		{#if drafts.length > 0}
			<section class="section">
				<h2 class="section-title">Drafts</h2>
				<div class="masonry">
					{#each drafts as page, idx (page.id)}
						{@const img = imageFor(page)}
						{@const emb = embedFor(page)}
						<a class="card draft" href={`/editor/${page.id}`} class:tall={idx % 4 === 1} class:dark={page.dark_mode} class:cinematic={page.cinematic} class:has-user-bg={!!page.bg_color} style={cinematicStyle(page)}>
							<div class="card-visual" style={!img && !emb ? `background:${patternFor(page)}` : ''}>
								{#if img}
									<img src={img} alt={page.title || 'Page image'} />
								{:else if emb}
									<iframe src={emb} title="Embedded content" loading="lazy" sandbox="allow-scripts allow-same-origin"></iframe>
								{:else}
									<div class="card-default-icon">✎</div>
								{/if}
							</div>
							<div class="card-body">
								<div class="card-top-row">
									<span class="card-tag draft-tag">Draft</span>
									<button type="button" class="card-edit" on:click|preventDefault|stopPropagation={() => goto(`/editor/${page.id}`)}>✎ Edit</button>
								</div>
								<h3 class="card-title">{page.title || 'Untitled'}</h3>
								<div class="card-stats">
									<span><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /><line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" /><polyline points="10 9 9 9 8 9" /></svg> {page.block_count ?? 0} notes</span>
									<span><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 20h4l10-10-4-4L4 16v4z" /><path d="M12 6l4 4" /></svg> {page.proofread_count ?? 0} proofreads</span>
								</div>
								<div class="card-meta"><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><rect x="3" y="4" width="18" height="18" rx="2" ry="2" /><line x1="16" y1="2" x2="16" y2="6" /><line x1="8" y1="2" x2="8" y2="6" /><line x1="3" y1="10" x2="21" y2="10" /></svg> {formatDate(page.updated_at)}</div>
							</div>
						</a>
					{/each}
				</div>
			</section>
		{/if}
	{/if}
</div>

<style>
	:global(body) {
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	.dashboard {
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 28px 80px;
	}

	/* ---- NAV ---- */
	.nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 24px 0;
		border-bottom: 2px solid #1a1a1a;
	}

	.brand {
		font-size: 24px;
		font-weight: 800;
		color: #1a1a1a;
		text-decoration: none;
		letter-spacing: -0.04em;
	}

	.nav-links {
		display: flex;
		gap: 28px;
	}

	.nav-links a {
		font-size: 15px;
		font-weight: 500;
		color: #1a1a1a;
		text-decoration: none;
		transition: opacity 0.15s;
	}

	.nav-links a:hover {
		opacity: 0.5;
	}

	.nav-cta {
		font-size: 14px;
		font-weight: 600;
		color: #fff;
		background: #1a1a1a;
		padding: 8px 18px;
		border-radius: 6px;
		text-decoration: none;
		transition: background 0.15s;
	}

	.nav-cta:hover {
		background: #333;
	}

	/* ---- HERO ---- */
	.hero {
		padding: 52px 0 36px;
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: 20px;
	}

	.hero h1 {
		font-size: clamp(32px, 5vw, 52px);
		font-weight: 800;
		letter-spacing: -0.04em;
		margin: 0;
		text-transform: uppercase;
	}

	.open-form {
		display: flex;
		gap: 0;
	}

	.open-form input {
		padding: 10px 14px;
		border: 2px solid #1a1a1a;
		border-right: none;
		border-radius: 6px 0 0 6px;
		background: #fff;
		font-size: 14px;
		outline: none;
		min-width: 200px;
	}

	.open-form input::placeholder {
		color: #999;
	}

	.open-form button {
		padding: 10px 18px;
		border: 2px solid #1a1a1a;
		border-radius: 0 6px 6px 0;
		background: #1a1a1a;
		color: #fff;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
		transition: background 0.15s;
	}

	.open-form button:hover {
		background: #333;
	}

	/* ---- STATUS ---- */
	.status {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 80px 20px;
		font-size: 16px;
		color: #888;
	}

	.spinner {
		width: 18px;
		height: 18px;
		border: 2px solid #ddd;
		border-top-color: #1a1a1a;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.error-text {
		color: #c00;
	}

	.empty {
		text-align: center;
		padding: 80px 20px;
	}

	.empty-icon {
		font-size: 48px;
		margin-bottom: 16px;
		opacity: 0.3;
	}

	.empty p {
		font-size: 18px;
		color: #888;
		margin: 0 0 16px;
	}

	.empty-cta {
		font-size: 15px;
		font-weight: 600;
		color: #1a1a1a;
		text-decoration: underline;
		text-underline-offset: 3px;
	}

	/* ---- SECTIONS ---- */
	.section {
		margin-top: 20px;
	}

	.section-title {
		font-size: 13px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.14em;
		color: #888;
		margin: 0 0 20px;
		padding-bottom: 10px;
		border-bottom: 1px solid #ddd;
	}

	/* ---- MASONRY ---- */
	.masonry {
		columns: 3;
		column-gap: 12px;
	}

	/* ---- CARD ---- */
	.card {
		display: inline-block;
		width: 100%;
		margin-bottom: 12px;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		overflow: hidden;
		text-decoration: none;
		color: inherit;
		transition: transform 0.12s ease, box-shadow 0.12s ease;
		break-inside: avoid;
	}

	.card:hover {
		transform: translateY(-3px);
		box-shadow: 6px 6px 0 #1a1a1a;
	}

	.card.draft {
		border-style: dashed;
	}

	/* ---- CARD VISUAL ---- */
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

	.card.tall .card-visual {
		min-height: 260px;
	}

	.card-visual img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
		position: absolute;
		inset: 0;
	}

	.card-visual iframe {
		width: 100%;
		height: 100%;
		border: none;
		position: absolute;
		inset: 0;
		pointer-events: none;
	}

	.card-default-icon {
		font-size: 40px;
		opacity: 0.25;
		color: #1a1a1a;
		user-select: none;
	}

	/* ---- CARD BODY ---- */
	.card-body {
		padding: 16px 18px 18px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.card-tag {
		display: inline-block;
		font-size: 11px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		background: #1a1a1a;
		color: #fff;
		padding: 3px 8px;
		border-radius: 4px;
		align-self: flex-start;
	}

	.draft-tag {
		background: #888;
	}

	.card-title {
		font-size: 18px;
		font-weight: 700;
		letter-spacing: -0.02em;
		margin: 4px 0 0;
		line-height: 1.3;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.card-meta {
		font-size: 12px;
		color: #888;
		margin-top: 2px;
	}

	.card-top-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.card-edit {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #1a1a1a;
		text-decoration: none;
		border: 2px solid #1a1a1a;
		padding: 2px 8px;
		border-radius: 4px;
		cursor: pointer;
		transition: background 0.12s, color 0.12s;
	}

	.card-edit:hover {
		background: #1a1a1a;
		color: #fff;
	}

	.card-stats {
		display: flex;
		gap: 12px;
		font-size: 12px;
		color: #555;
		font-weight: 500;
	}

	.stat-icon {
		width: 14px;
		height: 14px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
		vertical-align: -2px;
		margin-right: 2px;
	}

	.card-meta .stat-icon {
		opacity: 0.6;
	}

	/* ---- DARK MODE CARD ---- */
	.card.dark {
		background: #0a0a0a;
		border-color: #333;
		color: #e8e8e8;
	}

	.card.dark:hover {
		box-shadow: 6px 6px 0 #444;
	}

	.card.dark .card-body {
		background: #0a0a0a;
	}

	.card.dark .card-title {
		color: #fff;
	}

	.card.dark .card-meta {
		color: #777;
	}

	.card.dark .card-stats {
		color: #999;
	}

	.card.dark .card-tag {
		background: #fff;
		color: #0a0a0a;
	}

	.card.dark .draft-tag {
		background: #666;
		color: #fff;
	}

	.card.dark .card-edit {
		color: #ccc;
		border-color: #555;
		background: transparent;
	}

	.card.dark .card-edit:hover {
		background: #fff;
		color: #0a0a0a;
		border-color: #fff;
	}

	.card.dark .card-default-icon {
		color: #fff;
	}

	.card.dark .card-visual {
		background: #111;
	}

	/* ---- CINEMATIC MODE CARD ---- */
	.card.cinematic {
		position: relative;
		border-color: var(--card-border, #b8b0a4);
		background: var(--card-bg, #faf8f4);
	}

	.card.cinematic:hover {
		box-shadow: 6px 6px 0 var(--card-shadow, #a89e90);
	}

	.card.cinematic .card-visual {
		position: relative;
	}

	.card.cinematic .card-visual::after {
		content: '';
		position: absolute;
		inset: 0;
		background: radial-gradient(ellipse at center, transparent 40%, rgba(0,0,0,0.15) 100%);
		pointer-events: none;
	}

	.card.cinematic .card-body {
		background: var(--card-bg, #faf8f4);
	}

	.card.cinematic .card-title {
		color: #2a2a2c;
	}

	.card.cinematic .card-meta {
		color: var(--card-muted, #8a8580);
	}

	.card.cinematic .card-stats {
		color: var(--card-muted, #7a756e);
	}

	.card.cinematic .card-tag {
		background: #3a3632;
	}

	.card.cinematic .card-edit {
		color: var(--card-muted, #5a5550);
		border-color: var(--card-border, #b8b0a4);
	}

	.card.cinematic .card-edit:hover {
		background: #3a3632;
		color: var(--card-bg, #faf8f4);
		border-color: #3a3632;
	}

	.card.cinematic .card-default-icon {
		color: var(--card-muted, #8a8580);
	}

	/* ---- DARK + CINEMATIC COMBO ---- */
	.card.dark.cinematic {
		background: #0c0b0a;
		border-color: #2a2824;
	}

	.card.dark.cinematic:hover {
		box-shadow: 6px 6px 0 #3a3630;
	}

	.card.dark.cinematic .card-visual::after {
		background: radial-gradient(ellipse at center, transparent 30%, rgba(0,0,0,0.3) 100%);
	}

	.card.dark.cinematic .card-body {
		background: #0c0b0a;
	}

	.card.dark.cinematic .card-title {
		color: #e8e4dc;
	}

	.card.dark.cinematic .card-meta {
		color: #6a665e;
	}

	.card.dark.cinematic .card-stats {
		color: #7a766e;
	}

	.card.dark.cinematic .card-tag {
		background: #d4cfc4;
		color: #0c0b0a;
	}

	.card.dark.cinematic .card-edit {
		color: #9a9488;
		border-color: #3a3630;
	}

	.card.dark.cinematic .card-edit:hover {
		background: #d4cfc4;
		color: #0c0b0a;
		border-color: #d4cfc4;
	}

	/* ---- USER BACKGROUND COLOR CARD ---- */
	.card.has-user-bg {
		background: var(--card-user-bg);
	}

	.card.has-user-bg .card-body {
		background: var(--card-user-bg);
	}

	.card.has-user-bg .card-visual {
		background: color-mix(in srgb, var(--card-user-bg) 80%, #000 20%);
	}

	/* ---- RESPONSIVE ---- */
	@media (max-width: 900px) {
		.masonry {
			columns: 2;
		}
	}

	@media (max-width: 560px) {
		.dashboard {
			padding: 0 16px 60px;
		}

		.masonry {
			columns: 1;
		}

		.nav-links {
			display: none;
		}

		.hero h1 {
			font-size: 28px;
		}

		.open-form {
			width: 100%;
		}

		.open-form input {
			min-width: 0;
			flex: 1;
		}
	}
</style>
