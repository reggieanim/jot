<script lang="ts">
	import { page } from '$app/stores';
	import { env } from '$env/dynamic/public';
	import { onMount } from 'svelte';
	import type { ApiPage } from '$lib/editor/types';
	import { user as authUser } from '$lib/stores/auth';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	$: username = $page.params.username || 'anonymous';

	type PublicProfile = {
		id: string;
		username: string;
		display_name: string;
		bio: string;
		avatar_url: string;
		follower_count: number;
		follow_count: number;
	};

	let profile: PublicProfile | null = null;
	let pages: ApiPage[] = [];
	let loading = true;
	let error = '';
	let isFollowing = false;
	let followLoading = false;

	/** Whether this is the logged-in user's own profile */
	$: isOwnProfile = $authUser?.username === username;

	/** Per-card cinematic tint extracted from cover image */
	let cardTints: Record<string, { bg: string; border: string; shadow: string; muted: string }> = {};

	/* default cover patterns */
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
			} catch { /* ignore */ }
		};
		img.src = imgSrc;
	}

	function cinematicStyle(p: ApiPage): string {
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

	onMount(async () => {
		try {
			/* Fetch the public profile */
			const profileRes = await fetch(`${apiUrl}/v1/users/username/${encodeURIComponent(username)}`, { credentials: 'include' });
			if (!profileRes.ok) throw new Error('User not found');
			profile = await profileRes.json();

			/* Fetch the user's published pages */
			const res = await fetch(`${apiUrl}/v1/users/${encodeURIComponent(profile.id)}/pages`);
			if (!res.ok) throw new Error('Failed to load pages');
			const payload = await res.json();
			const all: ApiPage[] = payload?.items ?? [];
			pages = all;

			/* Check if the logged-in user is following this profile */
			if ($authUser && profile && !isOwnProfile) {
				try {
					const followRes = await fetch(`${apiUrl}/v1/users/${profile.id}/is-following`, { credentials: 'include' });
					if (followRes.ok) {
						const data = await followRes.json();
						isFollowing = data.following;
					}
				} catch { /* ignore */ }
			}

			for (const p of pages) {
				if (!p.cinematic) continue;
				const img = imageFor(p);
				if (img) extractQuickTint(img, p.id);
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load pages';
		} finally {
			loading = false;
		}
	});

	async function toggleFollow() {
		if (!profile || !$authUser || followLoading) return;
		followLoading = true;
		try {
			const method = isFollowing ? 'DELETE' : 'POST';
			const res = await fetch(`${apiUrl}/v1/users/${profile.id}/follow`, { method, credentials: 'include' });
			if (!res.ok) throw new Error('Failed');
			isFollowing = !isFollowing;
			if (profile) {
				profile = { ...profile, follower_count: profile.follower_count + (isFollowing ? 1 : -1) };
			}
		} catch { /* ignore */ }
		finally { followLoading = false; }
	}

	function formatDate(iso?: string) {
		if (!iso) return '';
		return new Date(iso).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	function imageFor(p: ApiPage): string | null {
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

	function embedFor(p: ApiPage): string | null {
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

	$: totalProofreads = pages.reduce((s, p) => s + (p.proofread_count ?? 0), 0);
</script>

<div class="profile-page">
	{#if loading}
		<div class="loading-wrap">
			<div class="spinner"></div>
		</div>
	{:else if error}
		<div class="loading-wrap"><p class="error-text">{error}</p></div>
	{:else}
		<!-- LEFT SIDEBAR -->
		<aside class="sidebar">
			<!-- cover / banner area -->
			<div class="sidebar-cover">
				{#if pages.length > 0 && imageFor(pages[0])}
					<img class="sidebar-cover-img" src={imageFor(pages[0])} alt="Cover" />
				{/if}
				<div class="sidebar-cover-overlay"></div>
				<a href="/" class="sidebar-brand">Jot.</a>
			</div>

			<!-- profile info -->
			<div class="sidebar-profile">
				<div class="sidebar-avatar">
					{#if profile?.avatar_url}
						<img class="sidebar-avatar-img" src={profile.avatar_url} alt={profile.display_name || username} />
					{:else}
						<span class="sidebar-avatar-letter">{(username ?? '?').charAt(0).toUpperCase()}</span>
					{/if}
				</div>
				<h1 class="sidebar-name">{profile?.display_name || `@${username}`}</h1>
				<span class="sidebar-username">@{username}</span>
				{#if profile?.bio}
					<p class="sidebar-bio">{profile.bio}</p>
				{:else}
					<p class="sidebar-bio">Writer on Jot.</p>
				{/if}

				<div class="sidebar-stats">
					<div class="sidebar-stat">
						<span class="sidebar-stat-val">{pages.length}</span>
						<span class="sidebar-stat-lbl">Posts</span>
					</div>
					<div class="sidebar-stat">
						<span class="sidebar-stat-val">{totalProofreads}</span>
						<span class="sidebar-stat-lbl">Proofreads</span>
					</div>
					<div class="sidebar-stat">
						<span class="sidebar-stat-val">{profile?.follower_count ?? 0}</span>
						<span class="sidebar-stat-lbl">Followers</span>
					</div>
					<div class="sidebar-stat">
						<span class="sidebar-stat-val">{profile?.follow_count ?? 0}</span>
						<span class="sidebar-stat-lbl">Following</span>
					</div>
				</div>

				{#if $authUser && profile && !isOwnProfile}
					<button class="sidebar-follow" class:following={isFollowing} on:click={toggleFollow} disabled={followLoading}>
						{isFollowing ? 'Following' : 'Follow'}
					</button>
				{/if}
			</div>

			<nav class="sidebar-nav">
				<a href="/" class="sidebar-nav-link">Home</a>
				<a href="/feed" class="sidebar-nav-link">Feed</a>
				<a href="/editor" class="sidebar-nav-link">Editor</a>
				{#if $authUser}
					<a href="/editor" class="sidebar-nav-cta">+ New page</a>
				{:else}
					<a href="/signup" class="sidebar-nav-cta">Try it free</a>
				{/if}
			</nav>

			<div class="sidebar-footer">
				<span class="sidebar-footer-copy">© 2026 Jot.</span>
			</div>
		</aside>

		<!-- MAIN CONTENT -->
		<main class="main">
			{#if pages.length === 0}
				<div class="empty">
					<div class="empty-icon">✦</div>
					<p>No published pages yet.</p>
				</div>
			{:else}
				<div class="masonry">
					{#each pages as p, idx (p.id)}
						{@const img = imageFor(p)}
						{@const emb = embedFor(p)}
						<a class="card" href={`/public/${p.id}`} class:tall={idx % 3 === 0} class:dark={p.dark_mode} class:cinematic={p.cinematic} class:has-user-bg={!!p.bg_color} style={cinematicStyle(p)}>
							<div class="card-visual" style={!img && !emb ? `background:${patternFor(p)}` : ''}>
								{#if img}
									<img src={img} alt={p.title || 'Page image'} />
								{:else if emb}
									<iframe src={emb} title="Embedded content" loading="lazy" sandbox="allow-scripts allow-same-origin"></iframe>
								{:else}
									<div class="card-default-icon">✦</div>
								{/if}
							</div>
							<div class="card-body">
								<span class="card-tag">{p.title ? p.title.split(' ').slice(0, 3).join(' ').toUpperCase() : 'UNTITLED'}</span>
								<h3 class="card-title">{p.title || 'Untitled'}</h3>
							<div class="card-author">
								<span>✎ {profile?.display_name || username}.</span>
								<span>📅 {formatDate(p.published_at || p.updated_at)}</span>
							</div>
							{#if p.proofread_count}
								<span class="card-proofreads">{p.proofread_count} proofread{p.proofread_count === 1 ? '' : 's'}</span>
							{/if}
							<div class="card-read-more">
									<span>READ MORE</span>
								</div>
							</div>
						</a>
					{/each}
				</div>
			{/if}
		</main>
	{/if}
</div>

<style>
	:global(body) {
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	/* ---- TWO-COLUMN LAYOUT ---- */
	.profile-page {
		display: grid;
		grid-template-columns: 72px 1fr;
		min-height: 100vh;
		transition: grid-template-columns 0.35s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.profile-page:has(.sidebar:hover) {
		grid-template-columns: 340px 1fr;
	}

	/* ---- SIDEBAR ---- */
	.sidebar {
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

	.sidebar:hover {
		overflow-y: auto;
	}

	.sidebar-cover {
		position: relative;
		width: 100%;
		height: 0;
		background: #111;
		overflow: hidden;
		flex-shrink: 0;
		transition: height 0.35s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.sidebar:hover .sidebar-cover {
		height: 320px;
	}

	.sidebar-cover-img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
		filter: brightness(0.55) saturate(1.1);
	}

	.sidebar-cover-overlay {
		position: absolute;
		inset: 0;
		background: linear-gradient(to bottom, rgba(14,14,14,0) 30%, rgba(14,14,14,0.95) 100%);
		pointer-events: none;
	}

	.sidebar-brand {
		position: absolute;
		top: 20px;
		left: 24px;
		font-size: 1.4rem;
		font-weight: 900;
		color: #fff;
		text-decoration: none;
		letter-spacing: -0.04em;
		z-index: 2;
		opacity: 0;
		transition: opacity 0.25s;
		pointer-events: none;
	}

	.sidebar:hover .sidebar-brand {
		opacity: 0.85;
		pointer-events: auto;
	}

	.sidebar:hover .sidebar-brand:hover {
		opacity: 1;
	}

	/* ---- SIDEBAR PROFILE ---- */
	.sidebar-profile {
		padding: 16px 14px 16px;
		margin-top: 0;
		position: relative;
		z-index: 2;
		display: flex;
		flex-direction: column;
		align-items: center;
		transition: padding 0.35s, margin-top 0.35s;
	}

	.sidebar:hover .sidebar-profile {
		padding: 0 28px 28px;
		margin-top: -40px;
		align-items: flex-start;
	}

	.sidebar-avatar {
		width: 44px;
		height: 44px;
		border-radius: 50%;
		border: 3px solid #0e0e0e;
		background: #1a1a1a;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		overflow: hidden;
		box-shadow: 0 4px 20px rgba(0,0,0,0.4);
		transition: width 0.35s, height 0.35s;
	}

	.sidebar:hover .sidebar-avatar {
		width: 80px;
		height: 80px;
	}

	.sidebar-avatar-img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
	}

	.sidebar-avatar-letter {
		font-size: 30px;
		font-weight: 900;
		color: #fff;
		text-transform: uppercase;
		letter-spacing: -0.04em;
	}

	.sidebar-name {
		font-size: 22px;
		font-weight: 900;
		letter-spacing: -0.03em;
		margin: 16px 0 0;
		line-height: 1.15;
		color: #fff;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.25s, max-height 0.35s;
	}

	.sidebar:hover .sidebar-name {
		opacity: 1;
		max-height: 60px;
	}

	.sidebar-username {
		font-size: 13px;
		color: #777;
		margin-top: 3px;
		font-weight: 500;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.25s, max-height 0.35s;
	}

	.sidebar:hover .sidebar-username {
		opacity: 1;
		max-height: 24px;
	}

	.sidebar-bio {
		font-size: 13px;
		color: #999;
		line-height: 1.6;
		margin: 12px 0 0;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.25s, max-height 0.35s;
	}

	.sidebar:hover .sidebar-bio {
		opacity: 1;
		max-height: 100px;
	}

	/* ---- SIDEBAR STATS ---- */
	.sidebar-stats {
		display: flex;
		gap: 20px;
		margin-top: 24px;
		padding-top: 20px;
		border-top: 1px solid #2a2a2a;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.25s, max-height 0.35s;
	}

	.sidebar:hover .sidebar-stats {
		opacity: 1;
		max-height: 80px;
	}

	.sidebar-stat {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
	}

	.sidebar-stat-val {
		font-size: 20px;
		font-weight: 900;
		letter-spacing: -0.03em;
		color: #fff;
	}

	.sidebar-stat-lbl {
		font-size: 9px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: #666;
	}

	/* ---- SIDEBAR FOLLOW ---- */
	.sidebar-follow {
		margin-top: 24px;
		padding: 9px 0;
		width: 100%;
		font-family: inherit;
		font-size: 13px;
		font-weight: 700;
		color: #0e0e0e;
		background: #fff;
		border: 2px solid #fff;
		cursor: pointer;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		transition: background 0.15s, transform 0.15s, box-shadow 0.15s, opacity 0.25s, max-height 0.35s;
		opacity: 0;
		max-height: 0;
		overflow: hidden;
	}

	.sidebar:hover .sidebar-follow {
		opacity: 1;
		max-height: 50px;
	}

	.sidebar-follow:hover {
		background: #e0e0e0;
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(255,255,255,0.15);
	}

	.sidebar-follow.following {
		background: transparent;
		color: #fff;
		border-color: #555;
	}

	.sidebar-follow.following:hover {
		border-color: #888;
		background: rgba(255,255,255,0.05);
	}

	.sidebar-follow:disabled {
		opacity: 0.4;
		cursor: default;
	}

	/* ---- SIDEBAR NAV ---- */
	.sidebar-nav {
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

	.sidebar:hover .sidebar-nav {
		opacity: 1;
		max-height: 200px;
	}

	.sidebar-nav-link {
		font-size: 13px;
		font-weight: 600;
		color: #999;
		text-decoration: none;
		transition: color 0.15s;
	}

	.sidebar-nav-link:hover {
		color: #fff;
	}

	.sidebar-nav-cta {
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

	.sidebar-nav-cta:hover {
		background: #e0e0e0;
	}

	.sidebar-footer {
		padding: 16px 28px 24px;
		opacity: 0;
		transition: opacity 0.25s;
	}

	.sidebar:hover .sidebar-footer {
		opacity: 1;
	}

	.sidebar-footer-copy {
		font-size: 11px;
		color: #444;
		font-weight: 500;
	}

	/* ---- MAIN CONTENT ---- */
	.main {
		padding: 40px 36px 80px;
		min-height: 100vh;
		max-width: 1200px;
	}

	/* ---- LOADING / EMPTY ---- */
	.loading-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		grid-column: 1 / -1;
	}

	.spinner {
		width: 20px;
		height: 20px;
		border: 2px solid #ddd;
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
		padding: 120px 20px;
	}

	.empty-icon {
		font-size: 48px;
		margin-bottom: 16px;
		opacity: 0.2;
	}

	.empty p {
		font-size: 16px;
		color: #888;
		margin: 0;
	}

	/* ---- MASONRY ---- */
	.masonry {
		columns: 3;
		column-gap: 14px;
	}

	/* ---- CARD ---- */
	.card {
		display: inline-flex;
		flex-direction: column;
		width: 100%;
		margin-bottom: 14px;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		overflow: hidden;
		text-decoration: none;
		color: inherit;
		transition: transform 0.12s ease, box-shadow 0.12s ease;
		break-inside: avoid;
		position: relative;
	}

	.card:hover {
		transform: translateY(-3px);
		box-shadow: 6px 6px 0 #1a1a1a;
	}

	/* ---- CARD VISUAL ---- */
	.card-visual {
		width: 100%;
		min-height: 130px;
		background: #e8e8e4;
		overflow: hidden;
		display: flex;
		align-items: center;
		justify-content: center;
		position: relative;
	}

	.card.tall .card-visual {
		min-height: 200px;
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
		padding: 12px 14px 10px;
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
		font-size: 14px;
		font-weight: 800;
		letter-spacing: -0.02em;
		margin: 2px 0 0;
		line-height: 1.35;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.card-author {
		display: flex;
		gap: 10px;
		font-size: 10px;
		color: #888;
		font-weight: 500;
		margin-top: 2px;
	}

	/* ---- READ MORE BAR ---- */
	.card-proofreads {
		font-size: 10px;
		font-weight: 700;
		color: #888;
		letter-spacing: 0.02em;
	}

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

	.card.dark .card-author {
		color: #666;
	}

	.card.dark .card-tag {
		color: #ccc;
		border-bottom-color: #555;
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

	/* ---- CINEMATIC MODE CARD ---- */
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

	.card.cinematic .card-author {
		color: var(--card-muted, #8a8580);
	}

	.card.cinematic .card-tag {
		color: #3a3632;
		border-bottom-color: #3a3632;
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

	.card.dark.cinematic .card-author {
		color: #6a665e;
	}

	.card.dark.cinematic .card-tag {
		color: #d4cfc4;
		border-bottom-color: #3a3630;
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
	@media (max-width: 1100px) {
		.profile-page:has(.sidebar:hover) {
			grid-template-columns: 300px 1fr;
		}
		.main {
			padding: 32px 24px 60px;
		}
	}

	@media (max-width: 860px) {
		.profile-page {
			grid-template-columns: 1fr;
		}

		.profile-page:has(.sidebar:hover) {
			grid-template-columns: 1fr;
		}

		.sidebar {
			position: relative;
			height: auto;
			border-right: none;
			border-bottom: 3px solid #1a1a1a;
			overflow: visible;
		}

		/* On mobile, show everything expanded — no hover gating */
		.sidebar-cover {
			height: 240px;
		}

		.sidebar-brand {
			opacity: 0.85 !important;
			pointer-events: auto !important;
		}

		.sidebar-profile {
			padding: 0 24px 24px !important;
			margin-top: -36px !important;
			align-items: flex-start !important;
		}

		.sidebar-avatar {
			width: 72px !important;
			height: 72px !important;
		}

		.sidebar-name,
		.sidebar-username,
		.sidebar-bio {
			opacity: 1 !important;
			max-height: none !important;
		}

		.sidebar-stats {
			opacity: 1 !important;
			max-height: none !important;
			flex-wrap: wrap;
		}

		.sidebar-follow {
			opacity: 1 !important;
			max-height: none !important;
		}

		.sidebar-nav {
			opacity: 1 !important;
			max-height: none !important;
		}

		.sidebar-footer {
			opacity: 1 !important;
		}

		.main {
			padding: 28px 20px 60px;
		}

		.masonry {
			columns: 3;
		}
	}

	@media (max-width: 560px) {
		.sidebar-cover {
			height: 180px;
		}

		.sidebar-profile {
			padding: 0 16px 20px !important;
			margin-top: -30px !important;
		}

		.sidebar-avatar {
			width: 60px !important;
			height: 60px !important;
		}

		.sidebar-name {
			font-size: 18px;
		}

		.masonry {
			columns: 2;
		}

		.sidebar-stats {
			gap: 16px;
		}

		.main {
			padding: 20px 12px 48px;
		}
	}

	@media (max-width: 380px) {
		.masonry {
			columns: 1;
		}

		.sidebar-cover {
			height: 160px;
		}
	}
</style>
