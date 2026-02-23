<script lang="ts">
	import { page } from '$app/stores';
	import { env } from '$env/dynamic/public';
	import Cover from '$lib/components/Cover.svelte';
	import ReadonlyBlocks from '$lib/components/ReadonlyBlocks.svelte';
	import { buildThemeStyle, DEFAULT_THEME, extractPaletteFromImage } from '$lib/editor/theme';
	import type { ApiPage, ApiProofread, Rgb } from '$lib/editor/types';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';
	const originalAnchorPrefix = 'original-block-';

	let currentPage: ApiPage | null = null;
	let proofread: ApiProofread | null = null;
	let loading = true;
	let error = '';
	let themeStyle = DEFAULT_THEME;
	let cover: string | null = null;
	let darkMode = false;
	let cinematicEnabled = true;
	let moodStrength = 65;
	const FALLBACK_BASE: Rgb = [205, 207, 214];
	const FALLBACK_ACCENT: Rgb = [124, 92, 255];

	$: annotationByBlock = (proofread?.annotations || []).reduce((acc, item) => {
		const key = item.block_id || '__general__';
		if (!acc[key]) acc[key] = [];
		acc[key].push(item);
		return acc;
	}, {} as Record<string, NonNullable<ApiProofread['annotations']>>);

	function setThemeFromRgb(base: Rgb, accent: Rgb) {
		themeStyle = buildThemeStyle(base, accent, { darkMode, cinematicEnabled, moodStrength });
	}

	async function applyCoverPalette(imageSrc: string | null) {
		const palette = await extractPaletteFromImage(imageSrc, FALLBACK_BASE, FALLBACK_ACCENT);
		setThemeFromRgb(palette.base, palette.accent);
	}

	function noteLabel(index: number) {
		const n = index + 1;
		return `Ref ${n.toString().padStart(2, '0')}`;
	}

	function sourceHref(blockId: string) {
		return `#${originalAnchorPrefix}${blockId}`;
	}

	async function loadProofread() {
		const proofreadId = $page.params.proofreadId;
		if (!proofreadId) return;
		loading = true;
		error = '';
		try {
			const response = await fetch(`${apiUrl}/v1/public/proofreads/${encodeURIComponent(proofreadId)}`);
			if (!response.ok) throw new Error('Proofread not found');
			const payload = await response.json();
			proofread = payload?.proofread || null;
			currentPage = payload?.page || null;
			cover = currentPage?.cover || null;
			darkMode = !!currentPage?.dark_mode;
			cinematicEnabled = currentPage?.cinematic !== false;
			moodStrength = Number(currentPage?.mood ?? 65);
			await applyCoverPalette(cover);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load proofread';
		} finally {
			loading = false;
		}
	}

	$: if ($page.params.proofreadId) {
		void loadProofread();
	}
</script>

<div class="editor-shell">
	{#if loading}
		<main class="editor-main" style={themeStyle}><div class="editor-wrapper"><p>Loading…</p></div></main>
	{:else if error}
		<main class="editor-main" style={themeStyle}><div class="editor-wrapper"><p class="error">{error}</p></div></main>
	{:else if proofread && currentPage}
		<aside class="cover-rail" class:has-cover={!!cover}>
			<Cover {cover} {apiUrl} readonly={true} />
		</aside>

		<main class="editor-main" class:dark={darkMode} class:cinematic-on={cinematicEnabled} style={themeStyle}>
			<div class="editor-wrapper">
				<header class="head">
					<h1>{proofread.title}</h1>
					<p>{proofread.author_name} · {proofread.stance}</p>
					<p class="summary">{proofread.summary}</p>
				</header>

				<section class="grid">
					<article class="original">
						<h2>Original page</h2>
						<ReadonlyBlocks blocks={currentPage.blocks || []} anchorPrefix={originalAnchorPrefix} />
					</article>
					<aside class="notes">
						<h2>Proofread notes</h2>
						{#if (proofread.annotations || []).length === 0}
							<p>No annotations yet.</p>
						{:else}
							{#each Object.entries(annotationByBlock) as [blockId, notes]}
								<div class="note-group">
									<div class="block-label">
										<a class="source-link" href={sourceHref(blockId)}>Jump to source block</a>
									</div>
									{#each notes as note, idx (note.id)}
										<div class="note" class:assert={note.kind === 'assert'} class:debunk={note.kind === 'debunk'} class:strike={note.kind === 'strike'}>
											<div class="kind-row">
												<div class="kind">{note.kind}</div>
												<a class="ref-chip" href={sourceHref(blockId)}>{noteLabel(idx)}</a>
											</div>
											{#if note.quote}
												<p class="quote">“{note.quote}”</p>
											{/if}
											<p>{note.text}</p>
										</div>
									{/each}
								</div>
							{/each}
						{/if}
					</aside>
				</section>
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
		transition: width 0.2s ease, min-width 0.2s ease, flex-basis 0.2s ease, border-color 0.2s ease;
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

	.editor-main {
		flex: 1;
		min-width: 0;
		background: radial-gradient(circle at top, color-mix(in srgb, var(--note-accent, #7c5cff) var(--note-wash, 8%), var(--note-bg, #ffffff)) 0%, var(--note-bg, #ffffff) 48%);
		position: relative;
		overflow: hidden;
	}

	.editor-main.dark {
		background: var(--note-bg, #000000);
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

	.editor-wrapper {
		flex: 1;
		max-width: 1100px;
		margin: 0 auto;
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		padding: 0 48px 80px;
		background: transparent;
	}

	.head { margin: 28px 0 20px; }
	.head h1 { margin: 0; font-size: 46px; font-family: var(--font-display); letter-spacing: 0.01em; color: var(--note-title, #111827); }
	.head h1 {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 100%;
	}
	.head p { margin: 8px 0 0; color: var(--note-muted, #4b5563); }
	.summary { color: var(--note-text, #1f2328); font-weight: 500; }
	.grid { margin-top: 20px; display: grid; grid-template-columns: 1.55fr 1fr; gap: 16px; }
	.original, .notes { background: var(--note-surface, #fff); border: 2px solid var(--note-title, #1a1a1a); border-radius: 8px; padding: 18px; box-shadow: 6px 6px 0 var(--note-title, #1a1a1a); }
	.original h2, .notes h2 { color: var(--note-title, #111827); }
	.note-group { margin-bottom: 14px; }
	.block-label { font-size: 12px; font-weight: 700; color: var(--note-muted, #6b7280); margin-bottom: 8px; }
	.source-link { color: var(--note-title, #1a1a1a); text-decoration: none; font-weight: 800; font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; border: 2px solid var(--note-title, #1a1a1a); padding: 3px 8px; border-radius: 6px; }
	.note { border: 2px solid var(--note-title, #1a1a1a); background: var(--note-surface, #fff); border-radius: 8px; padding: 10px; margin-bottom: 8px; color: var(--note-text, #1f2328); }
	.kind-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 4px; }
	.note .kind { font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--note-muted, #6b7280); margin-bottom: 4px; }
	.ref-chip {
		display: inline-flex;
		align-items: center;
		padding: 3px 8px;
		font-size: 11px;
		font-weight: 800;
		border-radius: 6px;
		border: 2px solid var(--note-title, #1a1a1a);
		color: var(--note-title, #1a1a1a);
		text-decoration: none;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.note.assert { border-color: #166534; background: color-mix(in srgb, var(--note-surface, #fff) 85%, #22c55e 15%); }
	.note.debunk { border-color: #991b1b; background: color-mix(in srgb, var(--note-surface, #fff) 85%, #ef4444 15%); }
	.note.strike .quote { text-decoration: line-through; }
	.quote { margin: 0 0 6px; color: var(--note-muted, #4b5563); }


	.error { color: #b91c1c; }
	@media (max-width: 1000px) {
		.editor-shell { flex-direction: column; }
		.cover-rail { width: 100%; min-width: 0; height: 220px; }
		.cover-rail:hover,
		.cover-rail.has-cover { width: 100%; min-width: 0; flex: 0 0 auto; }
		.editor-wrapper { padding: 0 16px 64px; }
		.grid { grid-template-columns: 1fr; }
	}
</style>
