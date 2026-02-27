<script lang="ts">
	import { env } from '$env/dynamic/public';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import type { ApiFeedPage } from '$lib/editor/types';
	import { user, authLoading, logout } from '$lib/stores/auth';
	import MiniMusicPlayer from '$lib/components/MiniMusicPlayer.svelte';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let pages: ApiFeedPage[] = [];
	let loading = true;
	let loadingMore = false;
	let error = '';
	let offset = 0;
	let hasMore = true;
	const LIMIT = 20;

	type ApiCollabUser = { user_id: string; username: string; display_name: string; avatar_url: string; access: string; };
	let collabUsers: Record<string, ApiCollabUser[]> = {};

	type SortMode = 'new' | 'top' | 'hot';
	let sort: SortMode = 'new';

	type FilterMode = 'all' | 'following';
	let filter: FilterMode = 'all';

	const sortOptions: { value: SortMode; label: string }[] = [
		{ value: 'hot', label: 'Hot' },
		{ value: 'new', label: 'New' },
		{ value: 'top', label: 'Top' },
	];

	const filterOptions: { value: FilterMode; label: string }[] = [
		{ value: 'all', label: 'All' },
		{ value: 'following', label: 'Following' },
	];

	function changeSort(newSort: SortMode) {
		if (newSort === sort) return;
		sort = newSort;
		offset = 0;
		pages = [];
		cardTints = {};
		loadFeed();
	}

	function changeFilter(newFilter: FilterMode) {
		if (newFilter === filter) return;
		filter = newFilter;
		offset = 0;
		pages = [];
		cardTints = {};
		loadFeed();
	}

	/** Per-card cinematic tint extracted from cover image */
	let cardTints: Record<string, { bg: string; border: string; shadow: string; muted: string }> = {};

	const defaultPatterns = [
		'repeating-linear-gradient(45deg, #e8e8e4 0px, #e8e8e4 10px, #f5f5f3 10px, #f5f5f3 20px)',
		'repeating-linear-gradient(-45deg, #e8e8e4 0px, #e8e8e4 10px, #f5f5f3 10px, #f5f5f3 20px)',
		'repeating-linear-gradient(90deg, #e8e8e4 0px, #e8e8e4 8px, #f5f5f3 8px, #f5f5f3 16px)',
		'radial-gradient(circle at 20% 30%, #e0e0dc 2px, transparent 2px), radial-gradient(circle at 70% 60%, #e0e0dc 2px, transparent 2px), radial-gradient(circle at 40% 80%, #e0e0dc 2px, transparent 2px)',
		'repeating-conic-gradient(#e8e8e4 0% 25%, #f5f5f3 0% 50%) 0 0 / 20px 20px',
		'linear-gradient(135deg, #e8e8e4 25%, transparent 25%) -10px 0, linear-gradient(225deg, #e8e8e4 25%, transparent 25%) -10px 0, linear-gradient(315deg, #e8e8e4 25%, transparent 25%), linear-gradient(45deg, #e8e8e4 25%, transparent 25%)',
	];

	function patternFor(id: string): string {
		let hash = 0;
		for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) | 0;
		return defaultPatterns[Math.abs(hash) % defaultPatterns.length];
	}

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

	function cinematicStyle(p: ApiFeedPage): string {
		const parts: string[] = [];
		const t = cardTints[p.id];
		if (t) {
			parts.push(`--card-bg:${t.bg}`, `--card-border:${t.border}`, `--card-shadow:${t.shadow}`, `--card-muted:${t.muted}`);
		}
		if (p.bg_color) {
			parts.push(`--card-user-bg:${p.bg_color}`);
		}
		return parts.join(';');
	}

	function imageFor(p: ApiFeedPage): string | null {
		if (p.cover) return p.cover;
		if (p.blocks) {
			for (const b of p.blocks) {
				if (b.type === 'image' && b.data?.url) return b.data.url;
				if (b.type === 'gallery' && Array.isArray(b.data?.items)) {
					const img = b.data.items.find((i: any) => i.kind === 'image' && i.value);
					if (img) return img.value;
				}
			}
		}
		return null;
	}

	function embedFor(p: ApiFeedPage): string | null {
		if (p.blocks) {
			for (const b of p.blocks) {
				if (b.type === 'embed' && b.data?.url) return b.data.url;
				if (b.type === 'gallery' && Array.isArray(b.data?.items)) {
					const emb = b.data.items.find((i: any) => i.kind === 'embed' && i.value);
					if (emb) return emb.value;
				}
			}
		}
		return null;
	}

	function musicFor(p: ApiFeedPage): { url: string; title?: string; artist?: string; coverUrl?: string } | null {
		if (p.blocks) {
			for (const b of p.blocks) {
				if (b.type === 'music' && b.data?.url) return b.data;
			}
		}
		return null;
	}

	function formatDate(iso?: string) {
		if (!iso) return '';
		return new Date(iso).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric',
		});
	}

	function extractTintsFor(items: ApiFeedPage[]) {
		for (const p of items) {
			if (!p.cinematic) continue;
			const img = imageFor(p);
			if (img) extractQuickTint(img, p.id);
		}
	}

	async function loadFeed(append = false) {
		if (append) {
			loadingMore = true;
		} else {
			loading = true;
		}
		try {
			let url = `${apiUrl}/v1/public/feed?limit=${LIMIT}&offset=${offset}&sort=${sort}`;
			if (filter === 'following') {
				url += '&following=true';
			}
			const res = await fetch(url, { credentials: 'include' }); // Include credentials for auth
			if (!res.ok) throw new Error('Failed to load feed');
			const payload = await res.json();
			const items: ApiFeedPage[] = payload?.items ?? [];
			if (append) {
				pages = [...pages, ...items];
			} else {
				pages = items;
			}
			hasMore = items.length === LIMIT;
			extractTintsFor(items);

			/* fetch collaborators for cards that have share links */
			const shareItems = items.filter(p => p.has_share_links);
			if (shareItems.length > 0) {
				const results = await Promise.allSettled(
					shareItems.map(p =>
						fetch(`${apiUrl}/v1/public/pages/${encodeURIComponent(p.id)}/collaborators`)
							.then(r => r.ok ? r.json() : null)
							.then(data => ({ id: p.id, users: data?.collaborators ?? [] }))
					)
				);
				const map: Record<string, ApiCollabUser[]> = append ? { ...collabUsers } : {};
				for (const res of results) {
					if (res.status === 'fulfilled') map[res.value.id] = res.value.users;
				}
				collabUsers = map;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load feed';
		} finally {
			loading = false;
			loadingMore = false;
		}
	}

	function loadMore() {
		offset += LIMIT;
		loadFeed(true);
	}

	function handleLogout() {
		logout();
		goto('/login');
	}

	onMount(() => {
		loadFeed();
	});
</script>

<div class="feed-page">
	<!-- COVER RAIL / SIDEBAR -->
	<aside class="cover-rail">
		<div class="cover-bg">
			<div class="cover-grain"></div>
			<div class="cover-gradient"></div>
		</div>

		<a href="/" class="cover-brand">Jot.</a>

		<div class="cover-content">
			<h1 class="cover-title">Discover</h1>
			<p class="cover-sub">Fresh writing from the Jot community.</p>

			<nav class="cover-sort">
				{#each sortOptions as opt (opt.value)}
					<button
						class="sort-tab"
						class:active={sort === opt.value}
						on:click={() => changeSort(opt.value)}
					>
						<svg class="sort-icon" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
							{#if opt.value === 'hot'}
								<path d="M8 1C8 1 3 6 3 9.5C3 12 5.2 14 8 14C10.8 14 13 12 13 9.5C13 6 8 1 8 1ZM8 12.5C6.1 12.5 4.5 11 4.5 9.5C4.5 7.2 7 4 8 2.8C9 4 11.5 7.2 11.5 9.5C11.5 11 9.9 12.5 8 12.5Z" fill="currentColor"/>
							{:else if opt.value === 'new'}
								<path d="M8 2L9.1 5.5H12.8L9.9 7.7L10.9 11.2L8 9L5.1 11.2L6.1 7.7L3.2 5.5H6.9L8 2Z" fill="currentColor"/>
							{:else}
								<path d="M8 3L12 10H4L8 3Z" fill="currentColor"/><rect x="4" y="11" width="8" height="2" rx="0.5" fill="currentColor"/>
							{/if}
						</svg>
						{opt.label}
					</button>
				{/each}
			</nav>

			{#if $user}
				<nav class="cover-filter">
					{#each filterOptions as opt (opt.value)}
						<button
							class="filter-tab"
							class:active={filter === opt.value}
							on:click={() => changeFilter(opt.value)}
						>
							{opt.label}
						</button>
					{/each}
				</nav>
			{/if}
		</div>

		<div class="cover-nav">
			{#if $authLoading}
				<!-- loading -->
			{:else if $user}
				<a href="/user/{$user.username}" class="cover-nav-link">{$user.display_name || $user.username}</a>
				<a href="/editor" class="cover-nav-cta">+ New page</a>
			{:else}
				<a href="/login" class="cover-nav-link">Log in</a>
				<a href="/editor" class="cover-nav-cta">Post anonymously</a>
				<a href="/signup" class="cover-nav-link">Create account</a>
			{/if}
		</div>

		<div class="cover-footer">
			<span class="cover-footer-copy">© 2026 Jot.</span>
		</div>
	</aside>

	<!-- MAIN CONTENT -->
	<main class="feed-main">
		{#if loading}
			<div class="loading-wrap">
				<div class="spinner"></div>
			</div>
		{:else if error}
			<div class="loading-wrap"><p class="error-text">{error}</p></div>
		{:else if pages.length === 0}
			<div class="empty">
				<div class="empty-icon">✦</div>
				<p>No published pages yet. Be the first to share something.</p>
				{#if $user}
					<a href="/editor" class="empty-cta">Create a page</a>
				{:else}
					<a href="/signup" class="empty-cta">Join Jot</a>
				{/if}
			</div>
		{:else}
			<!-- MOBILE SORT (visible only on small screens) -->
			<div class="mobile-sort">
				{#each sortOptions as opt (opt.value)}
					<button
						class="sort-tab"
						class:active={sort === opt.value}
						on:click={() => changeSort(opt.value)}
					>
						<svg class="sort-icon" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
							{#if opt.value === 'hot'}
								<path d="M8 1C8 1 3 6 3 9.5C3 12 5.2 14 8 14C10.8 14 13 12 13 9.5C13 6 8 1 8 1ZM8 12.5C6.1 12.5 4.5 11 4.5 9.5C4.5 7.2 7 4 8 2.8C9 4 11.5 7.2 11.5 9.5C11.5 11 9.9 12.5 8 12.5Z" fill="currentColor"/>
							{:else if opt.value === 'new'}
								<path d="M8 2L9.1 5.5H12.8L9.9 7.7L10.9 11.2L8 9L5.1 11.2L6.1 7.7L3.2 5.5H6.9L8 2Z" fill="currentColor"/>
							{:else}
								<path d="M8 3L12 10H4L8 3Z" fill="currentColor"/><rect x="4" y="11" width="8" height="2" rx="0.5" fill="currentColor"/>
							{/if}
						</svg>
						{opt.label}
					</button>
				{/each}
				{#if $user}
					{#each filterOptions as opt (opt.value)}
						<button
							class="filter-tab"
							class:active={filter === opt.value}
							on:click={() => changeFilter(opt.value)}
						>
							{opt.label}
						</button>
					{/each}
				{/if}
			</div>

			<div class="masonry">
				{#each pages as p, idx (p.id)}
					{@const img = imageFor(p)}
					{@const emb = embedFor(p)}
					{@const mus = musicFor(p)}
					<a class="card" href={`/public/${p.id}`} class:tall={idx % 5 === 0} class:wide={idx % 7 === 2} class:dark={p.dark_mode} class:cinematic={p.cinematic} class:has-user-bg={!!p.bg_color} style={cinematicStyle(p)}>
						<div class="card-visual" style={!img && !emb && !mus ? `background:${patternFor(p.id)}` : ''}>
							{#if img}
								<img src={img} alt={p.title || 'Page image'} />
							{:else if emb}
								<iframe src={emb} title="Embedded content" loading="lazy" sandbox="allow-scripts allow-same-origin"></iframe>
							{:else if mus}
								<MiniMusicPlayer url={mus.url} title={mus.title || ''} artist={mus.artist || ''} coverUrl={mus.coverUrl || ''} />
							{:else}
								<div class="card-default-icon">✦</div>
							{/if}
						</div>
						<div class="card-body">
							<span class="card-tag">{p.title ? p.title.split(' ').slice(0, 3).join(' ').toUpperCase() : 'UNTITLED'}</span>
							<h3 class="card-title">{p.title || 'Untitled'}</h3>
							<div class="card-meta">
								<div class="card-author">
									{#if p.author_avatar_url}
										<img class="card-author-avatar" src={p.author_avatar_url} alt={p.author_display_name || p.author_username} />
									{:else}
										<span class="card-author-letter">{(p.author_username || '?').charAt(0).toUpperCase()}</span>
									{/if}
									{#if p.author_username && p.author_username !== 'anonymous'}
										<span class="card-author-name" role="link" tabindex="0" on:click|stopPropagation|preventDefault={() => goto(`/user/${p.author_username}`)} on:keydown|stopPropagation={(e) => { if (e.key === 'Enter') goto(`/user/${p.author_username}`); }}>{p.author_display_name || p.author_username}</span>
									{:else}
										<span class="card-author-name">{p.author_display_name || 'Anonymous'}</span>
									{/if}
								</div>
								<span class="card-date">{formatDate(p.published_at || p.updated_at)}</span>
							</div>
							{#if p.proofread_count}
								<span class="card-proofreads">{p.proofread_count} proofread{p.proofread_count === 1 ? '' : 's'}</span>
							{/if}
							{#if p.has_share_links}
								<div class="collab-row">
									{#if collabUsers[p.id]?.length > 0}
										<div class="collab-avatars">
											{#each collabUsers[p.id].slice(0, 4) as cu (cu.user_id)}
												{#if cu.avatar_url}
													<a href={`/user/${cu.username}`} class="collab-avatar-link" on:click|stopPropagation><img class="collab-avatar" src={cu.avatar_url} alt={cu.display_name || cu.username} title={cu.display_name || cu.username} /></a>
												{:else}
													<a href={`/user/${cu.username}`} class="collab-avatar-link" on:click|stopPropagation><span class="collab-avatar collab-avatar-letter" title={cu.display_name || cu.username}>{(cu.display_name || cu.username || '?').charAt(0).toUpperCase()}</span></a>
												{/if}
											{/each}
											{#if collabUsers[p.id].length > 4}
												<span class="collab-avatar collab-avatar-more">+{collabUsers[p.id].length - 4}</span>
											{/if}
											<span class="collab-avatars-label">{collabUsers[p.id].length} collab{collabUsers[p.id].length === 1 ? '' : 's'}</span>
										</div>
									{:else}
										<span class="collab-pip">
											<span class="collab-dot"></span>
											Collab active
										</span>
									{/if}
								</div>
							{/if}
							<div class="card-read-more">
								<span>READ MORE</span>
								<span class="card-arrow">→</span>
							</div>
						</div>
					</a>
				{/each}
			</div>

			{#if hasMore}
				<div class="load-more-wrap">
					<button class="load-more" on:click={loadMore} disabled={loadingMore}>
						{#if loadingMore}
							<div class="spinner-sm"></div>
						{:else}
							Load more
						{/if}
					</button>
				</div>
			{/if}
		{/if}
	</main>
</div>

<style>
	:global(body) {
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	/* ━━ TWO-COLUMN LAYOUT ━━ */
	.feed-page {
		display: grid;
		grid-template-columns: 72px 1fr;
		min-height: 100vh;
		transition: grid-template-columns 0.35s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.feed-page:has(.cover-rail:hover) {
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
			radial-gradient(circle at 20% 30%, rgba(255, 255, 255, 0.12) 0.55px, transparent 0.8px),
			radial-gradient(circle at 80% 20%, rgba(255, 255, 255, 0.08) 0.6px, transparent 0.95px),
			radial-gradient(circle at 35% 70%, rgba(255, 255, 255, 0.06) 0.5px, transparent 0.9px);
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
			radial-gradient(ellipse at 30% 20%, rgba(255, 255, 255, 0.04) 0%, transparent 60%),
			linear-gradient(180deg, rgba(14, 14, 14, 0) 0%, rgba(14, 14, 14, 0.6) 100%);
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
		animation: titleReveal 0.6s cubic-bezier(0.4, 0, 0.2, 1) both;
	}

	.cover-rail:hover .cover-title {
		font-size: clamp(2.5rem, 5vw, 4rem);
	}

	@keyframes titleReveal {
		from {
			opacity: 0;
			transform: translateY(16px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.cover-sub {
		font-size: 13px;
		font-weight: 400;
		color: #666;
		margin: 12px 0 0;
		line-height: 1.5;
		letter-spacing: -0.01em;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.3s 0.05s, max-height 0.35s;
	}

	.cover-rail:hover .cover-sub {
		opacity: 1;
		max-height: 60px;
	}

	/* ━━ SORT TABS (SIDEBAR) ━━ */
	.cover-sort {
		display: flex;
		flex-direction: column;
		gap: 0;
		margin-top: 32px;
		border-top: 1px solid #2a2a2a;
		padding-top: 20px;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.3s 0.05s, max-height 0.35s;
	}

	.cover-rail:hover .cover-sort {
		opacity: 1;
		max-height: 200px;
	}

	/* ━━ FILTER TABS (SIDEBAR) ━━ */
	.cover-filter {
		display: flex;
		flex-direction: column;
		gap: 0;
		margin-top: 16px;
		border-top: 1px solid #2a2a2a;
		padding-top: 16px;
		opacity: 1;
		max-height: 200px;
		overflow: hidden;
		transition: opacity 0.3s 0.05s, max-height 0.35s;
	}

	.filter-tab {
		font-family: inherit;
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #666;
		background: none;
		border: none;
		padding: 8px 0;
		cursor: pointer;
		transition: color 0.15s, padding-left 0.15s;
		display: flex;
		align-items: center;
		gap: 8px;
		text-align: left;
	}

	.filter-tab:hover {
		color: #fff;
		padding-left: 4px;
	}

	.filter-tab.active {
		color: #fff;
	}

	.filter-tab.active::before {
		content: '';
		display: inline-block;
		width: 12px;
		height: 2px;
		background: #fff;
		flex-shrink: 0;
	}

	.sort-tab {
		font-family: inherit;
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #666;
		background: none;
		border: none;
		padding: 10px 0;
		cursor: pointer;
		transition: color 0.15s, padding-left 0.15s;
		display: flex;
		align-items: center;
		gap: 8px;
		text-align: left;
	}

	.sort-tab:hover {
		color: #fff;
		padding-left: 4px;
	}

	.sort-tab.active {
		color: #fff;
	}

	.sort-tab.active::before {
		content: '';
		display: inline-block;
		width: 12px;
		height: 2px;
		background: #fff;
		flex-shrink: 0;
	}

	.sort-icon {
		width: 13px;
		height: 13px;
		flex-shrink: 0;
	}

	/* ━━ MOBILE SORT (INLINE) ━━ */
	.mobile-sort {
		display: none;
		gap: 0;
		margin-bottom: 24px;
		border-bottom: 2px solid #e8e8e4;
		padding-bottom: 0;
	}

	.mobile-sort .sort-tab,
	.mobile-sort .filter-tab {
		font-family: inherit;
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #666;
		background: none;
		border: none;
		padding: 12px 16px;
		cursor: pointer;
		transition: color 0.15s, background 0.15s;
		border-radius: 6px;
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.mobile-sort .sort-tab:hover,
	.mobile-sort .filter-tab:hover {
		color: #1a1a1a;
		background: #f5f5f3;
	}

	.mobile-sort .sort-tab.active,
	.mobile-sort .filter-tab.active {
		color: #1a1a1a;
		background: #1a1a1a;
		color: #fff;
	}

	.mobile-sort .sort-icon {
		width: 12px;
		height: 12px;
		flex-shrink: 0;
	}

	/* ━━ COVER NAV ━━ */
	.cover-nav {
		position: relative;
		z-index: 2;
		margin-top: auto;
		padding: 20px 28px;
		border-top: 1px solid #2a2a2a;
		display: flex;
		flex-direction: column;
		gap: 10px;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.25s, max-height 0.35s;
	}

	.cover-rail:hover .cover-nav {
		opacity: 1;
		max-height: 200px;
	}

	.cover-nav-link {
		font-size: 13px;
		font-weight: 600;
		color: #999;
		text-decoration: none;
		transition: color 0.15s;
	}

	.cover-nav-link:hover {
		color: #fff;
	}

	.cover-nav-cta {
		margin-top: 4px;
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

	.cover-nav-cta:hover {
		background: #e0e0e0;
	}

	/* ━━ COVER FOOTER ━━ */
	.cover-footer {
		position: relative;
		z-index: 2;
		padding: 16px 28px 24px;
		opacity: 0;
		transition: opacity 0.25s;
	}

	.cover-rail:hover .cover-footer {
		opacity: 1;
	}

	.cover-footer-copy {
		font-size: 11px;
		color: #444;
		font-weight: 500;
	}

	/* ━━ MAIN CONTENT ━━ */
	.feed-main {
		padding: 40px 36px 80px;
		min-height: 100vh;
		max-width: 1200px;
		animation: contentFadeIn 0.6s cubic-bezier(0.4, 0, 0.2, 1) both;
	}

	@keyframes contentFadeIn {
		from {
			opacity: 0;
			transform: translateY(12px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	/* ━━ LOADING / EMPTY ━━ */
	.loading-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 120px 0;
	}

	.spinner {
		width: 20px;
		height: 20px;
		border: 2px solid #ddd;
		border-top-color: #1a1a1a;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	.spinner-sm {
		width: 14px;
		height: 14px;
		border: 2px solid #888;
		border-top-color: #1a1a1a;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.error-text {
		color: #c00;
		font-size: 16px;
	}

	.empty {
		text-align: center;
		padding: 100px 20px;
	}

	.empty-icon {
		font-size: 56px;
		margin-bottom: 16px;
		opacity: 0.15;
	}

	.empty p {
		font-size: 16px;
		color: #888;
		margin: 0 0 28px;
	}

	.empty-cta {
		display: inline-block;
		padding: 10px 28px;
		font-family: inherit;
		font-size: 13px;
		font-weight: 700;
		color: #fff;
		background: #1a1a1a;
		border: 2px solid #1a1a1a;
		text-decoration: none;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		transition: background 0.15s, transform 0.12s;
	}

	.empty-cta:hover {
		background: #333;
		transform: translateY(-1px);
	}

	/* ━━ MASONRY ━━ */
	.masonry {
		columns: 4;
		column-gap: 16px;
	}

	/* ━━ CARD ━━ */
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

	/* ━━ CARD VISUAL ━━ */
	.card-edit-btn {
		position: absolute;
		top: 10px;
		right: 10px;
		z-index: 10;
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: #fff;
		border: 1.5px solid #1a1a1a;
		border-radius: 6px;
		cursor: pointer;
		color: #1a1a1a;
		opacity: 0;
		transition: opacity 0.15s, background 0.12s;
		padding: 0;
	}

	.card-edit-btn svg {
		width: 13px;
		height: 13px;
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.card:hover .card-edit-btn {
		opacity: 1;
	}

	.card-edit-btn:hover {
		background: #f5f5f3;
	}

	.card.dark .card-edit-btn {
		background: rgba(30,30,30,0.85);
		border-color: #555;
		color: #ccc;
	}

	.card.dark .card-edit-btn:hover {
		background: rgba(255,255,255,0.1);
		border-color: #fff;
		color: #fff;
	}

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

	.card-music-preview {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		background: #1a1a1a;
		color: #fff;
		padding: 20px 16px;
		box-sizing: border-box;
	}

	.card-music-icon {
		font-size: 32px;
		line-height: 1;
		opacity: 0.8;
	}

	.card-music-info {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 3px;
		text-align: center;
		max-width: 100%;
	}

	.card-music-title {
		font-size: 13px;
		font-weight: 700;
		color: #fff;
		line-height: 1.3;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 160px;
	}

	.card-music-artist {
		font-size: 11px;
		color: rgba(255,255,255,0.5);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 160px;
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
		text-decoration: none;
		cursor: pointer;
		transition: opacity 0.15s;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 120px;
	}

	.card-author-name:hover {
		opacity: 0.5;
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

	/* ━━ COLLAB STRIP ━━ */
	.collab-row {
		margin-top: 4px;
	}

	.collab-avatars {
		display: inline-flex;
		align-items: center;
	}

	.collab-avatar {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		border: 2px solid #fafaf8;
		object-fit: cover;
		margin-right: -5px;
		flex-shrink: 0;
	}

	.collab-avatar-letter,
	.collab-avatar-more {
		background: #1a1a1a;
		color: #fafaf8;
		font-size: 8px;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		user-select: none;
	}

	.collab-avatars-label {
		margin-left: 11px;
		font-size: 9px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.09em;
		color: #888;
	}

	.collab-avatar-link {
		display: contents;
		text-decoration: none;
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

	/* ━━ READ MORE BAR ━━ */
	.card-read-more {
		margin-top: 8px;
		padding-top: 10px;
		border-top: 1px solid #e0dfdc;
		display: flex;
		align-items: center;
		justify-content: space-between;
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

	/* ━━ DARK MODE CARD ━━ */
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

	.card.dark .card-read-more {
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

	/* ━━ CINEMATIC MODE CARD ━━ */
	.card.cinematic {
		border-color: var(--card-border, #b8b0a4);
		background: var(--card-bg, #faf8f4);
	}

	.card.cinematic:hover {
		box-shadow: 6px 6px 0 var(--card-shadow, #a89e90);
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

	.card.cinematic .card-read-more {
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

	/* ━━ DARK + CINEMATIC ━━ */
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

	.card.dark.cinematic .card-read-more {
		border-top-color: #2a2824;
	}

	.card.dark.cinematic .card-read-more span {
		color: #d4cfc4;
	}

	.card.dark.cinematic .card-proofreads {
		color: #6a665e;
	}

	/* ━━ USER BACKGROUND COLOR CARD ━━ */
	.card.has-user-bg {
		background: var(--card-user-bg);
	}

	.card.has-user-bg .card-body {
		background: var(--card-user-bg);
	}

	.card.has-user-bg .card-visual {
		background: color-mix(in srgb, var(--card-user-bg) 80%, #000 20%);
	}

	/* ━━ LOAD MORE ━━ */
	.load-more-wrap {
		display: flex;
		justify-content: center;
		padding: 40px 0 0;
	}

	.load-more {
		padding: 12px 40px;
		font-family: inherit;
		font-size: 13px;
		font-weight: 700;
		color: #1a1a1a;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 0;
		cursor: pointer;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		transition: background 0.15s, transform 0.12s, box-shadow 0.12s;
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 160px;
		min-height: 44px;
	}

	.load-more:hover:not(:disabled) {
		background: #1a1a1a;
		color: #fff;
		transform: translateY(-2px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.load-more:disabled {
		opacity: 0.5;
		cursor: default;
	}

	/* ━━ RESPONSIVE ━━ */
	@media (max-width: 1200px) {
		.masonry {
			columns: 3;
		}
	}

	@media (max-width: 1100px) {
		.feed-page:has(.cover-rail:hover) {
			grid-template-columns: 300px 1fr;
		}

		.feed-main {
			padding: 32px 24px 60px;
		}
	}

	@media (max-width: 860px) {
		.feed-page {
			grid-template-columns: 1fr;
		}

		.feed-page:has(.cover-rail:hover) {
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

		.cover-bg {
			position: absolute;
		}

		.cover-brand {
			opacity: 0.9 !important;
			padding: 20px 24px !important;
		}

		.cover-content {
			padding: 0 24px !important;
		}

		.cover-title {
			font-size: clamp(2.5rem, 6vw, 3.5rem) !important;
		}

		.cover-sub {
			opacity: 1 !important;
			max-height: none !important;
		}

		.cover-sort {
			opacity: 1 !important;
			max-height: none !important;
		}

		.cover-filter {
			opacity: 1 !important;
			max-height: none !important;
		}

		.cover-nav {
			opacity: 1 !important;
			max-height: none !important;
			padding: 20px 24px;
		}

		.cover-footer {
			opacity: 1 !important;
			padding: 12px 24px 0;
		}

		.mobile-sort {
			display: none;
		}

		.feed-main {
			padding: 28px 20px 60px;
		}

		.masonry {
			columns: 3;
		}
	}

	@media (max-width: 560px) {
		.cover-brand {
			padding: 16px 16px !important;
		}

		.cover-content {
			padding: 0 16px !important;
		}

		.cover-title {
			font-size: 2rem !important;
		}

		.cover-nav {
			padding: 16px 16px;
		}

		.feed-main {
			padding: 20px 12px 48px;
		}

		.masonry {
			columns: 1;
		}

		.card-meta {
			flex-direction: column;
			align-items: flex-start;
			gap: 4px;
		}
	}

	@media (max-width: 380px) {
		.masonry {
			columns: 1;
		}
	}
</style>
