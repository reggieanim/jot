<script lang="ts">
	import { onDestroy, tick as svelteTick } from 'svelte';

	export let url: string = '';
	export let title: string = '';
	export let artist: string = '';
	export let coverUrl: string = '';

	let audioEl: HTMLAudioElement;
	let waveCanvas: HTMLCanvasElement;

	// Lazy load – src is only set on first play
	let audioReady = false;
	let isPlaying = false;
	let currentTime = 0;
	let duration = 0;

	let peaks: number[] = [];
	let peaksReady = false;
	let peaksLoading = false;

	let scrubbing = false;
	let animFrameId: number;

	// ── Waveform ───────────────────────────────────────────────────────────────
	function generateFallback(count: number): number[] {
		const result: number[] = [];
		let v = 0.5;
		for (let i = 0; i < count; i++) {
			v = Math.max(0.08, Math.min(1.0, v + (Math.random() - 0.5) * 0.38));
			result.push(v);
		}
		for (let pass = 0; pass < 3; pass++) {
			for (let i = 1; i < result.length - 1; i++) {
				result[i] = result[i - 1] * 0.25 + result[i] * 0.5 + result[i + 1] * 0.25;
			}
		}
		return result;
	}

	async function loadPeaks(audioUrl: string) {
		peaksLoading = true;
		peaksReady = false;
		try {
			const res = await fetch(audioUrl, { mode: 'cors' });
			if (!res.ok) throw new Error('fetch failed');
			const buf = await res.arrayBuffer();
			const actx = new AudioContext();
			const decoded = await actx.decodeAudioData(buf);
			await actx.close();
			const channel = decoded.getChannelData(0);
			const BAR_COUNT = 80;
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
			peaks = generateFallback(80);
		}
		peaksReady = true;
		peaksLoading = false;
		await svelteTick();
		drawWave();
	}

	function drawWave() {
		if (!waveCanvas || peaks.length === 0) return;
		const dpr = window.devicePixelRatio || 1;
		const W = waveCanvas.clientWidth || 200;
		const H = 40;
		waveCanvas.width = Math.round(W * dpr);
		waveCanvas.height = Math.round(H * dpr);
		const ctx = waveCanvas.getContext('2d');
		if (!ctx) return;
		ctx.scale(dpr, dpr);
		ctx.clearRect(0, 0, W, H);

		const barCount = peaks.length;
		const barW = W / barCount;
		const gap = Math.max(0.5, barW * 0.22);
		const bw = Math.max(1, barW - gap);
		const midY = H * 0.58;
		const maxBarH = midY - 2;
		const progress = duration > 0 ? currentTime / duration : 0;
		const playedX = progress * W;

		for (let i = 0; i < barCount; i++) {
			const x = i * barW + gap / 2;
			const bh = Math.max(1, peaks[i] * maxBarH);
			const played = x + bw / 2 < playedX;
			ctx.fillStyle = played ? '#ffffff' : 'rgba(255,255,255,0.22)';
			ctx.fillRect(x, midY - bh, bw, bh);
			ctx.fillStyle = played ? 'rgba(255,255,255,0.14)' : 'rgba(255,255,255,0.06)';
			ctx.fillRect(x, midY + 1, bw, Math.max(1, bh * 0.35));
		}

		if (progress > 0 && progress < 1) {
			ctx.fillStyle = 'rgba(255,255,255,0.55)';
			ctx.fillRect(playedX - 0.75, 0, 1.5, H);
		}
	}

	// ── Animation loop ─────────────────────────────────────────────────────────
	function animLoop() {
		if (!audioEl) return;
		currentTime = audioEl.currentTime;
		drawWave();
		if (isPlaying) animFrameId = requestAnimationFrame(animLoop);
	}

	// ── Controls ───────────────────────────────────────────────────────────────
	async function togglePlay(e: MouseEvent) {
		e.preventDefault();
		e.stopPropagation();
		if (!audioEl) return;

		if (!audioReady) {
			// First play: lazy-load source + start peak decoding
			audioEl.src = url;
			audioEl.load();
			audioReady = true;
			loadPeaks(url); // fire-and-forget
		}

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
		if (!scrubbing && audioEl) currentTime = audioEl.currentTime;
	}

	// ── Scrubbing ──────────────────────────────────────────────────────────────
	function xRatio(e: MouseEvent): number {
		const rect = waveCanvas.getBoundingClientRect();
		return Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
	}

	function handleWaveDown(e: MouseEvent) {
		e.stopPropagation();
		if (!peaksReady || !audioReady) return;
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

	function fmt(s: number): string {
		if (!isFinite(s) || isNaN(s) || s === 0) return '0:00';
		return `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}`;
	}

	onDestroy(() => {
		if (typeof cancelAnimationFrame !== 'undefined') cancelAnimationFrame(animFrameId);
	});
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="mini-player" on:click|stopPropagation>
	<!-- svelte-ignore a11y-media-has-caption -->
	<audio
		bind:this={audioEl}
		preload="none"
		on:play={handlePlay}
		on:pause={handlePause}
		on:ended={handleEnded}
		on:loadedmetadata={handleLoadedMetadata}
		on:timeupdate={handleTimeUpdate}
	></audio>

	<div class="mp-inner">
		<!-- Cover + meta -->
		<div class="mp-top">
			{#if coverUrl}
				<img class="mp-cover" src={coverUrl} alt={title || 'Track'} />
			{:else}
				<div class="mp-cover-ph">
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
						<circle cx="12" cy="12" r="10"/>
						<circle cx="12" cy="12" r="3"/>
					</svg>
				</div>
			{/if}
			<div class="mp-meta">
				<div class="mp-title">{title || 'Untitled Track'}</div>
				{#if artist}<div class="mp-artist">{artist}</div>{/if}
			</div>
		</div>

		<!-- Waveform -->
		<div class="mp-wave-wrap">
			{#if peaksLoading}
				<div class="mp-skel" aria-hidden="true">
					{#each Array(22) as _, i}
						<div class="mp-skel-bar" style="animation-delay:{i * 0.065}s; height:{10 + Math.sin(i * 0.85) * 9}px"></div>
					{/each}
				</div>
			{:else if !peaksReady}
				<div class="mp-wave-hint" aria-hidden="true">▶ play to load</div>
			{/if}
			<!-- svelte-ignore a11y-no-static-element-interactions -->
			<canvas
				bind:this={waveCanvas}
				class="mp-canvas"
				class:invisible={!peaksReady}
				on:mousedown={handleWaveDown}
				on:mousemove={handleWaveMove}
				on:mouseup={handleWaveUp}
				on:mouseleave={handleWaveUp}
				style="cursor:{scrubbing ? 'grabbing' : peaksReady ? 'pointer' : 'default'}"
			></canvas>
		</div>

		<!-- Controls -->
		<div class="mp-controls">
			<button
				class="mp-play"
				class:playing={isPlaying}
				on:click={togglePlay}
				aria-label={isPlaying ? 'Pause' : 'Play'}
				type="button"
			>
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
			<div class="mp-time">
				<span class="mp-cur">{fmt(currentTime)}</span>
				<span class="mp-sep">/</span>
				<span class="mp-tot">{fmt(duration)}</span>
			</div>
		</div>
	</div>
</div>

<svelte:window on:mouseup={handleWaveUp} on:mousemove={handleWaveMove} />

<style>
	.mini-player {
		position: absolute;
		inset: 0;
		background: #111111;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		color: #fff;
		font-family: inherit;
	}

	.mp-inner {
		flex: 1;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		padding: 13px 13px 11px;
		gap: 8px;
		min-height: 0;
	}

	/* ── Top row ─────────────────────────────────────────────────────────────── */
	.mp-top {
		display: flex;
		align-items: center;
		gap: 9px;
		min-width: 0;
	}

	.mp-cover {
		width: 34px;
		height: 34px;
		border-radius: 5px;
		object-fit: cover;
		flex-shrink: 0;
	}

	.mp-cover-ph {
		width: 34px;
		height: 34px;
		border-radius: 5px;
		background: rgba(255, 255, 255, 0.08);
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		color: rgba(255, 255, 255, 0.3);
	}

	.mp-cover-ph svg {
		width: 16px;
		height: 16px;
	}

	.mp-meta {
		flex: 1;
		min-width: 0;
	}

	.mp-title {
		font-size: 11.5px;
		font-weight: 700;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		letter-spacing: -0.01em;
		line-height: 1.3;
	}

	.mp-artist {
		font-size: 10px;
		color: rgba(255, 255, 255, 0.45);
		margin-top: 1px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* ── Waveform ────────────────────────────────────────────────────────────── */
	.mp-wave-wrap {
		position: relative;
		width: 100%;
		height: 40px;
		border-radius: 5px;
		overflow: hidden;
		background: rgba(255, 255, 255, 0.04);
		flex-shrink: 0;
	}

	.mp-canvas {
		display: block;
		width: 100%;
		height: 40px;
		user-select: none;
	}

	.mp-canvas.invisible {
		opacity: 0;
		pointer-events: none;
	}

	@keyframes skelPulse {
		0%, 100% { opacity: 0.18; transform: scaleY(1); }
		50%       { opacity: 0.48; transform: scaleY(1.14); }
	}

	.mp-skel {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 2.5px;
	}

	.mp-skel-bar {
		width: 3px;
		background: #fff;
		border-radius: 2px;
		animation: skelPulse 1.3s ease-in-out infinite;
		transform-origin: center;
	}

	.mp-wave-hint {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 9px;
		color: rgba(255, 255, 255, 0.2);
		letter-spacing: 0.06em;
		user-select: none;
	}

	/* ── Controls ────────────────────────────────────────────────────────────── */
	.mp-controls {
		display: flex;
		align-items: center;
		gap: 9px;
	}

	.mp-play {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		border: none;
		background: #fff;
		color: #111;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		flex-shrink: 0;
		transition: transform 0.1s, opacity 0.12s;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
	}

	.mp-play:hover {
		transform: scale(1.1);
	}

	.mp-play:active {
		transform: scale(0.94);
	}

	.mp-play svg {
		width: 12px;
		height: 12px;
	}

	.mp-time {
		font-size: 10px;
		font-variant-numeric: tabular-nums;
	}

	.mp-cur {
		color: rgba(255, 255, 255, 0.8);
		font-weight: 600;
	}

	.mp-sep {
		color: rgba(255, 255, 255, 0.25);
		margin: 0 2px;
	}

	.mp-tot {
		color: rgba(255, 255, 255, 0.35);
	}
</style>
