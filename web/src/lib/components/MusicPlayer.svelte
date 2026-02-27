<script lang="ts">
	import { onMount, onDestroy, tick as svelteTick, createEventDispatcher } from 'svelte';
	import { isLocalMediaRef, putLocalMediaBlob, resolveLocalMediaObjectURL } from '$lib/editor/localMedia';

	export let url: string = '';
	export let title: string = '';
	export let artist: string = '';
	export let coverUrl: string = '';
	export let readonly: boolean = false;
	export let apiUrl: string = 'http://localhost:8080';
	export let pageId: string = '';
	export let shareToken: string = '';
	export let allowLocalMedia: boolean = false;

	const dispatch = createEventDispatcher<{
		change: { url: string; title: string; artist: string; coverUrl: string };
	}>();

	// ── Setup / edit form state ────────────────────────────────────────────────
	let editing = !url;
	let draftUrl = url;
	let draftTitle = title || '';
	let draftArtist = artist || '';
	let draftCoverUrl = coverUrl || '';
	let audioFileInputEl: HTMLInputElement;
	let coverFileInputEl: HTMLInputElement;
	let uploading = false;
	let uploadError = '';
	let resolvedAudioUrl = '';
	let resolvedCoverUrl = '';
	let resolveAudioRun = 0;
	let resolveCoverRun = 0;

	// ── Playback state ─────────────────────────────────────────────────────────
	let audioEl: HTMLAudioElement;
	let waveCanvas: HTMLCanvasElement;
	let isPlaying = false;
	let currentTime = 0;
	let duration = 0;
	let volume = 1;
	let isMuted = false;
	let prevVolume = 1;

	// ── Waveform state ─────────────────────────────────────────────────────────
	let peaks: number[] = [];
	let peaksReady = false;
	let scrubbing = false;
	let animFrameId: number;
	let mounted = false;
	let lastWaveSource = '';

	$: void resolveAudioSource();
	$: void resolveCoverSource();
	$: {
		const waveSource = (resolvedAudioUrl || (isLocalMediaRef(url) ? '' : url) || '').trim();
		if (!editing && mounted && waveSource && waveSource !== lastWaveSource) {
			lastWaveSource = waveSource;
			void loadPeaks(waveSource);
		}
	}

	// ── Waveform helpers ───────────────────────────────────────────────────────
	function generateFallback(count: number): number[] {
		const result: number[] = [];
		let v = 0.5;
		for (let i = 0; i < count; i++) {
			v = Math.max(0.08, Math.min(1.0, v + (Math.random() - 0.5) * 0.38));
			result.push(v);
		}
		// Smooth twice
		for (let pass = 0; pass < 3; pass++) {
			for (let i = 1; i < result.length - 1; i++) {
				result[i] = (result[i - 1] * 0.25 + result[i] * 0.5 + result[i + 1] * 0.25);
			}
		}
		return result;
	}

	async function loadPeaks(audioUrl: string) {
		peaksReady = false;
		try {
			const res = await fetch(audioUrl, { mode: 'cors' });
			if (!res.ok) throw new Error('fetch failed');
			const buf = await res.arrayBuffer();
			const ctx = new AudioContext();
			const audio = await ctx.decodeAudioData(buf);
			await ctx.close();
			const channel = audio.getChannelData(0);
			const BAR_COUNT = 200;
			const segLen = Math.floor(channel.length / BAR_COUNT);
			const raw: number[] = [];
			for (let i = 0; i < BAR_COUNT; i++) {
				let max = 0;
				for (let j = 0; j < segLen; j++) {
					const abs = Math.abs(channel[i * segLen + j]);
					if (abs > max) max = abs;
				}
				raw.push(max);
			}
			const maxV = Math.max(...raw, 0.0001);
			peaks = raw.map((p) => p / maxV);
		} catch {
			peaks = generateFallback(200);
		}
		peaksReady = true;
		await svelteTick();
		drawWave();
	}

	function drawWave() {
		if (!waveCanvas || !peaksReady) return;
		const canvas = waveCanvas;
		const dpr = window.devicePixelRatio || 1;
		const W = canvas.clientWidth || canvas.offsetWidth || 600;
		const H = 88;
		canvas.width = Math.round(W * dpr);
		canvas.height = Math.round(H * dpr);
		const ctx = canvas.getContext('2d');
		if (!ctx) return;
		ctx.scale(dpr, dpr);
		ctx.clearRect(0, 0, W, H);

		const barCount = peaks.length;
		const barW = W / barCount;
		const gap = Math.max(0.8, barW * 0.22);
		const bw = Math.max(1, barW - gap);
		const midY = H * 0.54;
		const maxBarH = midY - 4;

		const progress = duration > 0 ? currentTime / duration : 0;
		const playedX = progress * W;

		const style = getComputedStyle(canvas);
		const accent = style.getPropertyValue('--note-text').trim() || '#1f2328';
		const unplayed = style.getPropertyValue('--note-border').trim() || '#d1d5db';

		// Parse accent hex to rgba for gradient
		const accentRgb = hexToRgb(accent) ?? [124, 92, 255];

		for (let i = 0; i < barCount; i++) {
			const x = i * barW + gap / 2;
			const peak = peaks[i];
			const bh = Math.max(2, peak * maxBarH);
			const center = x + bw / 2;
			const played = center < playedX;

			// ── Top bar ───────────────────────────────────────────────────────
			ctx.fillStyle = played ? accent : unplayed;
			drawRoundRect(ctx, x, midY - bh, bw, bh, Math.min(2, bw / 2));
			ctx.fill();

			// ── Reflection (faded mirror below) ──────────────────────────────
			const refH = bh * 0.42;
			if (played) {
				ctx.fillStyle = `rgba(${accentRgb[0]},${accentRgb[1]},${accentRgb[2]}, 0.28)`;
			} else {
				ctx.fillStyle = unplayed;
				ctx.globalAlpha = 0.28;
			}
			drawRoundRect(ctx, x, midY + 2, bw, Math.max(1, refH), Math.min(1, bw / 2));
			ctx.fill();
			ctx.globalAlpha = 1;
		}

		// Playhead
		if (progress > 0 && progress < 1) {
			ctx.fillStyle = `rgba(${accentRgb[0]},${accentRgb[1]},${accentRgb[2]}, 0.75)`;
			ctx.fillRect(playedX - 1, 0, 2, H);
			// Small dot on playhead
			ctx.beginPath();
			ctx.arc(playedX, midY - 4, 4, 0, Math.PI * 2);
			ctx.fillStyle = accent;
			ctx.fill();
		}
	}

	function drawRoundRect(
		ctx: CanvasRenderingContext2D,
		x: number,
		y: number,
		w: number,
		h: number,
		r: number
	) {
		ctx.beginPath();
		if (typeof (ctx as any).roundRect === 'function') {
			(ctx as any).roundRect(x, y, w, h, r);
		} else {
			const R = Math.min(r, w / 2, h / 2);
			ctx.moveTo(x + R, y);
			ctx.lineTo(x + w - R, y);
			ctx.quadraticCurveTo(x + w, y, x + w, y + R);
			ctx.lineTo(x + w, y + h - R);
			ctx.quadraticCurveTo(x + w, y + h, x + w - R, y + h);
			ctx.lineTo(x + R, y + h);
			ctx.quadraticCurveTo(x, y + h, x, y + h - R);
			ctx.lineTo(x, y + R);
			ctx.quadraticCurveTo(x, y, x + R, y);
			ctx.closePath();
		}
	}

	function hexToRgb(hex: string): [number, number, number] | null {
		const clean = hex.trim().replace('#', '');
		if (clean.length !== 6) return null;
		const r = parseInt(clean.slice(0, 2), 16);
		const g = parseInt(clean.slice(2, 4), 16);
		const b = parseInt(clean.slice(4, 6), 16);
		return [r, g, b];
	}

	// ── Animation loop ─────────────────────────────────────────────────────────
	function animLoop() {
		if (!audioEl) return;
		currentTime = audioEl.currentTime;
		drawWave();
		if (isPlaying) {
			animFrameId = requestAnimationFrame(animLoop);
		}
	}

	// ── Playback controls ──────────────────────────────────────────────────────
	function togglePlay() {
		if (!audioEl) return;
		if (isPlaying) {
			audioEl.pause();
		} else {
			audioEl.play().catch(() => {});
		}
	}

	function handlePlay() {
		isPlaying = true;
		animFrameId = requestAnimationFrame(animLoop);
	}

	function handlePause() {
		isPlaying = false;
		cancelAnimationFrame(animFrameId);
		drawWave();
	}

	function handleEnded() {
		isPlaying = false;
		cancelAnimationFrame(animFrameId);
		if (audioEl) audioEl.currentTime = 0;
		currentTime = 0;
		drawWave();
	}

	function handleLoadedMetadata() {
		if (audioEl) duration = audioEl.duration;
	}

	function handleTimeUpdate() {
		if (!scrubbing && audioEl) {
			currentTime = audioEl.currentTime;
		}
	}

	function toggleMute() {
		if (!audioEl) return;
		if (isMuted) {
			isMuted = false;
			volume = prevVolume || 1;
			audioEl.volume = volume;
		} else {
			prevVolume = volume;
			isMuted = true;
			audioEl.volume = 0;
		}
	}

	function handleVolumeInput(e: Event) {
		const val = parseFloat((e.target as HTMLInputElement).value);
		volume = val;
		isMuted = val === 0;
		if (audioEl) audioEl.volume = val;
	}

	// ── Waveform scrubbing ─────────────────────────────────────────────────────
	function xRatio(e: MouseEvent): number {
		const rect = waveCanvas.getBoundingClientRect();
		return Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
	}

	function handleWaveDown(e: MouseEvent) {
		scrubbing = true;
		seekTo(xRatio(e));
	}

	function handleWaveMove(e: MouseEvent) {
		if (!scrubbing) return;
		seekTo(xRatio(e));
	}

	function handleWaveUp() {
		scrubbing = false;
	}

	function seekTo(ratio: number) {
		if (!audioEl || !duration) return;
		audioEl.currentTime = ratio * duration;
		currentTime = audioEl.currentTime;
		drawWave();
	}

	// ── Time formatting ────────────────────────────────────────────────────────
	function fmt(s: number): string {
		if (!isFinite(s) || isNaN(s)) return '0:00';
		const m = Math.floor(s / 60);
		const sec = Math.floor(s % 60);
		return `${m}:${sec.toString().padStart(2, '0')}`;
	}

	// ── Setup form ─────────────────────────────────────────────────────────────
	function buildAudioEndpoint(): string {
		const shareQ = shareToken ? `?share=${encodeURIComponent(shareToken)}` : '';
		if (pageId) return `/v1/pages/${encodeURIComponent(pageId)}/media/audio${shareQ}`;
		return '/v1/media/audio';
	}

	async function resolveAudioSource() {
		const runId = ++resolveAudioRun;
		if (!url) {
			resolvedAudioUrl = '';
			return;
		}
		if (!isLocalMediaRef(url)) {
			resolvedAudioUrl = url;
			return;
		}
		const objectUrl = await resolveLocalMediaObjectURL(url);
		if (runId !== resolveAudioRun) return;
		resolvedAudioUrl = objectUrl || '';
	}

	async function resolveCoverSource() {
		const runId = ++resolveCoverRun;
		if (!coverUrl) {
			resolvedCoverUrl = '';
			return;
		}
		if (!isLocalMediaRef(coverUrl)) {
			resolvedCoverUrl = coverUrl;
			return;
		}
		const objectUrl = await resolveLocalMediaObjectURL(coverUrl);
		if (runId !== resolveCoverRun) return;
		resolvedCoverUrl = objectUrl || '';
	}

	async function handleAudioFilePick(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		uploadError = ''; uploading = true;
		try {
			if (allowLocalMedia && !pageId && !shareToken) {
				draftUrl = await putLocalMediaBlob(file);
			} else {
				const form = new FormData(); form.append('file', file);
				const res = await fetch(`${apiUrl}${buildAudioEndpoint()}`, { method: 'POST', credentials: 'include', body: form });
				if (!res.ok) throw new Error(((await res.json().catch(() => ({}))) as any).error || 'Upload failed');
				const payload = await res.json() as any;
				draftUrl = payload.url;
			}
			if (!draftTitle) draftTitle = file.name.replace(/\.[^.]+$/, '');
		} catch (err: any) { uploadError = err.message || 'Upload failed'; }
		finally { uploading = false; (e.target as HTMLInputElement).value = ''; }
	}

	async function handleCoverFilePick(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		uploadError = ''; uploading = true;
		try {
			if (allowLocalMedia && !pageId && !shareToken) {
				draftCoverUrl = await putLocalMediaBlob(file);
			} else {
				const form = new FormData(); form.append('file', file);
				const endpoint = pageId ? `${apiUrl}/v1/pages/${encodeURIComponent(pageId)}/media/images` : `${apiUrl}/v1/media/images`;
				const res = await fetch(endpoint, { method: 'POST', credentials: 'include', body: form });
				if (!res.ok) throw new Error('Cover upload failed');
				const payload = await res.json() as any;
				if (payload?.url) draftCoverUrl = payload.url;
			}
		} catch (err: any) { uploadError = err.message || 'Cover upload failed'; }
		finally { uploading = false; (e.target as HTMLInputElement).value = ''; }
	}

	function confirmSetup() {
		const trimmedUrl = draftUrl.trim();
		if (!trimmedUrl) return;
		url = trimmedUrl;
		title = draftTitle.trim() || 'Untitled Track';
		artist = draftArtist.trim();
		coverUrl = draftCoverUrl.trim();
		editing = false;
		dispatch('change', { url, title, artist, coverUrl });
	}

	function enterEdit() {
		draftUrl = url;
		draftTitle = title;
		draftArtist = artist;
		draftCoverUrl = coverUrl;
		editing = true;
	}

	function handleSetupKey(e: KeyboardEvent) {
		if (e.key === 'Enter') confirmSetup();
	}

	// ── Resize observer ────────────────────────────────────────────────────────
	let ro: ResizeObserver | null = null;

	function attachResize() {
		if (!waveCanvas || ro) return;
		ro = new ResizeObserver(() => {
			if (peaksReady) drawWave();
		});
		ro.observe(waveCanvas);
	}

	$: if (!editing && peaksReady && waveCanvas) {
		attachResize();
	}

	onMount(async () => {
		mounted = true;
		if (!editing && (resolvedAudioUrl || url)) {
			await loadPeaks(resolvedAudioUrl || url);
		}
	});

	onDestroy(() => {
		if (typeof cancelAnimationFrame !== 'undefined') cancelAnimationFrame(animFrameId);
		if (ro) ro.disconnect();
	});
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="music-player">
	{#if editing && !readonly}
		<!-- ── Setup / Edit Form ─────────────────────────────────────────────── -->
		<div class="setup-form">
			<div class="setup-icon">♫</div>
			<p class="setup-hint">Upload an MP3 or paste a direct audio URL</p>
			{#if allowLocalMedia && !pageId && !shareToken}
				<p class="setup-hint local-media-hint">Stored locally until publish</p>
			{/if}
			<div class="setup-fields">
				<div class="setup-upload-row">
					<button
						class="upload-btn"
						class:uploading
						on:click={() => audioFileInputEl?.click()}
						disabled={uploading}
						type="button"
					>{uploading ? 'Uploading…' : '↑ Upload MP3'}</button>
					<span class="setup-or">or</span>
					<input
						class="setup-input setup-url"
						type="url"
						placeholder="Paste audio URL…"
						bind:value={draftUrl}
						on:keydown={handleSetupKey}
						spellcheck="false"
						style="flex:1;min-width:120px"
					/>
					<input bind:this={audioFileInputEl} type="file"
						accept="audio/mp3,audio/mpeg,audio/ogg,audio/wav,audio/flac,audio/aac,.mp3,.ogg,.wav,.flac,.aac,.m4a"
						style="display:none" on:change={handleAudioFilePick} />
				</div>
				{#if uploadError}<p class="upload-error">{uploadError}</p>{/if}
				{#if draftUrl && !draftUrl.startsWith('blob:') && !isLocalMediaRef(draftUrl)}
					<p class="url-preview">{draftUrl}</p>
				{/if}
				<div class="setup-row">
					<input class="setup-input" type="text" placeholder="Track title" bind:value={draftTitle} on:keydown={handleSetupKey} />
					<input class="setup-input" type="text" placeholder="Artist (optional)" bind:value={draftArtist} on:keydown={handleSetupKey} />
				</div>
				<div class="setup-cover-row">
					<input class="setup-input" style="flex:1" type="url" placeholder="Cover image URL (optional)" bind:value={draftCoverUrl} on:keydown={handleSetupKey} spellcheck="false" />
					<button class="upload-btn upload-btn-sm" on:click={() => coverFileInputEl?.click()} type="button">↑ Cover</button>
					<input bind:this={coverFileInputEl} type="file" accept="image/*" style="display:none" on:change={handleCoverFilePick} />
				</div>
			</div>
			<div class="setup-actions">
				<button class="confirm-btn" on:click={confirmSetup} disabled={!draftUrl.trim() || uploading}>
					Add Track
				</button>
				{#if url}
					<button class="cancel-btn" on:click={() => (editing = false)}>Cancel</button>
				{/if}
			</div>
		</div>
	{:else if url}
		<!-- ── Player ────────────────────────────────────────────────────────── -->
		<!-- svelte-ignore a11y-media-has-caption -->
		<audio
			bind:this={audioEl}
			src={resolvedAudioUrl || (isLocalMediaRef(url) ? '' : url)}
			preload="metadata"
			on:play={handlePlay}
			on:pause={handlePause}
			on:ended={handleEnded}
			on:loadedmetadata={handleLoadedMetadata}
			on:timeupdate={handleTimeUpdate}
		></audio>

		<div class="player-inner">
			<!-- Album art + meta -->
			<div class="player-top">
				<div class="cover-art">
					{#if coverUrl}
						<img src={resolvedCoverUrl || (isLocalMediaRef(coverUrl) ? '' : coverUrl)} alt={title} class="cover-img" />
					{:else}
						<div class="cover-placeholder">
							<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
								<circle cx="12" cy="12" r="10"/>
								<circle cx="12" cy="12" r="3"/>
								<line x1="12" y1="2" x2="12" y2="4"/>
								<line x1="12" y1="20" x2="12" y2="22"/>
							</svg>
						</div>
					{/if}
					{#if isPlaying}
						<div class="play-shimmer" aria-hidden="true"></div>
					{/if}
				</div>

				<div class="track-meta">
					<div class="track-title">{title || 'Untitled Track'}</div>
					{#if artist}
						<div class="track-artist">{artist}</div>
					{/if}
				</div>

				{#if !readonly}
					<button class="edit-track-btn" on:click={enterEdit} title="Edit track">
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
							<path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
						</svg>
					</button>
				{/if}
			</div>

			<!-- Waveform -->
			<div class="waveform-wrap">
				{#if !peaksReady}
					<div class="wave-loading">
						<div class="wave-loading-bars">
							{#each Array(30) as _, i}
								<div class="wave-loading-bar" style="animation-delay: {i * 0.06}s; height: {20 + Math.sin(i * 0.8) * 18}px"></div>
							{/each}
						</div>
					</div>
				{/if}
				<!-- svelte-ignore a11y-no-static-element-interactions -->
				<canvas
					bind:this={waveCanvas}
					class="waveform-canvas"
					class:hidden={!peaksReady}
					on:mousedown={handleWaveDown}
					on:mousemove={handleWaveMove}
					on:mouseup={handleWaveUp}
					on:mouseleave={handleWaveUp}
					style="cursor: {scrubbing ? 'grabbing' : 'pointer'};"
				></canvas>
			</div>

			<!-- Controls -->
			<div class="controls">
				<!-- Play / Pause -->
				<button class="play-btn" class:playing={isPlaying} on:click={togglePlay} aria-label={isPlaying ? 'Pause' : 'Play'}>
					{#if isPlaying}
						<svg viewBox="0 0 24 24" fill="currentColor">
							<rect x="6" y="4" width="4" height="16" rx="1"/>
							<rect x="14" y="4" width="4" height="16" rx="1"/>
						</svg>
					{:else}
						<svg viewBox="0 0 24 24" fill="currentColor">
							<polygon points="5,3 19,12 5,21"/>
						</svg>
					{/if}
				</button>

				<!-- Time -->
				<div class="time-display">
					<span class="time-current">{fmt(currentTime)}</span>
					<span class="time-sep">/</span>
					<span class="time-total">{fmt(duration)}</span>
				</div>

				<!-- Spacer -->
				<div class="controls-spacer"></div>

				<!-- Volume -->
				<button class="mute-btn" on:click={toggleMute} aria-label={isMuted ? 'Unmute' : 'Mute'}>
					{#if isMuted || volume === 0}
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
							<line x1="23" y1="9" x2="17" y2="15"/>
							<line x1="17" y1="9" x2="23" y2="15"/>
						</svg>
					{:else if volume < 0.5}
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
							<path d="M15.54 8.46a5 5 0 010 7.07"/>
						</svg>
					{:else}
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/>
							<path d="M19.07 4.93a10 10 0 010 14.14"/>
							<path d="M15.54 8.46a5 5 0 010 7.07"/>
						</svg>
					{/if}
				</button>
				<input
					class="volume-slider"
					type="range"
					min="0"
					max="1"
					step="0.02"
					value={isMuted ? 0 : volume}
					on:input={handleVolumeInput}
					aria-label="Volume"
				/>
			</div>
		</div>
	{/if}
</div>

<svelte:window on:mouseup={handleWaveUp} on:mousemove={handleWaveMove} />

<style>
	.music-player {
		width: 100%;
		border-radius: 16px;
		overflow: hidden;
		background: var(--note-surface, #ffffff);
		border: 1.5px solid var(--note-border, #e5e7eb);
		font-family: inherit;
		box-shadow: 0 2px 16px rgba(15, 23, 42, 0.06);
	}

	/* ── Setup Form ──────────────────────────────────────────────────────────── */
	.setup-form {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		padding: 36px 28px;
	}

	.setup-icon {
		font-size: 2.4rem;
		opacity: 0.3;
		line-height: 1;
	}

	.setup-hint {
		margin: 0;
		font-size: 13px;
		color: var(--note-muted, #6b7280);
		text-align: center;
	}

	.local-media-hint {
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		opacity: 0.82;
	}

	.setup-upload-row {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}

	.setup-or {
		font-size: 11px;
		color: var(--note-muted, #6b7280);
		flex-shrink: 0;
	}

	.setup-cover-row {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.upload-btn {
		padding: 8px 16px;
		background: var(--note-text, #1f2328);
		color: var(--note-surface, #fff);
		border: none;
		border-radius: 10px;
		font-size: 12px;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		white-space: nowrap;
		transition: opacity 0.15s, transform 0.1s;
		flex-shrink: 0;
	}

	.upload-btn-sm { padding: 8px 12px; font-size: 11px; }

	.upload-btn:hover:not(:disabled) { opacity: 0.75; transform: translateY(-1px); }
	.upload-btn:active { transform: translateY(0); }
	.upload-btn:disabled, .upload-btn.uploading { opacity: 0.45; cursor: not-allowed; }

	.upload-error {
		margin: 0;
		font-size: 12px;
		color: #ef4444;
		padding: 6px 10px;
		border-radius: 8px;
		background: color-mix(in srgb, #ef4444 10%, transparent);
	}

	.url-preview {
		margin: 0;
		font-size: 11px;
		color: var(--note-muted, #6b7280);
		font-family: monospace;
		word-break: break-all;
		padding: 4px 8px;
		border: 1px dashed var(--note-border, #e5e7eb);
		border-radius: 6px;
	}

	.setup-fields {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.setup-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 10px;
	}

	.setup-input {
		width: 100%;
		padding: 9px 13px;
		border: 1.5px solid var(--note-border, #e5e7eb);
		border-radius: 10px;
		background: var(--note-bg, #f9fafb);
		color: var(--note-text, #1f2328);
		font-size: 13px;
		font-family: inherit;
		outline: none;
		box-sizing: border-box;
		transition: border-color 0.15s, box-shadow 0.15s;
	}

	.setup-input:focus {
		border-color: var(--note-text, #1f2328);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--note-text, #1f2328) 12%, transparent);
	}

	.setup-url {
		font-size: 12px;
	}

	.setup-actions {
		display: flex;
		gap: 10px;
	}

	.confirm-btn {
		padding: 9px 22px;
		background: var(--note-text, #1f2328);
		color: var(--note-surface, #fff);
		border: none;
		border-radius: 10px;
		font-size: 13px;
		font-weight: 700;
		font-family: inherit;
		cursor: pointer;
		transition: opacity 0.15s, transform 0.1s;
	}

	.confirm-btn:hover:not(:disabled) {
		opacity: 0.88;
		transform: translateY(-1px);
	}

	.confirm-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.cancel-btn {
		padding: 9px 18px;
		background: transparent;
		color: var(--note-muted, #6b7280);
		border: 1.5px solid var(--note-border, #e5e7eb);
		border-radius: 10px;
		font-size: 13px;
		font-family: inherit;
		cursor: pointer;
		transition: background 0.15s;
	}

	.cancel-btn:hover {
		background: var(--note-border, #e5e7eb);
	}

	/* ── Player ──────────────────────────────────────────────────────────────── */
	.player-inner {
		padding: 20px 22px 18px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.player-top {
		display: flex;
		align-items: center;
		gap: 14px;
	}

	/* Album art */
	.cover-art {
		position: relative;
		width: 52px;
		height: 52px;
		border-radius: 10px;
		overflow: hidden;
		flex-shrink: 0;
		background: color-mix(in srgb, var(--note-border, #e5e7eb) 60%, var(--note-surface, #fff));
	}

	.cover-img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}

	.cover-placeholder {
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--note-muted, #6b7280);
		opacity: 0.6;
	}

	.cover-placeholder svg {
		width: 26px;
		height: 26px;
	}

	@keyframes shimmer {
		0%, 100% { opacity: 0.3; }
		50% { opacity: 0.65; }
	}

	.play-shimmer {
		position: absolute;
		inset: 0;
		background: color-mix(in srgb, var(--note-text, #1f2328) 15%, transparent);
		animation: shimmer 1.8s ease-in-out infinite;
		border-radius: inherit;
	}

	/* Track meta */
	.track-meta {
		flex: 1;
		min-width: 0;
	}

	.track-title {
		font-size: 15px;
		font-weight: 700;
		color: var(--note-text, #1f2328);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		letter-spacing: -0.02em;
	}

	.track-artist {
		font-size: 12px;
		color: var(--note-muted, #6b7280);
		margin-top: 2px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.edit-track-btn {
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: 1.5px solid var(--note-border, #e5e7eb);
		border-radius: 8px;
		color: var(--note-muted, #6b7280);
		cursor: pointer;
		flex-shrink: 0;
		transition: background 0.15s, color 0.15s, border-color 0.15s;
	}

	.edit-track-btn:hover {
		background: color-mix(in srgb, var(--note-text, #1f2328) 8%, transparent);
		border-color: var(--note-text, #1f2328);
		color: var(--note-text, #1f2328);
	}

	.edit-track-btn svg {
		width: 14px;
		height: 14px;
	}

	/* Waveform */
	.waveform-wrap {
		width: 100%;
		height: 88px;
		position: relative;
		border-radius: 10px;
		overflow: hidden;
		background: color-mix(in srgb, var(--note-border, #e5e7eb) 40%, transparent);
	}

	.waveform-canvas {
		display: block;
		width: 100%;
		height: 88px;
		border-radius: 10px;
		user-select: none;
	}

	.waveform-canvas.hidden {
		visibility: hidden;
		position: absolute;
		inset: 0;
	}

	/* Loading skeleton bars */
	@keyframes wavePulse {
		0%, 100% { opacity: 0.25; transform: scaleY(1); }
		50% { opacity: 0.6; transform: scaleY(1.18); }
	}

	.wave-loading {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.wave-loading-bars {
		display: flex;
		align-items: center;
		gap: 3px;
		height: 56px;
	}

	.wave-loading-bar {
		width: 4px;
		background: var(--note-text, #1f2328);
		border-radius: 2px;
		animation: wavePulse 1.4s ease-in-out infinite;
		transform-origin: center bottom;
	}

	/* Controls */
	.controls {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.controls-spacer {
		flex: 1;
	}

	/* Play button */
	.play-btn {
		width: 42px;
		height: 42px;
		border-radius: 50%;
		border: none;
		background: var(--note-text, #1f2328);
		color: var(--note-surface, #fff);
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		flex-shrink: 0;
		transition: transform 0.12s, opacity 0.15s;
		box-shadow: 0 4px 14px color-mix(in srgb, var(--note-text, #1f2328) 25%, transparent);
	}

	.play-btn:hover {
		transform: scale(1.08);
		opacity: 0.85;
	}

	.play-btn:active {
		transform: scale(0.97);
	}

	.play-btn svg {
		width: 18px;
		height: 18px;
	}

	.play-btn .pause-icon {
		width: 16px;
		height: 16px;
	}

	/* Time display */
	.time-display {
		display: flex;
		align-items: center;
		gap: 4px;
		font-size: 12px;
		font-variant-numeric: tabular-nums;
		letter-spacing: 0;
	}

	.time-current {
		color: var(--note-text, #1f2328);
		font-weight: 600;
	}

	.time-sep {
		color: var(--note-muted, #6b7280);
	}

	.time-total {
		color: var(--note-muted, #6b7280);
	}

	/* Mute button */
	.mute-btn {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		color: var(--note-muted, #9ca3af);
		cursor: pointer;
		border-radius: 6px;
		flex-shrink: 0;
		transition: color 0.15s, background 0.15s;
	}

	.mute-btn:hover {
		color: var(--note-text, #1f2328);
		background: color-mix(in srgb, var(--note-border, #e5e7eb) 60%, transparent);
	}

	.mute-btn svg {
		width: 16px;
		height: 16px;
	}

	/* Volume slider */
	.volume-slider {
		-webkit-appearance: none;
		appearance: none;
		width: 80px;
		height: 4px;
		border-radius: 4px;
		background: linear-gradient(
			to right,
			var(--note-text, #1f2328) 0%,
			var(--note-text, #1f2328) calc(var(--volume-pct, 100%) * 1%),
			var(--note-border, #d1d5db) calc(var(--volume-pct, 100%) * 1%),
			var(--note-border, #d1d5db) 100%
		);
		outline: none;
		cursor: pointer;
		flex-shrink: 0;
	}

	.volume-slider::-webkit-slider-thumb {
		-webkit-appearance: none;
		appearance: none;
		width: 13px;
		height: 13px;
		border-radius: 50%;
		background: var(--note-text, #1f2328);
		cursor: pointer;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
		transition: transform 0.12s;
	}

	.volume-slider::-webkit-slider-thumb:hover {
		transform: scale(1.25);
	}

	.volume-slider::-moz-range-thumb {
		width: 13px;
		height: 13px;
		border-radius: 50%;
		border: none;
		background: var(--note-text, #1f2328);
		cursor: pointer;
	}
</style>
