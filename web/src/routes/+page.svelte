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

	type ApiCollabUser = {
		user_id: string;
		username: string;
		display_name: string;
		avatar_url: string;
		access: string;
		last_seen_at: string;
	};
	let collabUsers: Record<string, ApiCollabUser[]> = {};

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

			/* fetch collaborators for published pages that have share links */
			const sharePages = pages.filter(p => p.published && p.has_share_links);
			if (sharePages.length > 0) {
				const collabResults = await Promise.allSettled(
					sharePages.map(p =>
						fetch(`${apiUrl}/v1/pages/${encodeURIComponent(p.id)}/collaborators`, { credentials: 'include' })
							.then(r => r.ok ? r.json() : null)
							.then(data => ({ id: p.id, users: data?.collaborators ?? [] }))
					)
				);
				const map: Record<string, ApiCollabUser[]> = {};
				for (const res of collabResults) {
					if (res.status === 'fulfilled') map[res.value.id] = res.value.users;
				}
				collabUsers = map;
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
	<!-- SIDE RAIL -->
	<aside class="cover-rail">
		<div class="cover-bg">
			<div class="cover-grain"></div>
			<div class="cover-gradient"></div>
		</div>

		<a href="/" class="cover-brand">Jot.</a>

		<div class="cover-content">
			<h1 class="cover-title">Your<br/>Pages.</h1>
			<p class="cover-sub">All your writing, in one place.</p>

			<nav class="cover-nav-links">
				<a href="/" class="cover-link">Home</a>
				<a href="/feed" class="cover-link">Feed</a>
				<a href="/editor" class="cover-link">Editor</a>
				{#if $user}
					<a href="/user/{$user.username}" class="cover-link">{$user.display_name || $user.username}</a>
					<a href="/settings" class="cover-link">Settings</a>
				{/if}
			</nav>

			<form class="open-form" on:submit={openPageById}>
				<input type="text" placeholder="Paste page ID…" bind:value={pageIdInput} />
				<button type="submit">→</button>
			</form>
		</div>

		<div class="cover-foot">
			<a href="/editor" class="cover-cta">+ New page</a>
			{#if $user}
				<button class="cover-logout" on:click={() => { logout(); goto('/login'); }}>Log out</button>
			{/if}
			<span class="cover-footer-copy">© 2026 Jot.</span>
		</div>
	</aside>

	<!-- MAIN CONTENT -->
	<main class="dash-main">

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
						<a class="card" href={`/public/${page.id}`} class:tall={idx % 5 === 0} class:wide={idx % 7 === 2} class:dark={page.dark_mode} class:cinematic={page.cinematic} class:has-user-bg={!!page.bg_color} class:collab={page.has_share_links} style={cinematicStyle(page)}>
							<div class="card-visual" style={!img && !emb ? `background:${patternFor(page)}` : ''}>
								{#if img}
									<img src={img} alt={page.title || 'Page image'} />
								{:else if emb}
									<iframe src={emb} title="Embedded content" loading="lazy" sandbox="allow-scripts allow-same-origin"></iframe>
								{:else}
									<div class="card-default-icon">✦</div>
								{/if}
								{#if page.has_share_links}
									<div class="collab-stripe" aria-hidden="true"></div>
								{/if}
							</div>
							<div class="card-body">
								<span class="card-tag">{page.title ? page.title.split(' ').slice(0, 3).join(' ').toUpperCase() : 'UNTITLED'}</span>
								<h3 class="card-title">{page.title || 'Untitled'}</h3>
								<div class="card-meta">
									<div class="card-author">
										{#if $user?.avatar_url}
											<img class="card-author-avatar" src={$user.avatar_url} alt={$user.display_name || $user.username} />
										{:else}
											<span class="card-author-letter">{($user?.username || '?').charAt(0).toUpperCase()}</span>
										{/if}
										<span class="card-author-name">{$user?.display_name || $user?.username}</span>
									</div>
									<span class="card-date">{formatDate(page.published_at || page.updated_at)}</span>
								</div>
								{#if page.has_share_links}
									<div class="collab-row">
										{#if collabUsers[page.id]?.length > 0}
											<div class="collab-avatars">
												{#each collabUsers[page.id].slice(0, 4) as cu (cu.user_id)}
													{#if cu.avatar_url}
														<a href={`/user/${cu.username}`} class="collab-avatar-link" on:click|stopPropagation><img class="collab-avatar" src={cu.avatar_url} alt={cu.display_name || cu.username} title={cu.display_name || cu.username} /></a>
													{:else}
														<a href={`/user/${cu.username}`} class="collab-avatar-link" on:click|stopPropagation><span class="collab-avatar collab-avatar-letter" title={cu.display_name || cu.username}>{(cu.display_name || cu.username || '?').charAt(0).toUpperCase()}</span></a>
													{/if}
												{/each}
												{#if collabUsers[page.id].length > 4}
													<span class="collab-avatar collab-avatar-more">+{collabUsers[page.id].length - 4}</span>
												{/if}
												<span class="collab-avatars-label">{collabUsers[page.id].length} collab{collabUsers[page.id].length === 1 ? '' : 's'}</span>
											</div>
										{:else}
											<span class="collab-pip" title="Live collaboration active">
												<span class="collab-dot"></span>
												Collab active
											</span>
										{/if}
									</div>
								{/if}
								{#if page.proofread_count}
									<span class="card-proofreads">{page.proofread_count} proofread{page.proofread_count === 1 ? '' : 's'}</span>
								{/if}
								<div class="card-footer-row">
									<div class="card-read-more">
										<span>VIEW PAGE</span>
										<span class="card-arrow">→</span>
									</div>
									<div class="card-actions">
										<button type="button" class="card-action card-edit-btn" on:click|preventDefault|stopPropagation={() => goto(`/editor/${page.id}`)} title="Edit">✎</button>
										<button type="button" class="card-action archive-action" on:click|preventDefault|stopPropagation={() => archivePage(page.id)} title="Archive">
											<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>
										</button>
										<button type="button" class="card-action delete-action" on:click|preventDefault|stopPropagation={() => { confirmDeleteId = page.id; }} title="Delete">
											<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
										</button>
									</div>
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
								<span class="card-tag draft-tag">{page.title ? page.title.split(' ').slice(0, 3).join(' ').toUpperCase() : 'DRAFT'}</span>
								<h3 class="card-title">{page.title || 'Untitled'}</h3>
								<div class="card-meta">
									<div class="card-author">
										{#if $user?.avatar_url}
											<img class="card-author-avatar" src={$user.avatar_url} alt={$user.display_name || $user.username} />
										{:else}
											<span class="card-author-letter">{($user?.username || '?').charAt(0).toUpperCase()}</span>
										{/if}
										<span class="card-author-name">{$user?.display_name || $user?.username}</span>
									</div>
									<span class="card-date">{formatDate(page.updated_at)}</span>
								</div>
								<div class="card-footer-row">
									<div class="card-read-more">
										<span>EDIT DRAFT</span>
										<span class="card-arrow">→</span>
									</div>
									<div class="card-actions">
										<button type="button" class="card-action archive-action" on:click|preventDefault|stopPropagation={() => archivePage(page.id)} title="Archive">
											<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>
										</button>
										<button type="button" class="card-action delete-action" on:click|preventDefault|stopPropagation={() => { confirmDeleteId = page.id; }} title="Delete">
											<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
										</button>
									</div>
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
									<span class="card-tag archived-tag">ARCHIVED</span>
									<h3 class="card-title">{page.title || 'Untitled'}</h3>
									<div class="card-footer-row" style="margin-top:8px;padding-top:8px;border-top:1px solid #e0dfdc;">
										<span class="card-date" style="font-size:10px;color:#999;">{formatDate(page.deleted_at || page.updated_at)}</span>
										<div class="card-actions" style="opacity:1;">
											<button type="button" class="card-action restore-action" on:click|stopPropagation={() => restorePage(page.id)} title="Restore">
												<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg>
											</button>
											<button type="button" class="card-action delete-action" on:click|stopPropagation={() => { confirmDeleteId = page.id; }} title="Delete permanently">
												<svg viewBox="0 0 24 24" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
											</button>
										</div>
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
	</main>
</div>
{/if}

<style>
	:global(body) {
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	.dashboard {
		display: grid;
		grid-template-columns: 72px 1fr;
		min-height: 100vh;
		transition: grid-template-columns 0.35s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.dashboard:has(.cover-rail:hover) {
		grid-template-columns: 340px 1fr;
	}

	/* ━━ COVER RAIL ━━ */
	.cover-rail {
		position: sticky;
		top: 0;
		height: 100vh;
		display: flex;
		flex-direction: column;
		background: #0e0e0e;
		color: #fff;
		overflow: hidden;
		border-right: 3px solid #1a1a1a;
		min-width: 0;
		transition: all 0.35s cubic-bezier(0.4, 0, 0.2, 1);
		z-index: 10;
	}

	.cover-rail:hover {
		overflow-y: auto;
	}

	/* ━━ COVER BACKGROUND ━━ */
	.cover-bg {
		position: absolute;
		inset: 0;
		z-index: 0;
	}

	.cover-grain {
		position: absolute;
		inset: 0;
		opacity: 0.35;
		background-image:
			radial-gradient(circle at 20% 30%, rgba(255,255,255,0.12) 0.55px, transparent 0.8px),
			radial-gradient(circle at 80% 20%, rgba(255,255,255,0.08) 0.6px, transparent 0.95px),
			radial-gradient(circle at 35% 70%, rgba(255,255,255,0.06) 0.5px, transparent 0.9px);
		background-size: 3px 3px, 4px 4px, 5px 5px;
		mix-blend-mode: overlay;
		animation: grainShift 12s ease-in-out infinite alternate;
	}

	@keyframes grainShift {
		0% { transform: translate(0, 0); }
		100% { transform: translate(1px, 1px); }
	}

	.cover-gradient {
		position: absolute;
		inset: 0;
		background:
			radial-gradient(ellipse at 30% 20%, rgba(255,255,255,0.04) 0%, transparent 60%),
			linear-gradient(180deg, rgba(14,14,14,0) 0%, rgba(14,14,14,0.6) 100%);
		pointer-events: none;
	}

	/* ━━ COVER BRAND ━━ */
	.cover-brand {
		position: relative;
		z-index: 2;
		display: block;
		padding: 20px 14px;
		font-size: 1.4rem;
		font-weight: 900;
		color: #fff;
		text-decoration: none;
		letter-spacing: -0.04em;
		opacity: 0.6;
		transition: opacity 0.25s, padding 0.35s;
		flex-shrink: 0;
	}

	.cover-rail:hover .cover-brand {
		opacity: 0.9;
		padding: 24px 28px;
	}

	.cover-brand:hover {
		opacity: 1 !important;
	}

	/* ━━ COVER CONTENT ━━ */
	.cover-content {
		position: relative;
		z-index: 2;
		padding: 0 14px;
		flex: 1;
		display: flex;
		flex-direction: column;
		justify-content: center;
		transition: padding 0.35s;
		overflow: hidden;
	}

	.cover-rail:hover .cover-content {
		padding: 0 28px;
	}

	.cover-title {
		font-size: 28px;
		font-weight: 900;
		letter-spacing: -0.04em;
		line-height: 0.95;
		margin: 0;
		color: #fff;
		transition: font-size 0.35s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.cover-rail:hover .cover-title {
		font-size: clamp(2.5rem, 5vw, 4rem);
	}

	.cover-sub {
		font-size: 13px;
		color: #666;
		margin: 12px 0 0;
		line-height: 1.5;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.3s 0.05s, max-height 0.35s;
	}

	.cover-rail:hover .cover-sub {
		opacity: 1;
		max-height: 60px;
	}

	/* ━━ COVER NAV LINKS ━━ */
	.cover-nav-links {
		display: flex;
		flex-direction: column;
		gap: 0;
		margin-top: 32px;
		border-top: 1px solid #2a2a2a;
		padding-top: 20px;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.3s 0.05s, max-height 0.4s;
	}

	.cover-rail:hover .cover-nav-links {
		opacity: 1;
		max-height: 300px;
	}

	.cover-link {
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #666;
		text-decoration: none;
		padding: 10px 0;
		transition: color 0.15s, padding-left 0.15s;
		border: none;
	}

	.cover-link:hover {
		color: #fff;
		padding-left: 4px;
	}

	/* ━━ OPEN FORM (page ID) ━━ */
	.open-form {
		display: flex;
		gap: 0;
		margin-top: 20px;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.3s 0.05s, max-height 0.35s;
	}

	.cover-rail:hover .open-form {
		opacity: 1;
		max-height: 48px;
	}

	.open-form input {
		flex: 1;
		padding: 7px 10px;
		border: 1.5px solid #333;
		border-right: none;
		background: #1a1a1a;
		color: #fff;
		font-size: 11px;
		font-family: inherit;
		outline: none;
		transition: border-color 0.15s;
	}

	.open-form input::placeholder {
		color: #555;
	}

	.open-form input:focus {
		border-color: #555;
	}

	.open-form button {
		padding: 7px 12px;
		border: 1.5px solid #333;
		background: #fff;
		color: #0e0e0e;
		font-size: 13px;
		font-weight: 700;
		cursor: pointer;
		transition: background 0.15s;
		line-height: 1;
	}

	.open-form button:hover {
		background: #e0e0e0;
	}

	/* ━━ COVER FOOT ━━ */
	.cover-foot {
		position: relative;
		z-index: 2;
		padding: 20px 28px 24px;
		border-top: 1px solid #2a2a2a;
		display: flex;
		flex-direction: column;
		gap: 10px;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.25s, max-height 0.35s;
	}

	.cover-rail:hover .cover-foot {
		opacity: 1;
		max-height: 200px;
	}

	.cover-cta {
		padding: 8px 0;
		font-family: inherit;
		font-size: 12px;
		font-weight: 700;
		color: #0e0e0e;
		background: #fff;
		text-align: center;
		text-decoration: none;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		transition: background 0.15s;
	}

	.cover-cta:hover {
		background: #e0e0e0;
	}

	.cover-logout {
		font-family: inherit;
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: #666;
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		text-align: left;
		transition: color 0.15s;
	}

	.cover-logout:hover {
		color: #fff;
	}

	.cover-footer-copy {
		font-size: 11px;
		color: #444;
		font-weight: 500;
	}

	/* ━━ MAIN CONTENT ━━ */
	.dash-main {
		padding: 40px 36px 80px;
		min-height: 100vh;
		max-width: 1200px;
		animation: contentFadeIn 0.6s cubic-bezier(0.4, 0, 0.2, 1) both;
	}

	@keyframes contentFadeIn {
		from { opacity: 0; transform: translateY(12px); }
		to { opacity: 1; transform: translateY(0); }
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
		columns: 4;
		column-gap: 16px;
	}

	/* ---- CARD ---- */
	.card {
		display: inline-flex;
		flex-direction: column;
		width: 100%;
		margin-bottom: 16px;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		overflow: hidden;
		text-decoration: none;
		color: inherit;
		transition: transform 0.14s ease, box-shadow 0.14s ease;
		break-inside: avoid;
		position: relative;
	}

	.card:hover {
		transform: translateY(-4px);
		box-shadow: 6px 6px 0 #1a1a1a;
	}

	.card.draft {
		border-style: dashed;
	}

	/* ── Collab card ── */
	.card.collab {
	}

	/* Diagonal hatch stripe overlaid on the visual */
	.collab-stripe {
		position: absolute;
		inset: 0;
		pointer-events: none;
		background: repeating-linear-gradient(
			-55deg,
			transparent,
			transparent 7px,
			rgba(0,0,0,0.04) 7px,
			rgba(0,0,0,0.04) 9px
		);
	}

	/* Collab row below author strip */
	.collab-row {
		margin-top: 2px;
	}

	.collab-pip {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: 9px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.09em;
		color: #1a1a1a;
		border: 1.5px solid #1a1a1a;
		padding: 2px 7px 2px 5px;
		border-radius: 4px;
	}

	.collab-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: #1a1a1a;
		flex-shrink: 0;
		animation: collab-pulse 2.2s ease-in-out infinite;
	}

	@keyframes collab-pulse {
		0%, 100% { opacity: 1; transform: scale(1); }
		50% { opacity: 0.35; transform: scale(0.75); }
	}

	/* Collab avatar strip */
	.collab-avatars {
		display: flex;
		align-items: center;
		gap: 0;
	}

	.collab-avatar {
		width: 22px;
		height: 22px;
		border-radius: 50%;
		border: 2px solid #fafaf8;
		object-fit: cover;
		margin-right: -6px;
		flex-shrink: 0;
	}

	.collab-avatar-letter,
	.collab-avatar-more {
		background: #1a1a1a;
		color: #fafaf8;
		font-size: 9px;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		user-select: none;
	}

	.collab-avatar-more {
		font-size: 8px;
		letter-spacing: -0.03em;
	}

	.collab-avatars-label {
		margin-left: 12px;
		font-size: 9px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.09em;
		color: #1a1a1a;
	}

	.collab-avatar-link {
		display: contents;
		text-decoration: none;
	}

	/* ---- CARD VISUAL ---- */
	.card-visual {
		width: 100%;
		min-height: 140px;
		background: #e8e8e4;
		overflow: hidden;
		display: flex;
		align-items: center;
		justify-content: center;
		position: relative;
	}

	.card.tall .card-visual {
		min-height: 220px;
	}

	.card.wide .card-visual {
		min-height: 160px;
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
		opacity: 0.2;
		color: #1a1a1a;
		user-select: none;
	}

	/* ---- CARD BODY ---- */
	.card-body {
		padding: 14px 16px 12px;
		display: flex;
		flex-direction: column;
		gap: 5px;
	}

	/* Feed-style tag: no fill, just a bottom border underline */
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

	/* Draft tag gets a muted underline */
	.draft-tag {
		color: #888;
		border-bottom-color: #888;
	}

	.card-title {
		font-size: 15px;
		font-weight: 800;
		letter-spacing: -0.02em;
		margin: 3px 0 0;
		line-height: 1.35;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	/* ---- CARD META / AUTHOR ---- */
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
		max-width: 120px;
	}

	.card-date {
		font-size: 10px;
		color: #999;
		font-weight: 500;
		white-space: nowrap;
	}

	.card-proofreads {
		font-size: 10px;
		font-weight: 700;
		color: #888;
		letter-spacing: 0.02em;
	}

	/* ---- CARD FOOTER ROW (read-more + actions) ---- */
	.card-footer-row {
		margin-top: 8px;
		padding-top: 10px;
		border-top: 1px solid #e0dfdc;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	/* ---- READ MORE BAR ---- */
	.card-read-more {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.card-read-more span {
		font-size: 10px;
		font-weight: 800;
		letter-spacing: 0.12em;
		color: #1a1a1a;
	}

	.card-arrow {
		font-size: 14px;
		transition: transform 0.15s;
	}

	.card:hover .card-arrow {
		transform: translateX(3px);
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

	.card.dark .card-tag {
		color: #ccc;
		border-bottom-color: #555;
	}

	.card.dark .draft-tag {
		color: #666;
		border-bottom-color: #444;
	}

	.card.dark .card-date {
		color: #666;
	}

	.card.dark .card-author-name {
		color: #ccc;
	}

	.card.dark .card-author-letter {
		background: #444;
		color: #fff;
	}

	.card.dark .card-author-avatar {
		border-color: #555;
	}

	.card.dark .card-footer-row {
		border-top-color: #2a2a2a;
	}

	.card.dark .card-read-more span {
		color: #ccc;
	}

	.card.dark .card-default-icon {
		color: #fff;
	}

	.card.dark .card-visual {
		background: #111;
	}

	.card.dark .card-proofreads {
		color: #666;
	}

	.card.dark .collab-pip {
		color: #ccc;
		border-color: #555;
	}

	.card.dark .collab-dot {
		background: #ccc;
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

	.card.cinematic .card-date {
		color: var(--card-muted, #8a8580);
	}

	.card.cinematic .card-tag {
		color: #3a3632;
		border-bottom-color: #3a3632;
	}

	.card.cinematic .card-author-name {
		color: #3a3632;
	}

	.card.cinematic .card-footer-row {
		border-top-color: var(--card-border, #d4cfc4);
	}

	.card.cinematic .card-read-more span {
		color: #3a3632;
	}

	.card.cinematic .card-default-icon {
		color: var(--card-muted, #8a8580);
	}

	.card.cinematic .card-proofreads {
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

	.card.dark.cinematic .card-date {
		color: #6a665e;
	}

	.card.dark.cinematic .card-tag {
		color: #d4cfc4;
		border-bottom-color: #3a3630;
	}

	.card.dark.cinematic .card-author-name {
		color: #d4cfc4;
	}

	.card.dark.cinematic .card-footer-row {
		border-top-color: #2a2824;
	}

	.card.dark.cinematic .card-read-more span {
		color: #d4cfc4;
	}

	.card.dark.cinematic .card-proofreads {
		color: #6a665e;
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

	/* ---- CARD ACTIONS (icon buttons in footer) ---- */
	.card-actions {
		display: flex;
		gap: 4px;
		opacity: 0;
		transition: opacity 0.15s;
	}

	.card:hover .card-actions {
		opacity: 1;
	}

	.card-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border: 1.5px solid #ddd;
		border-radius: 4px;
		background: transparent;
		color: #999;
		cursor: pointer;
		transition: all 0.12s;
		padding: 0;
		font-size: 12px;
	}

	.card-action svg {
		width: 12px;
		height: 12px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.card-edit-btn:hover {
		border-color: #1a1a1a;
		color: #1a1a1a;
		background: #f5f5f3;
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
		border-color: #333;
		color: #666;
	}

	.card.dark .card-edit-btn:hover {
		border-color: #fff;
		color: #fff;
		background: rgba(255,255,255,0.1);
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
		color: #999 !important;
		border-bottom-color: #ccc !important;
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
	@media (max-width: 1200px) {
		.masonry { columns: 3; }
	}

	@media (max-width: 1100px) {
		.dashboard:has(.cover-rail:hover) {
			grid-template-columns: 300px 1fr;
		}
		.dash-main {
			padding: 32px 24px 60px;
		}
	}

	@media (max-width: 860px) {
		.dashboard {
			grid-template-columns: 1fr;
		}
		.dashboard:has(.cover-rail:hover) {
			grid-template-columns: 1fr;
		}
		.cover-rail {
			position: relative;
			height: auto;
			border-right: none;
			border-bottom: 3px solid #1a1a1a;
			overflow: visible;
			padding-bottom: 28px;
		}
		.cover-bg { position: absolute; }
		.cover-brand { opacity: 0.9 !important; padding: 20px 24px !important; }
		.cover-content { padding: 0 24px !important; }
		.cover-title { font-size: clamp(2.5rem, 6vw, 3.5rem) !important; }
		.cover-sub { opacity: 1 !important; max-height: none !important; }
		.cover-nav-links { opacity: 1 !important; max-height: none !important; }
		.open-form { opacity: 1 !important; max-height: none !important; }
		.cover-foot { opacity: 1 !important; max-height: none !important; padding: 20px 24px; }
		.dash-main { padding: 28px 20px 60px; }
		.masonry { columns: 3; }
	}

	@media (max-width: 560px) {
		.cover-brand { padding: 16px 16px !important; }
		.cover-content { padding: 0 16px !important; }
		.cover-title { font-size: 2rem !important; }
		.cover-foot { padding: 16px 16px; }
		.dash-main { padding: 20px 12px 48px; }
		.masonry { columns: 2; }
		.card-actions { opacity: 1; }
		.modal-card { padding: 22px 20px; }
	}

	@media (max-width: 380px) {
		.masonry { columns: 1; }
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
