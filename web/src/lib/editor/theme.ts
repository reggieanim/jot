import type { Rgb } from './types';

export const DEFAULT_THEME =
	'--note-bg:#ffffff;--note-surface:#ffffff;--note-text:#1f2328;--note-muted:#6b7280;--note-title:#111827;--note-accent:#7c5cff;--note-border:#e5e7eb;--note-quote:#374151;--note-title-glow:transparent;--note-heading-bg:#111827;--note-heading-text:#f9fafb;--note-heading-border:#1f2937;--note-wash:0%;--note-grain-opacity:0;--note-fade:0;';

function clamp(value: number, min = 0, max = 255) {
	return Math.max(min, Math.min(max, value));
}

function luminance(r: number, g: number, b: number) {
	return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
}

function toHex(r: number, g: number, b: number) {
	return `#${[r, g, b].map((v) => clamp(v).toString(16).padStart(2, '0')).join('')}`;
}

function mix(colorA: Rgb, colorB: Rgb, ratio: number): Rgb {
	const inv = 1 - ratio;
	return [
		Math.round(colorA[0] * inv + colorB[0] * ratio),
		Math.round(colorA[1] * inv + colorB[1] * ratio),
		Math.round(colorA[2] * inv + colorB[2] * ratio)
	];
}

export function buildThemeStyle(base: Rgb, accent: Rgb, options: { darkMode: boolean; cinematicEnabled: boolean; moodStrength: number }) {
	const { darkMode, cinematicEnabled, moodStrength } = options;
	const mood = clamp(moodStrength, 0, 100) / 100;

	/* ── helper: desaturate an rgb color toward its luminance ── */
	function desat(c: Rgb, amount: number): Rgb {
		const l = Math.round(0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]);
		return mix(c, [l, l, l], amount);
	}

	/* ── helper: warm-shift an rgb toward a temperature (positive = warm, negative = cool) ── */
	function temperature(c: Rgb, warmth: number): Rgb {
		return [
			clamp(c[0] + Math.round(warmth * 8)),
			clamp(c[1] + Math.round(warmth * 2)),
			clamp(c[2] - Math.round(warmth * 6))
		];
	}

	if (darkMode) {
		/* ── dark + cinematic: cover colors tint the dark surfaces ── */
		const tintStrength = cinematicEnabled ? 0.12 + mood * 0.14 : 0;
		const tintedBlack: Rgb = cinematicEnabled
			? mix([0, 0, 0], desat(base, 0.7), tintStrength)
			: [0, 0, 0];

		const darkAccent = mix(accent, [120, 132, 168], cinematicEnabled ? 0.25 - mood * 0.08 : 0.4);
		const darkAccentMuted = mix(darkAccent, [80, 88, 110], 0.34 + mood * 0.18);
		const darkBg = tintedBlack;
		const darkSurface = mix(tintedBlack, [12, 14, 18], 0.4 + mood * 0.2);
		const darkText: Rgb = cinematicEnabled
			? temperature([245, 242, 238], mood * 2)
			: [255, 255, 255];
		const darkMuted = cinematicEnabled
			? mix(desat(accent, 0.6), [154, 163, 180], 0.6 - mood * 0.1)
			: mix([154, 163, 180], [0, 0, 0], 0.3 + mood * 0.12);
		const darkTitle: Rgb = cinematicEnabled
			? temperature([255, 252, 248], mood * 1.5)
			: [255, 255, 255];
		const darkBorder = cinematicEnabled
			? mix(desat(base, 0.5), [30, 32, 38], 0.7 - mood * 0.1)
			: mix([24, 28, 34], [80, 88, 110], 0.2 + mood * 0.08);
		const darkQuote = mix(darkAccentMuted, [202, 208, 224], 0.24 + mood * 0.08);
		const darkGlow = mix(darkAccentMuted, [174, 183, 208], 0.34 + mood * 0.12);

		return [
			`--note-bg:${toHex(darkBg[0], darkBg[1], darkBg[2])}`,
			`--note-surface:${toHex(darkSurface[0], darkSurface[1], darkSurface[2])}`,
			`--note-text:${toHex(darkText[0], darkText[1], darkText[2])}`,
			`--note-muted:${toHex(darkMuted[0], darkMuted[1], darkMuted[2])}`,
			`--note-title:${toHex(darkTitle[0], darkTitle[1], darkTitle[2])}`,
			`--note-accent:${toHex(darkAccentMuted[0], darkAccentMuted[1], darkAccentMuted[2])}`,
			`--note-border:${toHex(darkBorder[0], darkBorder[1], darkBorder[2])}`,
			`--note-quote:${toHex(darkQuote[0], darkQuote[1], darkQuote[2])}`,
			`--note-title-glow:${toHex(darkGlow[0], darkGlow[1], darkGlow[2])}`,
			`--note-heading-bg:${toHex(darkSurface[0], darkSurface[1], darkSurface[2])}`,
			`--note-heading-text:${toHex(darkTitle[0], darkTitle[1], darkTitle[2])}`,
			`--note-heading-border:${toHex(darkBorder[0], darkBorder[1], darkBorder[2])}`,
			'--note-rail-border:transparent',
			`--note-wash:${cinematicEnabled ? Math.round(4 + mood * 12) : 0}%`,
			`--note-grain-opacity:${cinematicEnabled ? (0.015 + mood * 0.035).toFixed(3) : '0'}`,
			`--note-fade:${cinematicEnabled ? (0.03 + mood * 0.10).toFixed(3) : '0'}`
		].join(';') + ';';
	}

	if (!cinematicEnabled) {
		return [
			'--note-bg:#ffffff',
			'--note-surface:#ffffff',
			'--note-text:#1f2328',
			'--note-muted:#6b7280',
			'--note-title:#111827',
			`--note-accent:${toHex(accent[0], accent[1], accent[2])}`,
			'--note-border:#e5e7eb',
			'--note-quote:#374151',
			'--note-title-glow:transparent',
			'--note-heading-bg:#111827',
			'--note-heading-text:#f9fafb',
			'--note-heading-border:#1f2937',
			'--note-rail-border:#f1f3f5',
			'--note-wash:0%',
			'--note-grain-opacity:0',
			'--note-fade:0'
		].join(';') + ';';
	}

	/* ── cinematic light mode: cover image colors grade the whole page ── */

	/* tint = subtle cover color, getting stronger with mood */
	const tint = desat(base, 0.55 - mood * 0.18);                        /* keep some chroma from cover */
	const accentTint = desat(accent, 0.35 - mood * 0.12);                /* accent stays more saturated */
	const warm = luminance(base[0], base[1], base[2]) > 0.5 ? 1 : -0.5;  /* auto warm/cool shift */

	/* backgrounds: white base tinted very subtly by cover color */
	const bg = temperature(mix(tint, [252, 251, 249], 0.92 - mood * 0.10), warm * mood);
	const surface = temperature(mix(tint, [255, 254, 252], 0.94 - mood * 0.08), warm * mood * 0.5);

	/* text: dark base with a whisper of the cover hue */
	const titleTone = mix(desat(tint, 0.8), [32, 30, 28], 0.82 - mood * 0.06);
	const textTone = mix(desat(tint, 0.75), [52, 50, 48], 0.78 - mood * 0.06);
	const mutedTone = mix(desat(tint, 0.6), [128, 124, 120], 0.68 - mood * 0.08);

	/* borders + accents: cover colors come through most here */
	const borderTone = mix(desat(tint, 0.4), [218, 215, 210], 0.78 - mood * 0.12);
	const accentColor = mix(accentTint, [140, 130, 120], 0.30 + mood * 0.08);
	const quoteTone = mix(accentTint, [80, 78, 84], 0.50 - mood * 0.08);
	const glowTone = mix(accentTint, [248, 246, 242], 0.75 - mood * 0.14);

	/* headings: accent-tinted dark blocks */
	const headingBgTone = mix(desat(accentTint, 0.3), [24, 26, 30], 0.65 - mood * 0.10);
	const headingTextTone = temperature(mix(tint, [246, 243, 238], 0.88), warm * 0.6);
	const headingBorderTone = mix(headingBgTone, [255, 255, 255], 0.14 + mood * 0.04);

	return [
		`--note-bg:${toHex(bg[0], bg[1], bg[2])}`,
		`--note-surface:${toHex(surface[0], surface[1], surface[2])}`,
		`--note-text:${toHex(textTone[0], textTone[1], textTone[2])}`,
		`--note-muted:${toHex(mutedTone[0], mutedTone[1], mutedTone[2])}`,
		`--note-title:${toHex(titleTone[0], titleTone[1], titleTone[2])}`,
		`--note-accent:${toHex(accentColor[0], accentColor[1], accentColor[2])}`,
		`--note-border:${toHex(borderTone[0], borderTone[1], borderTone[2])}`,
		`--note-quote:${toHex(quoteTone[0], quoteTone[1], quoteTone[2])}`,
		`--note-title-glow:${toHex(glowTone[0], glowTone[1], glowTone[2])}`,
		`--note-heading-bg:${toHex(headingBgTone[0], headingBgTone[1], headingBgTone[2])}`,
		`--note-heading-text:${toHex(headingTextTone[0], headingTextTone[1], headingTextTone[2])}`,
		`--note-heading-border:${toHex(headingBorderTone[0], headingBorderTone[1], headingBorderTone[2])}`,
		`--note-rail-border:${toHex(borderTone[0], borderTone[1], borderTone[2])}`,
		`--note-wash:${Math.round(4 + mood * 14)}%`,
		`--note-grain-opacity:${(0.02 + mood * 0.05).toFixed(3)}`,
		`--note-fade:${(0.06 + mood * 0.16).toFixed(3)}`
	].join(';') + ';';
}

export async function extractPaletteFromImage(imageSrc: string | null, fallbackBase: Rgb, fallbackAccent: Rgb) {
	if (!imageSrc) {
		return { base: fallbackBase, accent: fallbackAccent };
	}

	try {
		const image = new Image();
		image.crossOrigin = 'anonymous';
		image.src = imageSrc;

		await new Promise<void>((resolve, reject) => {
			image.onload = () => resolve();
			image.onerror = () => reject(new Error('Image load failed'));
		});

		const canvas = document.createElement('canvas');
		const context = canvas.getContext('2d', { willReadFrequently: true });
		if (!context) {
			return { base: fallbackBase, accent: fallbackAccent };
		}

		const width = 64;
		const height = 64;
		canvas.width = width;
		canvas.height = height;
		context.drawImage(image, 0, 0, width, height);

		const { data } = context.getImageData(0, 0, width, height);

		/* ── collect all opaque pixels ── */
		const pixels: Rgb[] = [];
		for (let i = 0; i < data.length; i += 4) {
			if (data[i + 3] < 180) continue;
			pixels.push([data[i], data[i + 1], data[i + 2]]);
		}

		if (pixels.length === 0) {
			return { base: fallbackBase, accent: fallbackAccent };
		}

		/* ── simple 3-means clustering for dominant + accent colors ── */
		const k = Math.min(3, pixels.length);
		let centroids: Rgb[] = [];
		const step = Math.floor(pixels.length / k);
		for (let i = 0; i < k; i++) centroids.push([...pixels[i * step]] as Rgb);

		for (let iter = 0; iter < 8; iter++) {
			const sums: [number, number, number][] = centroids.map(() => [0, 0, 0]);
			const counts = centroids.map(() => 0);

			for (const px of pixels) {
				let bestDist = Infinity;
				let bestIdx = 0;
				for (let c = 0; c < centroids.length; c++) {
					const dr = px[0] - centroids[c][0];
					const dg = px[1] - centroids[c][1];
					const db = px[2] - centroids[c][2];
					const dist = dr * dr + dg * dg + db * db;
					if (dist < bestDist) { bestDist = dist; bestIdx = c; }
				}
				sums[bestIdx][0] += px[0];
				sums[bestIdx][1] += px[1];
				sums[bestIdx][2] += px[2];
				counts[bestIdx]++;
			}

			centroids = centroids.map((old, i) =>
				counts[i] > 0
					? [Math.round(sums[i][0] / counts[i]), Math.round(sums[i][1] / counts[i]), Math.round(sums[i][2] / counts[i])]
					: old
			);
		}

		/* ── pick base = largest cluster, accent = most saturated cluster ── */
		const clusterSizes: number[] = centroids.map(() => 0);
		for (const px of pixels) {
			let bestDist = Infinity;
			let bestIdx = 0;
			for (let c = 0; c < centroids.length; c++) {
				const dr = px[0] - centroids[c][0];
				const dg = px[1] - centroids[c][1];
				const db = px[2] - centroids[c][2];
				const dist = dr * dr + dg * dg + db * db;
				if (dist < bestDist) { bestDist = dist; bestIdx = c; }
			}
			clusterSizes[bestIdx]++;
		}

		/* base: cluster with most pixels (dominant tone) */
		let baseIdx = 0;
		for (let i = 1; i < centroids.length; i++) {
			if (clusterSizes[i] > clusterSizes[baseIdx]) baseIdx = i;
		}

		/* accent: cluster with highest saturation (skip base cluster) */
		let accentIdx = -1;
		let bestSat = -1;
		for (let i = 0; i < centroids.length; i++) {
			const [r, g, b] = centroids[i];
			const maxC = Math.max(r, g, b);
			const minC = Math.min(r, g, b);
			const sat = maxC === 0 ? 0 : (maxC - minC) / maxC;
			const lum = luminance(r, g, b);
			const score = sat * 0.7 + (1 - Math.abs(lum - 0.45)) * 0.3;
			if (score > bestSat) { bestSat = score; accentIdx = i; }
		}
		if (accentIdx === baseIdx && centroids.length > 1) {
			/* if accent == base, pick next best */
			accentIdx = baseIdx === 0 ? 1 : 0;
		}

		return {
			base: centroids[baseIdx],
			accent: accentIdx >= 0 ? centroids[accentIdx] : centroids[baseIdx]
		};
	} catch {
		return { base: fallbackBase, accent: fallbackAccent };
	}
}
