<script lang="ts">
	import { goto } from '$app/navigation';
	import { env } from '$env/dynamic/public';
	import { onMount } from 'svelte';
	import type { ApiPage } from '$lib/editor/types';
	import { user, authLoading, logout } from '$lib/stores/auth';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let pages: ApiPage[] = [];
	let archivedPages: ApiPage[] = [];
	let loading = true;
	let error = '';
	let pageIdInput = '';
	let showArchived = false;
	let confirmDeleteId: string | null = null;

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
			const [pagesRes, archivedRes] = await Promise.all([
				fetch(`${apiUrl}/v1/pages`, { credentials: 'include' }),
				fetch(`${apiUrl}/v1/pages/archived`, { credentials: 'include' })
			]);
			if (!pagesRes.ok) throw new Error('Failed to load pages');
			const payload = await pagesRes.json();
			pages = payload?.items ?? [];

			if (archivedRes.ok) {
				const archivedPayload = await archivedRes.json();
				archivedPages = archivedPayload?.items ?? [];
			}

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

	async function archivePage(pageId: string) {
		try {
			const res = await fetch(`${apiUrl}/v1/pages/${encodeURIComponent(pageId)}/archive`, { method: 'PUT', credentials: 'include' });
			if (!res.ok) throw new Error('Failed to archive page');
			const page = pages.find(p => p.id === pageId);
			if (page) archivedPages = [page, ...archivedPages];
			pages = pages.filter(p => p.id !== pageId);
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Archive failed');
		}
	}

	async function restorePage(pageId: string) {
		try {
			const res = await fetch(`${apiUrl}/v1/pages/${encodeURIComponent(pageId)}/restore`, { method: 'PUT', credentials: 'include' });
			if (!res.ok) throw new Error('Failed to restore page');
			const page = archivedPages.find(p => p.id === pageId);
			if (page) pages = [page, ...pages];
			archivedPages = archivedPages.filter(p => p.id !== pageId);
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Restore failed');
		}
	}

	async function deletePage(pageId: string) {
		try {
			const res = await fetch(`${apiUrl}/v1/pages/${encodeURIComponent(pageId)}`, { method: 'DELETE', credentials: 'include' });
			if (!res.ok) throw new Error('Failed to delete page');
			pages = pages.filter(p => p.id !== pageId);
			archivedPages = archivedPages.filter(p => p.id !== pageId);
			confirmDeleteId = null;
		} catch (err) {
			alert(err instanceof Error ? err.message : 'Delete failed');
		}
	}

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

{#if $authLoading}
	<div class="landing-loading">
		<div class="landing-spinner"></div>
	</div>
{:else if !$user}
<!-- ═══════════════ LANDING PAGE ═══════════════ -->
<div class="landing">
	<!-- NAV -->
	<header class="landing-nav">
		<a href="/" class="landing-brand">Jot.</a>
		<nav class="landing-nav-links">
			<a href="/feed">Feed</a>
			<a href="/login">Log in</a>
			<a href="/signup" class="landing-nav-cta">Try it free</a>
		</nav>
	</header>

	<!-- HERO BENTO GRID -->
	<section class="bento">
		<!-- Card 1: Hero statement -->
		<div class="bento-card bento-hero">
			<div class="bento-card-top">
				<span class="bento-label">A TINY WRITING TOOL</span>
				<span class="bento-label">PAGE (N°001)</span>
			</div>
			<span class="bento-page-num">01</span>
			<div class="bento-dots"><span class="dot filled"></span><span class="dot"></span></div>
			<h1 class="bento-headline">
				Just start<br/>
				<em>writing</em> —<br/>
				we'll handle the rest.
			</h1>
			<span class="bento-tm">™</span>
			<span class="bento-year">©2026</span>
			<p class="bento-sub">
				A simple block editor for drafting,<br/>
				publishing, and sharing your thoughts.<br/>
				No fuss. Just words.
			</p>
		</div>

		<!-- Card 2: Year overview / stats -->
		<div class="bento-card bento-stats">
			<div class="bento-card-top">
				<span class="bento-label">THE GOOD STUFF</span>
				<span class="bento-label">PAGE (N°002)</span>
			</div>
			<div class="bento-stats-year">
				<span class="big-year">2026</span>
				<a href="/signup" class="arrow-btn" aria-label="Get started">→</a>
				<span class="big-year-suffix">in Numbers</span>
			</div>
			<div class="bento-dots"><span class="dot filled"></span><span class="dot"></span></div>
			<span class="bento-growth-label">BUILT FOR FUN. STAYED<br/>BECAUSE IT ACTUALLY WORKS.</span>
			<div class="bento-metrics-row">
				<div class="bento-metric">
					<span class="metric-value">∞</span>
					<span class="metric-divider"></span>
					<span class="metric-label">BLOCKS PER PAGE</span>
					<span class="metric-desc">Text, images, embeds, galleries —<br/>mix and match however you like.</span>
				</div>
				<div class="bento-metric">
					<span class="metric-value">8.4<span class="metric-unit">s</span></span>
					<span class="metric-divider"></span>
					<span class="metric-label">AVG. TIME TO PUBLISH</span>
					<span class="metric-desc">Hit publish and you're live.<br/>Seriously, that's it.</span>
				</div>
			</div>
		</div>

		<!-- Card 3: Features / Conference style -->
		<div class="bento-card bento-features dark-card">
			<div class="bento-card-top">
				<span class="bento-label light">FEATURES</span>
				<span class="bento-label light">©2026</span>
			</div>
			<h2 class="bento-features-headline">
				A pet project<br/>
				that got<br/>
				<em>a little</em> —<br/>
				out of<br/>
				hand.
			</h2>
			<div class="bento-features-meta">
				<div class="sound-wave" aria-hidden="true">
					{#each Array(24) as _, i}
						<span class="wave-bar" style="--h:{12 + Math.sin(i * 0.7) * 10 + Math.random() * 6}px"></span>
					{/each}
				</div>
				<div class="features-info">
					<span class="globe-icon">⊕</span>
					<span class="features-location">Works anywhere<br/>Open source<br/>Free forever</span>
				</div>
			</div>
			<div class="bento-dots"><span class="dot filled light-dot"></span><span class="dot light-dot"></span></div>
		</div>

		<!-- Card 4: Solution / CTA -->
		<div class="bento-card bento-solution">
			<div class="bento-card-top">
				<span class="bento-label">WHY THO</span>
			</div>
			<h2 class="bento-solution-headline">
				Most writing tools get in the way.
				<strong>Jot</strong> stays out of it —
				just a clean editor, some <em>blocks</em>,
				and a publish button.
			</h2>
			<div class="bento-solution-footer">
				<a href="/signup" class="bento-cta-pill">Try it out</a>
				<span class="dot filled"></span>
				<span class="dot"></span>
			</div>
		</div>
	</section>

	<!-- BOTTOM STRIP -->
	<section class="landing-bottom">
		<div class="bottom-left">
			<span class="bottom-num">04</span>
		</div>
		<div class="bottom-center">
			<a href="/signup" class="bottom-arrow" aria-label="Sign up">→</a>
		</div>
		<div class="bottom-right">
			<span class="bottom-big-year">2026</span>
		</div>
	</section>

	<!-- FOOTER -->
	<footer class="landing-footer">
		<span class="landing-brand-sm">Jot.</span>
		<span class="landing-copy">© 2026 Jot. All rights reserved.</span>
		<nav class="landing-footer-links">
			<a href="/feed">Feed</a>
			<a href="/login">Log in</a>
			<a href="/signup">Sign up</a>
		</nav>
	</footer>
</div>
<!-- ═══════════════ END LANDING PAGE ═══════════════ -->

{:else}
<div class="dashboard">
	<!-- NAV -->
	<header class="nav">
		<a href="/" class="brand">Jot.</a>
		<nav class="nav-links">
			<a href="/">Home.</a>
			<a href="/feed">Feed.</a>
			<a href="/editor">Editor.</a>
		</nav>

		<form class="open-form" on:submit={openPageById}>
			<input type="text" placeholder="Paste page ID…" bind:value={pageIdInput} />
			<button type="submit">→</button>
		</form>

		<div class="nav-right">
			{#if $user}
				<a class="nav-user" href="/user/{$user.username}">{$user.display_name || $user.username}</a>
				<a class="nav-settings" href="/settings">⚙</a>
				<button class="nav-logout" on:click={() => { logout(); goto('/login'); }}>Log out</button>
			{:else if !$authLoading}
				<a class="nav-cta" href="/login">Log in</a>
			{/if}
			<a class="nav-cta" href="/editor">+ New page</a>
		</div>
	</header>

	<!-- HERO -->
	<section class="hero">
		<h1>Your Pages</h1>
	</section>

	{#if loading}
		<div class="status">
			<div class="spinner"></div>
			<span>Loading pages…</span>
		</div>
	{:else if error}
		<div class="status error-text">{error}</div>
	{:else if pages.length === 0 && archivedPages.length === 0}
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
								<div class="card-actions">
									<button type="button" class="card-action archive-action" on:click|preventDefault|stopPropagation={() => archivePage(page.id)} title="Archive">
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>
									</button>
									<button type="button" class="card-action delete-action" on:click|preventDefault|stopPropagation={() => { confirmDeleteId = page.id; }} title="Delete">
										<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
									</button>
								</div>
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
								<div class="card-actions">
									<button type="button" class="card-action archive-action" on:click|preventDefault|stopPropagation={() => archivePage(page.id)} title="Archive">
										<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>
									</button>
									<button type="button" class="card-action delete-action" on:click|preventDefault|stopPropagation={() => { confirmDeleteId = page.id; }} title="Delete">
										<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
									</button>
								</div>
							</div>
						</a>
					{/each}
				</div>
			</section>
		{/if}

		<!-- Archived Section -->
		{#if archivedPages.length > 0}
			<section class="section archived-section">
				<button type="button" class="section-toggle" on:click={() => showArchived = !showArchived}>
					<h2 class="section-title" style="margin:0;border:none;padding:0;">Archived <span class="archived-count">{archivedPages.length}</span></h2>
					<span class="toggle-chevron" class:open={showArchived}>▸</span>
				</button>
				{#if showArchived}
					<div class="masonry" style="margin-top:16px;">
						{#each archivedPages as page (page.id)}
							<div class="card archived-card">
								<div class="card-visual archived-visual" style={`background:${patternFor(page)}`}>
									<div class="card-default-icon">📦</div>
								</div>
								<div class="card-body">
									<div class="card-top-row">
										<span class="card-tag archived-tag">Archived</span>
									</div>
									<h3 class="card-title">{page.title || 'Untitled'}</h3>
									<div class="card-meta"><svg class="stat-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><rect x="3" y="4" width="18" height="18" rx="2" ry="2" /><line x1="16" y1="2" x2="16" y2="6" /><line x1="8" y1="2" x2="8" y2="6" /><line x1="3" y1="10" x2="21" y2="10" /></svg> {formatDate(page.deleted_at || page.updated_at)}</div>
									<div class="card-actions">
										<button type="button" class="card-action restore-action" on:click|stopPropagation={() => restorePage(page.id)} title="Restore">
											<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg>
											Restore
										</button>
										<button type="button" class="card-action delete-action" on:click|stopPropagation={() => { confirmDeleteId = page.id; }} title="Delete permanently">
											<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
											Delete
										</button>
									</div>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</section>
		{/if}
	{/if}

	<!-- Delete confirmation modal -->
	{#if confirmDeleteId}
		<div class="modal-overlay" on:click={() => { confirmDeleteId = null; }} on:keydown={(e) => e.key === 'Escape' && (confirmDeleteId = null)} role="dialog" aria-modal="true" tabindex="-1">
			<div class="modal-card" on:click|stopPropagation role="document">
				<h3 class="modal-title">Delete this page?</h3>
				<p class="modal-text">This action is <strong>permanent</strong> and cannot be undone. All blocks and proofreads will be deleted.</p>
				<div class="modal-actions">
					<button type="button" class="modal-btn modal-cancel" on:click={() => { confirmDeleteId = null; }}>Cancel</button>
					<button type="button" class="modal-btn modal-delete" on:click={() => confirmDeleteId && deletePage(confirmDeleteId)}>Delete permanently</button>
				</div>
			</div>
		</div>
	{/if}
</div>
{/if}

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
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		text-decoration: none;
		box-shadow: 3px 3px 0 #1a1a1a;
		transition: background 0.15s, transform 0.15s, box-shadow 0.15s;
	}

	.nav-cta:hover {
		background: #333;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.nav-right {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.nav-user {
		font-size: 14px;
		font-weight: 600;
		color: #1a1a1a;
		text-decoration: none;
	}
	.nav-user:hover {
		text-decoration: underline;
	}
	.nav-settings {
		font-size: 16px;
		text-decoration: none;
		color: #1a1a1a;
		opacity: 0.5;
		transition: opacity 0.15s;
	}
	.nav-settings:hover {
		opacity: 1;
	}
	.nav-logout {
		font-family: inherit;
		font-size: 13px;
		font-weight: 600;
		color: #1a1a1a;
		background: none;
		border: 2px solid #1a1a1a;
		padding: 5px 12px;
		border-radius: 6px;
		cursor: pointer;
		transition: background 0.15s, color 0.15s;
	}
	.nav-logout:hover {
		background: #1a1a1a;
		color: #faf9f7;
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
		align-items: center;
	}

	.open-form input {
		padding: 6px 12px;
		border: 2px solid #1a1a1a;
		border-right: none;
		border-radius: 6px 0 0 6px;
		background: #fff;
		font-size: 12px;
		font-family: inherit;
		outline: none;
		width: 140px;
		transition: width 0.2s, box-shadow 0.15s;
	}

	.open-form input:focus {
		width: 200px;
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.open-form input::placeholder {
		color: #aaa;
		font-size: 11px;
	}

	.open-form button {
		padding: 6px 10px;
		border: 2px solid #1a1a1a;
		border-radius: 0 6px 6px 0;
		background: #1a1a1a;
		color: #fff;
		font-size: 13px;
		font-weight: 700;
		cursor: pointer;
		white-space: nowrap;
		transition: background 0.15s;
		line-height: 1;
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

	/* ---- CARD ACTIONS ---- */
	.card-actions {
		display: flex;
		gap: 6px;
		margin-top: 6px;
		opacity: 0;
		transition: opacity 0.15s;
	}

	.card:hover .card-actions {
		opacity: 1;
	}

	.card-action {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 3px 8px;
		border: 2px solid #ddd;
		border-radius: 4px;
		background: transparent;
		color: #888;
		cursor: pointer;
		transition: all 0.12s;
	}

	.card-action svg {
		width: 13px;
		height: 13px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.archive-action:hover {
		border-color: #f59e0b;
		color: #b45309;
		background: #fffbeb;
	}

	.delete-action:hover {
		border-color: #ef4444;
		color: #dc2626;
		background: #fef2f2;
	}

	.restore-action:hover {
		border-color: #22c55e;
		color: #16a34a;
		background: #f0fdf4;
	}

	/* Dark card action overrides */
	.card.dark .card-action {
		border-color: #444;
		color: #777;
	}

	.card.dark .archive-action:hover {
		border-color: #f59e0b;
		color: #fbbf24;
		background: rgba(245, 158, 11, 0.1);
	}

	.card.dark .delete-action:hover {
		border-color: #ef4444;
		color: #f87171;
		background: rgba(239, 68, 68, 0.1);
	}

	/* ---- ARCHIVED SECTION ---- */
	.archived-section {
		margin-top: 40px;
		border-top: 1px dashed #ccc;
		padding-top: 20px;
	}

	.section-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		background: none;
		border: none;
		cursor: pointer;
		padding: 0;
		text-align: left;
	}

	.toggle-chevron {
		font-size: 14px;
		color: #888;
		transition: transform 0.2s ease;
		display: inline-block;
	}

	.toggle-chevron.open {
		transform: rotate(90deg);
	}

	.archived-count {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 18px;
		height: 18px;
		padding: 0 5px;
		border-radius: 9px;
		background: #e0e0dc;
		color: #888;
		font-size: 11px;
		font-weight: 800;
		margin-left: 4px;
		vertical-align: middle;
	}

	.archived-card {
		border-style: dashed;
		border-color: #ccc;
		opacity: 0.75;
		transition: opacity 0.15s, transform 0.12s, box-shadow 0.12s;
	}

	.archived-card:hover {
		opacity: 1;
	}

	.archived-visual {
		min-height: 80px !important;
	}

	.archived-tag {
		background: #888 !important;
	}

	.archived-card .card-actions {
		opacity: 1;
	}

	/* ---- DELETE MODAL ---- */
	.modal-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 9999;
		padding: 20px;
	}

	.modal-card {
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 10px;
		box-shadow: 8px 8px 0 #1a1a1a;
		padding: 28px 32px;
		max-width: 420px;
		width: 100%;
	}

	.modal-title {
		font-size: 20px;
		font-weight: 800;
		letter-spacing: -0.02em;
		margin: 0 0 10px;
	}

	.modal-text {
		font-size: 14px;
		color: #555;
		line-height: 1.5;
		margin: 0 0 24px;
	}

	.modal-actions {
		display: flex;
		gap: 10px;
		justify-content: flex-end;
	}

	.modal-btn {
		font-size: 13px;
		font-weight: 700;
		padding: 8px 18px;
		border-radius: 6px;
		cursor: pointer;
		transition: all 0.12s;
	}

	.modal-cancel {
		background: #fff;
		border: 2px solid #ddd;
		color: #555;
	}

	.modal-cancel:hover {
		border-color: #1a1a1a;
		color: #1a1a1a;
	}

	.modal-delete {
		background: #dc2626;
		border: 2px solid #dc2626;
		color: #fff;
	}

	.modal-delete:hover {
		background: #b91c1c;
		border-color: #b91c1c;
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
			display: none;
		}

		.card-actions {
			opacity: 1;
		}

		.modal-card {
			padding: 22px 20px;
		}
	}

	/* ═══════════════════════════════════════════════
	   LANDING PAGE
	   ═══════════════════════════════════════════════ */

	.landing-loading {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: #f5f5f3;
	}
	.landing-spinner {
		width: 28px; height: 28px;
		border: 3px solid #e0dfdc;
		border-top-color: #1a1a1a;
		border-radius: 50%;
		animation: spin .7s linear infinite;
	}

	.landing {
		background: #f5f5f3;
		min-height: 100vh;
		color: #1a1a1a;
	}

	/* ---- NAV ---- */
	.landing-nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 18px 32px;
		border-bottom: 2px solid #1a1a1a;
		background: #faf9f7;
	}
	.landing-brand {
		font-size: 1.6rem;
		font-weight: 900;
		color: #1a1a1a;
		text-decoration: none;
		letter-spacing: -0.04em;
	}
	.landing-nav-links {
		display: flex;
		align-items: center;
		gap: 20px;
	}
	.landing-nav-links a {
		color: #1a1a1a;
		text-decoration: none;
		font-weight: 600;
		font-size: .9rem;
		transition: opacity .15s;
	}
	.landing-nav-links a:hover { opacity: 0.5; }
	.landing-nav-cta {
		background: #1a1a1a !important;
		color: #fff !important;
		padding: 8px 22px;
		border: 2px solid #1a1a1a !important;
		font-weight: 700 !important;
		font-size: .85rem !important;
		letter-spacing: .02em;
		box-shadow: 3px 3px 0 #1a1a1a;
		transition: background .15s, transform .15s, box-shadow .15s !important;
	}
	.landing-nav-cta:hover {
		background: #333 !important;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	/* ---- BENTO GRID ---- */
	.bento {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 20px;
		padding: 24px 32px;
	}

	.bento-card {
		background: #fff;
		color: #1a1a1a;
		padding: 36px 40px;
		position: relative;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		border: 2px solid #1a1a1a;
		border-radius: 12px;
		box-shadow: 8px 8px 0 #1a1a1a;
		transition: transform .2s, box-shadow .2s;
	}
	.bento-card:hover {
		transform: translateY(-2px);
		box-shadow: 10px 10px 0 #1a1a1a;
	}
	.bento-card.dark-card {
		background: #1a1a1a;
		color: #faf9f7;
		border-color: #1a1a1a;
		box-shadow: 8px 8px 0 #555;
	}
	.bento-card.dark-card:hover {
		box-shadow: 10px 10px 0 #555;
	}

	.bento-card-top {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}
	.bento-label {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: .12em;
		text-transform: uppercase;
		color: #888;
	}
	.bento-label.light { color: #777; }

	.bento-dots {
		display: flex;
		gap: 6px;
		margin: 12px 0;
	}
	.dot {
		width: 8px; height: 8px;
		border-radius: 50%;
		border: 1.5px solid #1a1a1a;
		background: transparent;
	}
	.dot.filled { background: #1a1a1a; }
	.dot.light-dot { border-color: #666; }
	.dot.filled.light-dot { background: #faf9f7; border-color: #faf9f7; }

	/* ---- CARD 1: HERO ---- */
	.bento-hero {
		min-height: 420px;
	}
	.bento-page-num {
		position: absolute;
		top: 20px;
		right: 40px;
		font-size: clamp(64px, 8vw, 96px);
		font-weight: 900;
		letter-spacing: -0.06em;
		color: #1a1a1a;
		line-height: 1;
	}
	.bento-headline {
		font-size: clamp(36px, 4.5vw, 56px);
		font-weight: 900;
		line-height: 1.05;
		letter-spacing: -0.04em;
		margin: auto 0 0 0;
	}
	.bento-headline em {
		font-style: italic;
		color: #1a1a1a;
	}
	.bento-tm {
		position: absolute;
		top: 200px;
		right: 180px;
		font-size: 13px;
		font-weight: 600;
		color: #1a1a1a;
	}
	.bento-year {
		position: absolute;
		bottom: 180px;
		right: 40px;
		font-size: 13px;
		font-weight: 600;
		color: #1a1a1a;
	}
	.bento-sub {
		font-size: 11px;
		line-height: 1.6;
		color: #888;
		text-transform: uppercase;
		letter-spacing: .06em;
		margin-top: 18px;
	}

	/* ---- CARD 2: STATS ---- */
	.bento-stats {
		min-height: 420px;
	}
	.bento-stats-year {
		display: flex;
		align-items: center;
		gap: 14px;
		margin-bottom: 4px;
	}
	.big-year {
		font-size: clamp(40px, 5vw, 64px);
		font-weight: 900;
		letter-spacing: -0.04em;
		line-height: 1;
	}
	.big-year-suffix {
		font-size: clamp(36px, 4vw, 56px);
		font-weight: 900;
		letter-spacing: -0.04em;
		line-height: 1;
	}
	.arrow-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 40px; height: 40px;
		border-radius: 50%;
		background: #1a1a1a;
		color: #fff;
		font-size: 18px;
		font-weight: 700;
		text-decoration: none;
		border: 2px solid #1a1a1a;
		box-shadow: 3px 3px 0 #1a1a1a;
		transition: background .15s, transform .15s, box-shadow .15s;
		flex-shrink: 0;
	}
	.arrow-btn:hover {
		background: #333;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.bento-growth-label {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: .1em;
		text-transform: uppercase;
		color: #888;
		line-height: 1.5;
		margin: 12px 0 24px;
	}

	.bento-metrics-row {
		display: flex;
		gap: 32px;
		margin-top: auto;
	}
	.bento-metric {
		flex: 1;
		display: flex;
		flex-direction: column;
	}
	.metric-value {
		font-size: clamp(48px, 6vw, 72px);
		font-weight: 900;
		letter-spacing: -0.04em;
		color: #1a1a1a;
		line-height: 1;
	}
	.metric-unit {
		font-size: 0.5em;
		font-weight: 700;
	}
	.metric-divider {
		width: 40px;
		height: 2px;
		background: #1a1a1a;
		margin: 10px 0;
	}
	.metric-label {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: .1em;
		text-transform: uppercase;
		color: #1a1a1a;
		margin-bottom: 6px;
	}
	.metric-desc {
		font-size: 10px;
		line-height: 1.5;
		color: #888;
		text-transform: uppercase;
		letter-spacing: .04em;
	}

	/* ---- CARD 3: FEATURES (DARK) ---- */
	.bento-features {
		min-height: 420px;
	}
	.bento-features-headline {
		font-size: clamp(32px, 4vw, 50px);
		font-weight: 900;
		line-height: 1.08;
		letter-spacing: -0.04em;
		margin: 0;
		flex: 1;
	}
	.bento-features-headline em {
		font-style: italic;
		color: #aaa;
	}
	.bento-features-meta {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		margin-top: 24px;
		gap: 16px;
	}
	.sound-wave {
		display: flex;
		align-items: flex-end;
		gap: 2px;
		height: 32px;
	}
	.wave-bar {
		display: block;
		width: 3px;
		height: var(--h, 14px);
		background: #faf9f7;
		border-radius: 1px;
	}
	.features-info {
		display: flex;
		align-items: flex-start;
		gap: 10px;
	}
	.globe-icon {
		font-size: 24px;
		color: #888;
		line-height: 1;
	}
	.features-location {
		font-size: 11px;
		font-weight: 600;
		color: #aaa;
		line-height: 1.5;
	}

	/* ---- CARD 4: SOLUTION ---- */
	.bento-solution {
		min-height: 420px;
		justify-content: center;
	}
	.bento-solution-headline {
		font-size: clamp(28px, 3.5vw, 44px);
		font-weight: 800;
		line-height: 1.15;
		letter-spacing: -0.03em;
		margin: 0;
	}
	.bento-solution-headline em {
		font-style: italic;
		color: #1a1a1a;
	}
	.bento-solution-headline strong {
		font-weight: 900;
	}
	.bento-solution-footer {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-top: 28px;
	}
	.bento-cta-pill {
		display: inline-block;
		padding: 10px 24px;
		background: #1a1a1a;
		color: #fff;
		font-size: 12px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: .08em;
		text-decoration: none;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		box-shadow: 4px 4px 0 #1a1a1a;
		transition: background .15s, transform .15s, box-shadow .15s;
	}
	.bento-cta-pill:hover {
		background: #333;
		transform: translateY(-1px);
		box-shadow: 5px 5px 0 #1a1a1a;
	}

	/* ---- BOTTOM STRIP ---- */
	.landing-bottom {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 32px 24px;
		gap: 20px;
	}
	.bottom-left, .bottom-center, .bottom-right {
		background: #fff;
		color: #1a1a1a;
		padding: 24px 40px;
		border: 2px solid #1a1a1a;
		border-radius: 12px;
		box-shadow: 6px 6px 0 #1a1a1a;
	}
	.bottom-left { flex: 0 0 auto; }
	.bottom-center {
		flex: 0 0 auto;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.bottom-right { flex: 1; text-align: right; }
	.bottom-num {
		font-size: clamp(48px, 6vw, 72px);
		font-weight: 900;
		letter-spacing: -0.04em;
		color: #1a1a1a;
	}
	.bottom-arrow {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px; height: 44px;
		border-radius: 50%;
		background: #1a1a1a;
		color: #fff;
		font-size: 20px;
		font-weight: 700;
		text-decoration: none;
		border: 2px solid #1a1a1a;
		box-shadow: 3px 3px 0 #1a1a1a;
		transition: background .15s, transform .15s, box-shadow .15s;
	}
	.bottom-arrow:hover {
		background: #333;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}
	.bottom-big-year {
		font-size: clamp(60px, 8vw, 100px);
		font-weight: 900;
		letter-spacing: -0.05em;
		color: #1a1a1a;
		line-height: 1;
	}

	/* ---- FOOTER ---- */
	.landing-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 20px 32px;
		border-top: 2px solid #1a1a1a;
		color: #888;
		font-size: .8rem;
		background: #faf9f7;
	}
	.landing-brand-sm {
		font-size: 1.1rem;
		font-weight: 800;
		color: #1a1a1a;
		letter-spacing: -0.04em;
	}
	.landing-copy {
		color: #888;
	}
	.landing-footer-links {
		display: flex;
		gap: 16px;
	}
	.landing-footer-links a {
		color: #1a1a1a;
		text-decoration: none;
		font-weight: 600;
		transition: opacity .15s;
	}
	.landing-footer-links a:hover { opacity: 0.5; }

	/* ---- LANDING RESPONSIVE ---- */
	@media (max-width: 800px) {
		.bento {
			grid-template-columns: 1fr;
			padding: 16px;
			gap: 16px;
		}
		.bento-hero, .bento-stats, .bento-features, .bento-solution {
			min-height: 360px;
		}
		.bento-page-num {
			font-size: 56px;
			right: 24px;
		}
		.bento-card {
			padding: 28px 24px;
			box-shadow: 6px 6px 0 #1a1a1a;
		}
		.bento-card.dark-card {
			box-shadow: 6px 6px 0 #555;
		}
		.bento-card:hover {
			transform: none;
			box-shadow: 6px 6px 0 #1a1a1a;
		}
		.bento-card.dark-card:hover {
			box-shadow: 6px 6px 0 #555;
		}
		.bento-metrics-row {
			flex-direction: column;
			gap: 20px;
		}
		.bento-tm { display: none; }
		.bento-year { right: 24px; bottom: 120px; }
		.landing-nav { padding: 14px 20px; }
		.landing-bottom {
			flex-direction: column;
			padding: 0 16px 16px;
			gap: 16px;
		}
		.bottom-left, .bottom-center, .bottom-right {
			width: 100%;
			box-sizing: border-box;
			text-align: center;
			box-shadow: 4px 4px 0 #1a1a1a;
		}
		.landing-footer {
			flex-direction: column;
			gap: 12px;
			text-align: center;
		}
	}
</style>
