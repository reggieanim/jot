<script lang="ts">
	import { page } from '$app/stores';
	import { env } from '$env/dynamic/public';
	import Cover from '$lib/components/Cover.svelte';
	import ReadonlyBlocks from '$lib/components/ReadonlyBlocks.svelte';
	import { plainTextFromBlockData } from '$lib/editor/richtext';
	import { buildThemeStyle, DEFAULT_THEME, extractPaletteFromImage } from '$lib/editor/theme';
	import type { ApiBlock, ApiPage, ApiProofread, Rgb } from '$lib/editor/types';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let title = 'Untitled';
	let cover: string | null = null;
	let blocks: ApiBlock[] = [];
	let cinematicEnabled = true;
	let darkMode = false;
	let bgColor = '';
	let moodStrength = 65;
	const FALLBACK_BASE: Rgb = [205, 207, 214];
	const FALLBACK_ACCENT: Rgb = [124, 92, 255];
	let paletteBase: Rgb = FALLBACK_BASE;
	let paletteAccent: Rgb = FALLBACK_ACCENT;
	let themeStyle = DEFAULT_THEME;
	let readCount = 0;

	let loading = true;
	let error = '';
	let creating = false;
	let hasShareLinks = false;

	type ApiCollabUser = {
		user_id: string;
		username: string;
		display_name: string;
		avatar_url: string;
		access: string;
	};
	let collabUsers: ApiCollabUser[] = [];

	let proofreadMode = false;
	let selectedBlockId = '';
	let proofreads: ApiProofread[] = [];
	let draftStates: Record<string, { kind: string; text: string }> = {};
	let authorName = '';
	let proofreadTitle = '';
	let proofreadSummary = '';
	let proofreadStance = 'review';

	function setThemeFromRgb(base: Rgb, accent: Rgb) {
		paletteBase = base;
		paletteAccent = accent;
		themeStyle = buildThemeStyle(base, accent, { darkMode, cinematicEnabled, moodStrength });
	}

	async function applyCoverPalette(imageSrc: string | null) {
		const palette = await extractPaletteFromImage(imageSrc, FALLBACK_BASE, FALLBACK_ACCENT);
		setThemeFromRgb(palette.base, palette.accent);
	}

	async function loadPageAndProofreads() {
		const pageId = $page.params.pageId;
		if (!pageId) return;
		loading = true;
		error = '';
		try {
			const pageRes = await fetch(`${apiUrl}/v1/public/pages/${encodeURIComponent(pageId)}`);
			if (!pageRes.ok) throw new Error('Page not found or not published');
			const currentPage: ApiPage = await pageRes.json();

			title = currentPage.title || 'Untitled';
			cover = currentPage.cover || null;
			blocks = currentPage.blocks || [];
			readCount = currentPage.read_count || 0;
			hasShareLinks = !!currentPage.has_share_links;
			darkMode = !!currentPage.dark_mode;
			cinematicEnabled = currentPage.cinematic !== false;
			moodStrength = Number(currentPage.mood ?? 65);
			bgColor = currentPage.bg_color || '';
			await applyCoverPalette(cover);

			const proofRes = await fetch(`${apiUrl}/v1/public/pages/${encodeURIComponent(pageId)}/proofreads`);
			if (proofRes.ok) {
				const payload = await proofRes.json();
				proofreads = payload?.items || [];
			}

			if (hasShareLinks) {
				const collabRes = await fetch(`${apiUrl}/v1/public/pages/${encodeURIComponent(pageId)}/collaborators`);
				if (collabRes.ok) {
					const collabPayload = await collabRes.json();
					collabUsers = collabPayload?.collaborators || [];
				}
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load page';
		} finally {
			loading = false;
		}
	}

	$: if ($page.params.pageId) {
		void loadPageAndProofreads();
	}

	function blockIdFor(block: ApiBlock, index: number) {
		return block.id || `${block.type}-${index}`;
	}

	function blockTextFor(block: ApiBlock) {
		const richText = plainTextFromBlockData(block.data);
		if (richText) return richText;
		if (typeof block.data?.url === 'string') return block.data.url;
		return '';
	}

	function handleSelectBlock(event: CustomEvent<{ blockId: string }>) {
		selectedBlockId = event.detail.blockId;
	}

	function updateSelectedDraftKind(kind: string) {
		if (!selectedBlockId) return;
		draftStates = {
			...draftStates,
			[selectedBlockId]: {
				kind,
				text: draftStates[selectedBlockId]?.text || ''
			}
		};
	}

	function updateSelectedDraftText(text: string) {
		if (!selectedBlockId) return;
		draftStates = {
			...draftStates,
			[selectedBlockId]: {
				kind: draftStates[selectedBlockId]?.kind || 'note',
				text
			}
		};
	}

	function removeSelectedDraft() {
		if (!selectedBlockId) return;
		const next = { ...draftStates };
		delete next[selectedBlockId];
		draftStates = next;
	}

	function toggleProofreadMode() {
		proofreadMode = !proofreadMode;
		if (!proofreadMode) selectedBlockId = '';
	}

	function makeId() {
		if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
		return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	}

	$: selectedBlock = blocks.find((block, index) => blockIdFor(block, index) === selectedBlockId) || null;
	$: selectedDraft = selectedBlockId ? draftStates[selectedBlockId] || { kind: 'note', text: '' } : { kind: 'note', text: '' };

	async function saveProofread() {
		const pageId = $page.params.pageId;
		if (!pageId || !authorName.trim() || !proofreadTitle.trim()) {
			error = 'Author name and proofread title are required.';
			return;
		}

		const annotations = blocks
			.map((block, index) => {
				const blockId = blockIdFor(block, index);
				const draft = draftStates[blockId];
				if (!draft?.text?.trim()) return null;
				return {
					id: makeId(),
					block_id: blockId,
					kind: draft.kind || 'note',
					quote: blockTextFor(block).slice(0, 200),
					text: draft.text.trim()
				};
			})
			.filter(Boolean);

		if (annotations.length === 0) {
			error = 'Add at least one block proofread note before saving.';
			return;
		}

		creating = true;
		error = '';
		try {
			const response = await fetch(`${apiUrl}/v1/public/pages/${encodeURIComponent(pageId)}/proofreads`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					author_name: authorName.trim(),
					title: proofreadTitle.trim(),
					summary: proofreadSummary.trim(),
					stance: proofreadStance,
					annotations
				})
			});

			if (!response.ok) throw new Error('Failed to save proofread');
			const saved = await response.json();
			if (saved?.id) {
				window.location.href = `/proofread/${saved.id}`;
				return;
			}
			await loadPageAndProofreads();
			proofreadMode = false;
			selectedBlockId = '';
			draftStates = {};
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save proofread';
		} finally {
			creating = false;
		}
	}
</script>

<div class="editor-shell">
	{#if loading}
		<main class="editor-main" style={themeStyle}><div class="editor-wrapper"><p>Loading…</p></div></main>
	{:else if error}
		<main class="editor-main" style={themeStyle}><div class="editor-wrapper"><p class="error">{error}</p></div></main>
	{:else}
		<aside class="cover-rail" class:has-cover={!!cover}>
			<Cover {cover} {apiUrl} {title} {blocks} readonly={true} />
		</aside>

		<main class="editor-main" class:dark={darkMode} class:cinematic-on={cinematicEnabled} class:has-bg-color={!!bgColor} style="{themeStyle}{bgColor ? `--note-user-bg:${bgColor};` : ''}">
			<div class="editor-wrapper">
				<div class="page-header">
					<div class="page-title-wrap">
						<h1 class="page-title">{title}</h1>
						<div class="proofread-topbar">
							<a class="feed-link" href="/feed" aria-label="Browse feed" title="Browse feed">Feed</a>
							<button
								class="proofread-toggle"
								class:active={proofreadMode}
								on:click={toggleProofreadMode}
								aria-label={proofreadMode ? 'Close proofread editor' : 'Start proofread'}
								title={proofreadMode ? 'Close proofread editor' : 'Start proofread'}
							>
								{#if proofreadMode}
									<svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
										<path d="M6 6L18 18M18 6L6 18" />
									</svg>
								{:else}
									<svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
										<path d="M4 20h4l10-10-4-4L4 16v4z" />
										<path d="M12 6l4 4" />
									</svg>
								{/if}
							</button>
							<span class="proofread-count">{Object.values(draftStates).filter((item) => item?.text?.trim()).length} notes</span>
						<span class="proofread-count">{readCount} reads</span>
						{#if hasShareLinks}
							{#if collabUsers.length > 0}
								<div class="pub-collab-avatars">
								{#each collabUsers.slice(0, 4) as cu (cu.user_id)}
									{#if cu.avatar_url}
										<a href={`/user/${cu.username}`} class="collab-avatar-link"><img class="pub-collab-avatar" src={cu.avatar_url} alt={cu.display_name || cu.username} title={cu.display_name || cu.username} /></a>
									{:else}
										<a href={`/user/${cu.username}`} class="collab-avatar-link"><span class="pub-collab-avatar pub-collab-avatar-letter" title={cu.display_name || cu.username}>{(cu.display_name || cu.username || '?').charAt(0).toUpperCase()}</span></a>
									{/if}
								{/each}
									{#if collabUsers.length > 4}
										<span class="pub-collab-avatar pub-collab-avatar-more">+{collabUsers.length - 4}</span>
									{/if}
									<span class="pub-collab-label">{collabUsers.length} collab{collabUsers.length === 1 ? '' : 's'}</span>
								</div>
							{:else}
								<span class="page-collab-badge" title="Live collaboration enabled">
									<span class="page-collab-dot"></span>
									Collab
								</span>
							{/if}
						{/if}
						</div>
					</div>
				</div>

				<div class="proofread-layout" class:active={proofreadMode}>
					<div class="blocks-container">
						<ReadonlyBlocks
							{blocks}
							pageId={$page.params.pageId}
							interactive={proofreadMode}
							{selectedBlockId}
							{draftStates}
							{proofreads}
							on:select={handleSelectBlock}
						/>

						{#if proofreads.length > 0}
							<div class="published-proofreads">
								<h3>Published proofreads</h3>
								<div class="proofread-list">
									{#each proofreads as proofread (proofread.id)}
										<a class="proofread-item" href={`/proofread/${proofread.id}`}>
											<strong>{proofread.title}</strong>
											<span>{proofread.author_name} · {proofread.stance}</span>
										</a>
									{/each}
								</div>
							</div>
						{/if}
					</div>

					{#if proofreadMode}
						<aside class="composer-panel">
							<h2>Proofread editor</h2>
							<p class="composer-help">Select a block to write over it. Original text fades while your commentary appears inline.</p>

							<div class="composer-fields">
								<input placeholder="Your name" bind:value={authorName} />
								<input placeholder="Proofread title" bind:value={proofreadTitle} />
								<input placeholder="Summary" bind:value={proofreadSummary} />
								<select bind:value={proofreadStance}>
									<option value="review">Review</option>
									<option value="assert">Assert</option>
									<option value="debunk">Debunk</option>
								</select>
							</div>

							{#if selectedBlock}
								<div class="block-editor">
									<div class="selected-badge">Editing block: {selectedBlockId}</div>
									<select value={selectedDraft.kind} on:change={(event) => updateSelectedDraftKind((event.target as HTMLSelectElement).value)}>
										<option value="note">Note</option>
										<option value="assert">Assert</option>
										<option value="debunk">Debunk</option>
										<option value="strike">Strike</option>
									</select>
									<textarea
										rows="6"
										placeholder="Write your proofread for this block..."
										value={selectedDraft.text}
										on:input={(event) => updateSelectedDraftText((event.target as HTMLTextAreaElement).value)}
									></textarea>
									<button class="ghost" on:click={removeSelectedDraft}>Clear selected note</button>
								</div>
							{:else}
								<p class="composer-help">Click a block to begin writing your proofread overlay.</p>
							{/if}

							{#if error}
								<p class="error">{error}</p>
							{/if}

							<button class="save-btn" on:click={saveProofread} disabled={creating}>
								{creating ? 'Saving…' : 'Publish proofread'}
							</button>
						</aside>
					{/if}
				</div>
			</div>
		</main>
	{/if}
</div>

<style>
	:global(body) {
		margin: 0;
		--font-ui: 'Moderat', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
		--font-display: 'Moderat', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
		font-family: var(--font-ui);
		background: var(--note-bg, #ffffff);
		color: var(--note-text, #1f2328);
	}

	.editor-shell {
		display: flex;
		flex-direction: row;
		min-height: 100vh;
	}

	.cover-rail {
		width: 24px;
		min-width: 24px;
		flex: 0 0 24px;
		background: transparent;
		border-right: 1px solid transparent;
		overflow: hidden;
		transition: width 0.45s cubic-bezier(0.4, 0, 0.2, 1),
			min-width 0.45s cubic-bezier(0.4, 0, 0.2, 1),
			flex-basis 0.45s cubic-bezier(0.4, 0, 0.2, 1),
			border-color 0.3s ease;
	}

	.cover-rail:hover,
	.cover-rail.has-cover {
		width: 280px;
		min-width: 280px;
		flex: 0 0 280px;
	}

	.cover-rail.has-cover {
		border-right: 1px solid var(--note-rail-border, #f1f3f5);
	}

	.cover-rail:not(.has-cover):hover {
		border-right: 1px solid color-mix(in srgb, var(--note-rail-border, #f1f3f5) 65%, transparent);
	}

	.editor-main {
		flex: 1;
		min-width: 0;
		background: radial-gradient(circle at top, color-mix(in srgb, var(--note-accent, #7c5cff) var(--note-wash, 8%), var(--note-bg, #ffffff)) 0%, var(--note-bg, #ffffff) 48%);
		position: relative;
		overflow: hidden;
		transition: flex 0.45s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.editor-main.dark {
		background: #000000;
	}

	.editor-main.dark::after {
		content: '';
		position: absolute;
		inset: 0;
		pointer-events: none;
		opacity: 0;
		background:
			radial-gradient(120% 80% at 50% -10%, color-mix(in srgb, var(--note-accent, #7c5cff) 22%, transparent) 0%, transparent 58%),
			radial-gradient(120% 100% at 50% 100%, rgba(0, 0, 0, 0) 38%, rgba(0, 0, 0, 0.78) 100%),
			linear-gradient(180deg, rgba(255, 255, 255, 0.06) 0%, rgba(0, 0, 0, 0.45) 55%, rgba(0, 0, 0, 0.82) 100%);
		mix-blend-mode: screen;
		transition: opacity 0.25s ease;
	}

	.editor-main.dark.cinematic-on::after {
		opacity: clamp(0.22, calc(var(--note-fade, 0.04) * 1.8), 0.6);
	}

	.editor-main::before {
		content: '';
		position: absolute;
		inset: 0;
		pointer-events: none;
		opacity: var(--note-grain-opacity, 0);
		background-image:
			radial-gradient(circle at 20% 30%, rgba(255, 255, 255, 0.5) 0.55px, transparent 0.8px),
			radial-gradient(circle at 80% 20%, rgba(0, 0, 0, 0.42) 0.6px, transparent 0.95px),
			radial-gradient(circle at 35% 70%, rgba(0, 0, 0, 0.3) 0.5px, transparent 0.9px),
			linear-gradient(180deg, rgba(255, 244, 228, var(--note-fade, 0)) 0%, rgba(0, 0, 0, 0) 40%, rgba(10, 8, 6, calc(var(--note-fade, 0) * 0.7)) 100%);
		background-size: 3px 3px, 4px 4px, 5px 5px, 100% 100%;
		mix-blend-mode: soft-light;
	}

	.editor-main.has-bg-color {
		background: var(--note-user-bg) !important;
		--note-bg: var(--note-user-bg);
		--note-surface: var(--note-user-bg);
	}

	.editor-main.has-bg-color::after {
		opacity: 0 !important;
	}

	.editor-main.has-bg-color::before {
		opacity: 0 !important;
	}

	.editor-wrapper {
		flex: 1;
		max-width: 900px;
		margin: 0 auto;
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		padding: 0 64px;
		background: transparent;
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

	.page-header {
		margin: 32px 0 24px;
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: 20px;
	}

	.page-title-wrap {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.proofread-topbar {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}

	.feed-link {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		height: 34px;
		padding: 0 12px;
		border: 2px solid #1a1a1a;
		border-radius: 8px;
		background: #fff;
		color: #1a1a1a;
		font-size: 12px;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-decoration: none;
		transition: transform 0.12s, box-shadow 0.12s;
	}

	.feed-link:hover {
		transform: translateY(-2px);
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.proofread-toggle {
		border: 2px solid #1a1a1a;
		background: #fff;
		color: #1a1a1a;
		width: 34px;
		height: 34px;
		padding: 0;
		border-radius: 8px;
		cursor: pointer;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		transition: transform 0.12s, box-shadow 0.12s;
	}

	.proofread-toggle svg {
		width: 16px;
		height: 16px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.9;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.proofread-toggle.active {
		background: #1a1a1a;
		color: #fff;
	}

	.proofread-toggle:hover {
		transform: translateY(-2px);
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.proofread-count {
		font-size: 12px;
		font-weight: 600;
		color: var(--note-muted, #6b7280);
	}

	/* ── Collab badge on public page ── */
	.page-collab-badge {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: 10px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.09em;
		color: #1a1a1a;
		border: 1.5px solid #1a1a1a;
		padding: 2px 7px 2px 5px;
		border-radius: 4px;
		background: transparent;
	}

	.page-collab-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: #1a1a1a;
		flex-shrink: 0;
		animation: collab-pulse-pub 2.2s ease-in-out infinite;
	}

	@keyframes collab-pulse-pub {
		0%, 100% { opacity: 1; transform: scale(1); }
		50% { opacity: 0.35; transform: scale(0.75); }
	}

	/* Public collab avatar strip */
	.pub-collab-avatars {
		display: inline-flex;
		align-items: center;
	}

	.pub-collab-avatar {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		border: 2px solid var(--note-bg, #fff);
		object-fit: cover;
		margin-right: -5px;
		flex-shrink: 0;
	}

	.pub-collab-avatar-letter,
	.pub-collab-avatar-more {
		background: #1a1a1a;
		color: #fafaf8;
		font-size: 8px;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		user-select: none;
	}

	.pub-collab-label {
		margin-left: 11px;
		font-size: 10px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.09em;
		color: var(--note-muted, #6b7280);
	}

	.collab-avatar-link {
		display: contents;
		text-decoration: none;
	}

	.page-title {
		font-size: clamp(24px, 5vw, 40px);
		font-weight: 700;
		line-height: 1.2;
		padding: 0;
		margin: 0;
		font-family: var(--font-display);
		letter-spacing: 0.01em;
		color: var(--note-title, #111827);
		text-shadow: 0 0 28px color-mix(in srgb, var(--note-title-glow, transparent) 20%, transparent);
		max-width: 100%;
		word-wrap: break-word;
		overflow-wrap: break-word;
		word-break: break-word;
	}

	.blocks-container {
		display: flex;
		flex-direction: column;
		gap: 0;
		width: 100%;
		min-width: 0;
		padding-bottom: 100px;
	}

	.proofread-layout {
		display: grid;
		grid-template-columns: 1fr;
		gap: 20px;
	}

	.proofread-layout.active {
		grid-template-columns: minmax(0, 1fr) 320px;
	}

	.composer-panel {
		position: sticky;
		top: 18px;
		height: fit-content;
		border: 2px solid #1a1a1a;
		background: #fff;
		border-radius: 8px;
		padding: 14px;
		display: grid;
		gap: 12px;
		box-shadow: 6px 6px 0 #1a1a1a;
	}

	.composer-panel h2 {
		margin: 0;
		font-size: 18px;
	}

	.composer-help {
		margin: 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--note-muted, #6b7280);
	}

	.composer-fields,
	.block-editor {
		display: grid;
		gap: 8px;
	}

	.composer-panel input,
	.composer-panel select,
	.composer-panel textarea {
		width: 100%;
		font: inherit;
		border: 2px solid #1a1a1a;
		background: #fff;
		color: #1a1a1a;
		padding: 8px 10px;
		border-radius: 6px;
	}

	.selected-badge {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.03em;
		text-transform: uppercase;
		color: var(--note-muted, #6b7280);
	}

	.save-btn,
	.ghost {
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		padding: 8px 16px;
		cursor: pointer;
		font-weight: 800;
		font-size: 13px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		transition: transform 0.12s, box-shadow 0.12s;
	}

	.save-btn:hover,
	.ghost:hover {
		transform: translateY(-2px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.save-btn {
		background: #1a1a1a;
		color: #fff;
	}

	.ghost {
		background: #fff;
		color: #1a1a1a;
	}

	.editor-main.dark .proofread-toggle {
		background: transparent;
		color: #f3f4f6;
		border-color: #f3f4f6;
	}

	.editor-main.dark .feed-link {
		background: transparent;
		color: #f3f4f6;
		border-color: #f3f4f6;
	}

	.editor-main.dark .feed-link:hover {
		box-shadow: 3px 3px 0 #f3f4f6;
	}

	.editor-main.dark .proofread-toggle.active {
		background: #f3f4f6;
		color: #1a1a1a;
	}

	.editor-main.dark .proofread-toggle:hover {
		box-shadow: 3px 3px 0 #f3f4f6;
	}

	.editor-main.dark .save-btn {
		background: #f3f4f6;
		color: #1a1a1a;
		border-color: #f3f4f6;
	}

	.editor-main.dark .ghost {
		background: transparent;
		color: #f3f4f6;
		border-color: #f3f4f6;
	}

	.editor-main.dark .composer-panel {
		background: #111;
		border-color: #f3f4f6;
		box-shadow: 6px 6px 0 rgba(243, 244, 246, 0.2);
	}

	.editor-main.dark .composer-panel input,
	.editor-main.dark .composer-panel select,
	.editor-main.dark .composer-panel textarea {
		background: #222;
		color: #f3f4f6;
		border-color: #f3f4f6;
	}

	.published-proofreads {
		margin-top: 26px;
		border-top: 1px solid color-mix(in srgb, var(--note-border, #d1d5db) 78%, transparent);
		padding-top: 14px;
	}

	.published-proofreads h3 {
		margin: 0 0 10px;
		font-size: 14px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--note-muted, #6b7280);
	}

	.proofread-list {
		display: grid;
		gap: 8px;
	}

	.proofread-item {
		display: grid;
		gap: 2px;
		padding: 10px;
		border-radius: 8px;
		border: 2px solid #1a1a1a;
		background: #fff;
		text-decoration: none;
		color: #1a1a1a;
		transition: transform 0.12s, box-shadow 0.12s;
	}

	.proofread-item:hover {
		transform: translateY(-2px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.proofread-item span {
		font-size: 12px;
		color: var(--note-muted, #6b7280);
	}

	.error {
		color: #b91c1c;
	}

	@media (max-width: 980px) {
		.editor-shell {
			flex-direction: column;
		}

		.cover-rail {
			width: 100%;
			min-width: 0;
			height: auto;
			max-height: 280px;
			flex: 0 0 auto;
			border-right: none;
			border-bottom: 1px solid var(--note-rail-border, #f1f3f5);
			transition: max-height 0.45s cubic-bezier(0.4, 0, 0.2, 1);
		}

		.cover-rail:not(.has-cover) {
			max-height: 0;
			border-bottom: none;
			overflow: hidden;
		}

		.cover-rail:hover,
		.cover-rail.has-cover {
			width: 100%;
			min-width: 0;
			flex: 0 0 auto;
		}

		.editor-wrapper {
			padding: 0 24px;
		}

		.page-header {
			flex-direction: column;
			align-items: stretch;
		}

		.proofread-layout.active {
			grid-template-columns: 1fr;
		}

		.composer-panel {
			position: static;
		}
	}

	@media (max-width: 680px) {
		.cover-rail.has-cover {
			max-height: 220px;
		}

		.editor-wrapper {
			padding: 0 12px;
		}

		.page-header {
			margin: 20px 0 16px;
		}
	}

	@media (max-width: 400px) {
		.cover-rail.has-cover {
			max-height: 180px;
		}

		.editor-wrapper {
			padding: 0 8px;
		}
	}

</style>
