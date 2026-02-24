<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { env } from '$env/dynamic/public';
	import { onDestroy, onMount } from 'svelte';
	import Block from '$lib/components/Block.svelte';
	import Cover from '$lib/components/Cover.svelte';
	import { getBlockAsGalleryItems, normalizeGalleryItems, toGalleryData } from '$lib/editor/blocks';
	import { buildThemeStyle, DEFAULT_THEME, extractPaletteFromImage } from '$lib/editor/theme';
	import type { ApiBlock, ApiPage, Rgb } from '$lib/editor/types';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let pageId = '';
	let title = 'Untitled';
	let titleEl: HTMLDivElement;
	let cover: string | null = null;
	let isPublished = false;
	let blocks: ApiBlock[] = [{ id: generateId(), type: 'paragraph', position: 0, data: { text: '' } }];
	let status = '';
	let controlsOpen = false;
	let draggedBlockId: string | null = null;
	let cinematicEnabled = false;
	let darkMode = false;
	let bgColor = '';
	let moodStrength = 65;
	const FALLBACK_BASE: Rgb = [205, 207, 214];
	const FALLBACK_ACCENT: Rgb = [124, 92, 255];
	let paletteBase: Rgb = FALLBACK_BASE;
	let paletteAccent: Rgb = FALLBACK_ACCENT;
	let themeStyle = DEFAULT_THEME;
	let tocVersion = 0;
	let tocBlocks: ApiBlock[] = [];
	let tocTimer: ReturnType<typeof setTimeout> | null = null;
	let pageRevision = '';
	let syncTimer: ReturnType<typeof setTimeout> | null = null;
	let syncRetryTimer: ReturnType<typeof setTimeout> | null = null;
	let syncInFlight = false;
	let syncRetryCount = 0;
	let hasUnsyncedChanges = false;
	let metaSyncTimer: ReturnType<typeof setTimeout> | null = null;
	let metaSyncInFlight = false;
	let hasUnsyncedMeta = false;
	let pendingSyncAfterInFlight = false;
	let creatingPagePromise: Promise<boolean> | null = null;
	let loadedRoutePageId = '';
	let liveEventsSource: EventSource | null = null;
	let liveEventsPageId = '';
	let liveReconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let liveRetryCount = 0;
	let liveConnectionState: 'idle' | 'connecting' | 'live' | 'reconnecting' = 'idle';
	let viewerSessionId = '';
	let viewerName = '';
	let typingLocks: Record<string, { sessionId: string; userName: string; expiresAt: number }> = {};
	let activeUsers: Record<string, { sessionId: string; userName: string; lastSeen: number }> = {};
	let visibleUsers: Array<{ sessionId: string; userName: string; lastSeen: number }> = [];
	const typingLockTimers: Record<string, ReturnType<typeof setTimeout>> = {};
	const typingHeartbeat: Record<string, number> = {};
	let presenceHeartbeatTimer: ReturnType<typeof setInterval> | null = null;
	const SYNC_DEBOUNCE_MS = 320;
	const TYPING_HEARTBEAT_MS = 1200;
	const TYPING_LOCK_TTL_MS = 3500;
	const PRESENCE_HEARTBEAT_MS = 5000;
	const PRESENCE_TTL_MS = 15000;
	$: visibleUsers = Object.values(activeUsers)
		.filter((user) => Date.now() - user.lastSeen <= PRESENCE_TTL_MS)
		.sort((a, b) => (a.sessionId === viewerSessionId ? -1 : b.sessionId === viewerSessionId ? 1 : a.userName.localeCompare(b.userName)));

	function generateClientId() {
		if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
			return crypto.randomUUID();
		}
		return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	}

	function setupViewerIdentity() {
		if (typeof window === 'undefined') return;
		const sessionKey = 'jot.viewer.session';
		const nameKey = 'jot.viewer.name';

		const existingSession = window.localStorage.getItem(sessionKey)?.trim();
		viewerSessionId = existingSession || generateClientId();
		window.localStorage.setItem(sessionKey, viewerSessionId);

		const existingName = window.localStorage.getItem(nameKey)?.trim();
		viewerName = existingName || `User-${viewerSessionId.slice(0, 6)}`;
		window.localStorage.setItem(nameKey, viewerName);
	}

	function clearTypingTimer(blockId: string) {
		const timer = typingLockTimers[blockId];
		if (!timer) return;
		clearTimeout(timer);
		delete typingLockTimers[blockId];
	}

	function scheduleTypingExpiry(blockId: string, expiresAt: number) {
		clearTypingTimer(blockId);
		const delay = Math.max(0, expiresAt - Date.now());
		typingLockTimers[blockId] = setTimeout(() => {
			const lock = typingLocks[blockId];
			if (!lock || lock.expiresAt > Date.now()) return;
			const next = { ...typingLocks };
			delete next[blockId];
			typingLocks = next;
			delete typingLockTimers[blockId];
		}, delay + 20);
	}

	async function publishTyping(blockId: string, isTyping: boolean) {
		if (!pageId || !blockId || !viewerSessionId || !viewerName) return;

		if (isTyping) {
			const now = Date.now();
			const last = typingHeartbeat[blockId] || 0;
			if (now - last < TYPING_HEARTBEAT_MS) return;
			typingHeartbeat[blockId] = now;
		} else {
			delete typingHeartbeat[blockId];
		}

		try {
			await fetch(`${apiUrl}/v1/pages/${pageId}/typing`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					block_id: blockId,
					session_id: viewerSessionId,
					user_name: viewerName,
					is_typing: isTyping
				})
			});
		} catch {
			// typing signals are best-effort
		}
	}

	async function publishPresence(isOnline: boolean) {
		if (!pageId || !viewerSessionId || !viewerName) return;
		try {
			await fetch(`${apiUrl}/v1/pages/${pageId}/presence`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					session_id: viewerSessionId,
					user_name: viewerName,
					is_online: isOnline
				})
			});
		} catch {
			// presence signals are best-effort
		}
	}

	function markSelfActive() {
		if (!viewerSessionId || !viewerName) return;
		activeUsers = {
			...activeUsers,
			[viewerSessionId]: {
				sessionId: viewerSessionId,
				userName: viewerName,
				lastSeen: Date.now()
			}
		};
	}

	function pruneStaleUsers() {
		const now = Date.now();
		const next: Record<string, { sessionId: string; userName: string; lastSeen: number }> = {};
		for (const [sessionId, user] of Object.entries(activeUsers)) {
			if (now - user.lastSeen <= PRESENCE_TTL_MS) {
				next[sessionId] = user;
			}
		}
		activeUsers = next;
	}

	function startPresenceHeartbeat() {
		if (presenceHeartbeatTimer || !pageId) return;
		void publishPresence(true);
		markSelfActive();
		presenceHeartbeatTimer = setInterval(() => {
			void publishPresence(true);
			markSelfActive();
			pruneStaleUsers();
		}, PRESENCE_HEARTBEAT_MS);
	}

	function stopPresenceHeartbeat() {
		if (!presenceHeartbeatTimer) return;
		clearInterval(presenceHeartbeatTimer);
		presenceHeartbeatTimer = null;
	}

	function handleBlockTyping(e: CustomEvent<{ id: string; isTyping: boolean }>) {
		const blockId = e.detail?.id;
		if (!blockId) return;
		void publishTyping(blockId, !!e.detail.isTyping);
	}

	function setThemeFromRgb(base: Rgb, accent: Rgb) {
		paletteBase = base;
		paletteAccent = accent;
		themeStyle = buildThemeStyle(base, accent, { darkMode, cinematicEnabled, moodStrength });
	}

	function handleMoodInput(e: Event) {
		moodStrength = Number((e.target as HTMLInputElement).value);
		setThemeFromRgb(paletteBase, paletteAccent);
		markMetaDirtyAndScheduleSync();
	}

	function handleCinematicToggle(e: Event) {
		cinematicEnabled = (e.target as HTMLInputElement).checked;
		setThemeFromRgb(paletteBase, paletteAccent);
		markMetaDirtyAndScheduleSync();
		void syncPageMetaNow();
	}

	function handleDarkModeToggle(e: Event) {
		darkMode = (e.target as HTMLInputElement).checked;
		setThemeFromRgb(paletteBase, paletteAccent);
		markMetaDirtyAndScheduleSync();
	}

	function handleBgColorInput(e: Event) {
		bgColor = (e.target as HTMLInputElement).value;
		markMetaDirtyAndScheduleSync();
	}

	function clearBgColor() {
		bgColor = '';
		markMetaDirtyAndScheduleSync();
	}

	async function applyCoverPalette(imageSrc: string | null) {
		const palette = await extractPaletteFromImage(imageSrc, FALLBACK_BASE, FALLBACK_ACCENT);
		setThemeFromRgb(palette.base, palette.accent);
	}

	function generateId() {
		if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
			return `tmp-${crypto.randomUUID()}`;
		}
		return `tmp-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	}

	function makeGalleryItemId() {
		return `g-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	}

	function addBlock(type: string, afterId?: string) {
		const defaultData =
			type === 'heading'
				? { text: '' }
				: type === 'image'
					? { url: '' }
					: type === 'embed'
						? { url: '' }
					: type === 'gallery'
						? { items: [], columns: 2 }
						: { text: '' };

		const newBlock: ApiBlock = {
			id: generateId(),
			type,
			position: blocks.length,
			data: defaultData
		};

		if (afterId) {
			const index = blocks.findIndex((b) => b.id === afterId);
			if (index !== -1) {
				blocks = [...blocks.slice(0, index + 1), newBlock, ...blocks.slice(index + 1)];
				markBlocksDirtyAndScheduleSync();
			}
		} else {
			blocks = [...blocks, newBlock];
			markBlocksDirtyAndScheduleSync();
		}
	}

	function addEmptyBlock() {
		addBlock('paragraph');
	}

	function updateBlock(e: CustomEvent) {
		const { id, type, data } = e.detail;
		const idx = blocks.findIndex((b) => b.id === id);
		if (idx !== -1) {
			// Only update the specific block, don't trigger full array reactivity
			// This prevents re-rendering all blocks on every keystroke
			blocks[idx] = { ...blocks[idx], type, data };
			// Don't reassign blocks array - the Block component manages its own state
			markBlocksDirtyAndScheduleSync();
		}
	}

	function deleteBlock(e: CustomEvent) {
		const { id } = e.detail;
		blocks = blocks.filter((b) => b.id !== id);
		markBlocksDirtyAndScheduleSync();
	}

	function addBlockAfter(e: CustomEvent) {
		const { id, type: blockType } = e.detail;
		const continueTypes = ['bullet', 'numbered'];
		const nextType = blockType && continueTypes.includes(blockType) ? blockType : 'paragraph';
		addBlock(nextType, id);
	}

	function transformBlock(e: CustomEvent) {
		const { id, newType } = e.detail;
		const block = blocks.find((b) => b.id === id);
		if (block) {
			block.type = newType;
			block.data =
				newType === 'divider'
					? {}
					: newType === 'image'
						? { url: '' }
						: newType === 'embed'
							? { url: '' }
						: newType === 'gallery'
							? { items: [], columns: 2 }
							: { text: '' };
			blocks = blocks;
			markBlocksDirtyAndScheduleSync();
		}
	}

	function mergeToGallery(e: CustomEvent) {
		const { targetId } = e.detail;
		if (!draggedBlockId || draggedBlockId === targetId) return;

		const sourceIdx = blocks.findIndex((block) => block.id === draggedBlockId);
		const targetIdx = blocks.findIndex((block) => block.id === targetId);
		if (sourceIdx === -1 || targetIdx === -1) return;

		const sourceBlock = blocks[sourceIdx];
		const targetBlock = blocks[targetIdx];

		const sourceItems = getBlockAsGalleryItems(sourceBlock, makeGalleryItemId);
		if (sourceItems.length === 0) return;

		const targetItems = getBlockAsGalleryItems(targetBlock, makeGalleryItemId);

		if (targetBlock.type !== 'image' && targetBlock.type !== 'gallery' && targetBlock.type !== 'embed') return;

		const nextBlocks = [...blocks];
		const targetColumns =
			targetBlock.type === 'gallery' ? Number(targetBlock.data?.columns || 2) : 2;
		nextBlocks[targetIdx] = {
			...targetBlock,
			type: 'gallery',
			data: toGalleryData(targetBlock.data, [...targetItems, ...sourceItems], targetColumns)
		};

		nextBlocks.splice(sourceIdx, 1);
		blocks = nextBlocks;
		draggedBlockId = null;
		markBlocksDirtyAndScheduleSync();
	}

	function handleGalleryCardDrop(e: DragEvent, afterId?: string) {
		e.preventDefault();
		e.stopPropagation();

		const raw = e.dataTransfer?.getData('application/x-jot-gallery-card');
		if (!raw) return;

		let payload: { sourceBlockId: string; itemId: string } | null = null;
		try {
			payload = JSON.parse(raw);
		} catch {
			payload = null;
		}
		if (!payload?.sourceBlockId || !payload.itemId) return;

		const sourceIndex = blocks.findIndex((block) => block.id === payload?.sourceBlockId);
		if (sourceIndex === -1) return;

		const sourceBlock = blocks[sourceIndex];
		if (sourceBlock.type !== 'gallery') return;

		const sourceItems = normalizeGalleryItems(sourceBlock.data);
		const extracted = sourceItems.find((item) => item.id === payload?.itemId);
		if (!extracted) return;

		const remainingItems = sourceItems.filter((item) => item.id !== payload?.itemId);
		const nextBlocks = [...blocks];
		nextBlocks[sourceIndex] = {
			...sourceBlock,
			data: toGalleryData(sourceBlock.data, remainingItems, Number(sourceBlock.data?.columns || 2))
		};

		const newBlock: ApiBlock = {
			id: generateId(),
			type: extracted.kind === 'image' ? 'image' : extracted.kind === 'embed' ? 'embed' : 'paragraph',
			position: nextBlocks.length,
			data:
				extracted.kind === 'image'
					? { url: extracted.value }
					: extracted.kind === 'embed'
						? { url: extracted.value }
						: { text: extracted.value }
		};

		if (afterId) {
			const insertIndex = nextBlocks.findIndex((block) => block.id === afterId);
			if (insertIndex !== -1) {
				nextBlocks.splice(insertIndex + 1, 0, newBlock);
			} else {
				nextBlocks.push(newBlock);
			}
		} else {
			nextBlocks.push(newBlock);
		}

		blocks = nextBlocks;
		draggedBlockId = null;
		markBlocksDirtyAndScheduleSync();
	}

	function handleDragStart(e: CustomEvent) {
		draggedBlockId = e.detail.id;
	}

	function handleDragOver(e: DragEvent, targetId: string) {
		e.preventDefault();
		if (!draggedBlockId || draggedBlockId === targetId) return;

		const draggedIndex = blocks.findIndex((b) => b.id === draggedBlockId);
		const targetIndex = blocks.findIndex((b) => b.id === targetId);

		if (draggedIndex !== -1 && targetIndex !== -1) {
			[blocks[draggedIndex], blocks[targetIndex]] = [blocks[targetIndex], blocks[draggedIndex]];
			blocks = blocks;
		}
	}

	function handleDragEnd() {
		draggedBlockId = null;
	}

	async function loadPage() {
		if (!pageId) return;
		status = 'Loading…';
		try {
			const response = await fetch(`${apiUrl}/v1/pages/${pageId}`);
			if (!response.ok) throw new Error('Failed to load');

			const page: ApiPage = await response.json();
			title = page.title;
			if (titleEl) titleEl.textContent = title;
			cover = page.cover || null;
			isPublished = !!page.published;
			darkMode = page.dark_mode ?? darkMode;
			cinematicEnabled = page.cinematic ?? false;
			moodStrength = page.mood ?? moodStrength;
			bgColor = page.bg_color ?? bgColor;
			applyCoverPalette(cover);
			blocks = page.blocks || [];
			syncTocNow();
			pageRevision = page.updated_at || '';
			hasUnsyncedChanges = false;
			hasUnsyncedMeta = false;
			status = 'Loaded.';
			setTimeout(() => (status = ''), 2000);
		} catch (error) {
			status = 'Failed to load.';
		}
	}

	async function createPage() {
		if (creatingPagePromise) {
			return creatingPagePromise;
		}

		creatingPagePromise = (async () => {
		status = 'Creating…';
		try {
			const response = await fetch(`${apiUrl}/v1/pages`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title, blocks, dark_mode: darkMode, cinematic: cinematicEnabled, mood: moodStrength, bg_color: bgColor })
			});

			if (!response.ok) throw new Error('Failed to create');

			const page: ApiPage = await response.json();
			pageId = page.id;
			title = page.title || title;
			if (titleEl) titleEl.textContent = title;
			cover = page.cover || cover;
			isPublished = !!page.published;
			darkMode = page.dark_mode ?? darkMode;
			cinematicEnabled = page.cinematic ?? false;
			moodStrength = page.mood ?? moodStrength;
			bgColor = page.bg_color ?? bgColor;
			blocks = page.blocks || blocks;
			syncTocNow();
			pageRevision = page.updated_at || '';
			await goto(`/editor?pageId=${encodeURIComponent(page.id)}`, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
			status = 'Created.';
			setTimeout(() => (status = ''), 2000);
			hasUnsyncedMeta = false;
			return true;
		} catch (error) {
			status = 'Failed to create.';
			return false;
		} finally {
			creatingPagePromise = null;
		}
		})();

		return creatingPagePromise;
	}

	async function savePage() {
		if (!pageId) return;
		await syncBlocksNow(true);
	}

	async function togglePublish() {
		if (!pageId) {
			const created = await createPage();
			if (!created || !pageId) return;
		}

		const nextPublished = !isPublished;
		status = nextPublished ? 'Publishing…' : 'Unpublishing…';
		try {
			const response = await fetch(`${apiUrl}/v1/pages/${pageId}/publish`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ published: nextPublished })
			});
			if (!response.ok) throw new Error('publish update failed');
			const payload = await response.json();
			const updated = payload?.page as ApiPage | undefined;
			isPublished = updated?.published ?? nextPublished;
			if (updated?.updated_at) {
				pageRevision = updated.updated_at;
			}
			status = isPublished ? 'Published.' : 'Unpublished.';
			setTimeout(() => {
				if (status === 'Published.' || status === 'Unpublished.') status = '';
			}, 1600);
		} catch {
			status = 'Publish update failed.';
		}
	}

	function scheduleTocUpdate() {
		if (tocTimer) clearTimeout(tocTimer);
		tocTimer = setTimeout(() => {
			tocTimer = null;
			tocBlocks = blocks.map(b => ({ ...b }));
		}, 400);
	}

	function syncTocNow() {
		if (tocTimer) clearTimeout(tocTimer);
		tocTimer = null;
		tocBlocks = blocks.map(b => ({ ...b }));
	}

	function markBlocksDirtyAndScheduleSync() {
		hasUnsyncedChanges = true;
		scheduleTocUpdate();
		if (syncRetryTimer) {
			clearTimeout(syncRetryTimer);
			syncRetryTimer = null;
		}

		if (pageId) {
			scheduleSync();
			return;
		}

		void (async () => {
			const created = await createPage();
			if (!created || !hasUnsyncedChanges) return;
			scheduleSync(80);
		})();
	}

	function markMetaDirtyAndScheduleSync() {
		hasUnsyncedMeta = true;

		if (pageId) {
			scheduleMetaSync();
			return;
		}

		void (async () => {
			const created = await createPage();
			if (!created || !hasUnsyncedMeta) return;
			scheduleMetaSync(80);
		})();
	}

	function scheduleMetaSync(delay = SYNC_DEBOUNCE_MS) {
		if (metaSyncTimer) clearTimeout(metaSyncTimer);
		metaSyncTimer = setTimeout(() => {
			metaSyncTimer = null;
			void syncPageMetaNow();
		}, delay);
	}

	function scheduleSync(delay = SYNC_DEBOUNCE_MS) {
		if (syncTimer) clearTimeout(syncTimer);
		syncTimer = setTimeout(() => {
			syncTimer = null;
			void syncBlocksNow();
		}, delay);
	}

	function blocksForSync() {
		return blocks.map((block, index) => ({ ...block, position: index }));
	}

	function scheduleRetry() {
		syncRetryCount += 1;
		const retryDelay = Math.min(4000, 350 * 2 ** Math.min(syncRetryCount, 4));
		if (syncRetryTimer) clearTimeout(syncRetryTimer);
		syncRetryTimer = setTimeout(() => {
			syncRetryTimer = null;
			void syncBlocksNow();
		}, retryDelay);
	}

	async function syncBlocksNow(forceStatus = false) {
		if (!pageId || !hasUnsyncedChanges) return;
		if (syncInFlight) {
			pendingSyncAfterInFlight = true;
			return;
		}

		syncInFlight = true;
		pendingSyncAfterInFlight = false;
		const payloadBlocks = blocksForSync();
		const baseUpdatedAt = pageRevision || undefined;

		if (forceStatus) {
			status = 'Saving…';
		}

		try {
			const response = await fetch(`${apiUrl}/v1/pages/${pageId}/realtime-blocks`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ blocks: payloadBlocks, base_updated_at: baseUpdatedAt })
			});

			if (response.status === 409) {
				const conflictPayload = await response.json();
				const latest = conflictPayload?.page as ApiPage | undefined;
				if (latest?.blocks) {
					blocks = latest.blocks;
					syncTocNow();
					pageRevision = latest.updated_at || pageRevision;
				}
				hasUnsyncedChanges = false;
				syncRetryCount = 0;
				status = 'Synced remote updates. Keep editing.';
				setTimeout(() => (status = ''), 2200);
				return;
			}

			if (!response.ok) {
				throw new Error('sync failed');
			}

			const payload = await response.json();
			const page = payload?.page as ApiPage | undefined;
			if (page?.updated_at) {
				pageRevision = page.updated_at;
			}
			hasUnsyncedChanges = false;
			syncRetryCount = 0;
			if (forceStatus) {
				status = 'Saved.';
				setTimeout(() => (status = ''), 1200);
			}
		} catch {
			status = 'Syncing… retrying';
			scheduleRetry();
		} finally {
			syncInFlight = false;
			if (pendingSyncAfterInFlight && hasUnsyncedChanges) {
				pendingSyncAfterInFlight = false;
				scheduleSync(120);
			}
		}
	}

	async function syncPageMetaNow() {
		if (!pageId || !hasUnsyncedMeta) return;
		if (metaSyncInFlight) return;

		metaSyncInFlight = true;
		const baseUpdatedAt = pageRevision || undefined;

		try {
			const response = await fetch(`${apiUrl}/v1/pages/${pageId}/meta`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					title,
					cover,
					dark_mode: darkMode,
					cinematic: cinematicEnabled,
					mood: moodStrength,
					bg_color: bgColor,
					base_updated_at: baseUpdatedAt
				})
			});

			if (response.status === 409) {
				const conflictPayload = await response.json();
				const latest = conflictPayload?.page as ApiPage | undefined;
				if (latest) {
					title = latest.title || title;
					if (titleEl) titleEl.textContent = title;
					cover = latest.cover || null;
					darkMode = latest.dark_mode ?? darkMode;
					cinematicEnabled = latest.cinematic ?? cinematicEnabled;
					moodStrength = latest.mood ?? moodStrength;
					bgColor = latest.bg_color ?? bgColor;
					blocks = latest.blocks || blocks;
					syncTocNow();
					pageRevision = latest.updated_at || pageRevision;
					setThemeFromRgb(paletteBase, paletteAccent);
				}
				hasUnsyncedMeta = false;
				return;
			}

			if (!response.ok) {
				throw new Error('meta sync failed');
			}

			const payload = await response.json();
			const updatedPage = payload?.page as ApiPage | undefined;
			if (updatedPage?.updated_at) {
				pageRevision = updatedPage.updated_at;
			}
			hasUnsyncedMeta = false;
		} catch {
			// keep dirty state for next debounce/retry cycle
		} finally {
			metaSyncInFlight = false;
		}
	}

	function updateTitle(e: Event) {
		const el = e.target as HTMLElement;
		title = el.textContent || '';
		markMetaDirtyAndScheduleSync();
	}

	function handleTitleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
		}
	}

	function handleTitlePaste(e: ClipboardEvent) {
		e.preventDefault();
		const text = e.clipboardData?.getData('text/plain') || '';
		document.execCommand('insertText', false, text.replace(/\n/g, ' '));
	}

	function handleCoverChange(e: CustomEvent) {
		cover = e.detail.cover;
		void applyCoverPalette(cover);
		markMetaDirtyAndScheduleSync();
	}

	function handleBeforeUnload(e: BeforeUnloadEvent) {
		if (!hasUnsyncedChanges && !hasUnsyncedMeta) return;
		e.preventDefault();
		e.returnValue = '';
	}

	function closeLiveEvents() {
		stopPresenceHeartbeat();
		if (liveReconnectTimer) {
			clearTimeout(liveReconnectTimer);
			liveReconnectTimer = null;
		}
		if (liveEventsSource) {
			liveEventsSource.close();
			liveEventsSource = null;
		}
		liveEventsPageId = '';
		liveRetryCount = 0;
		liveConnectionState = 'idle';
		for (const blockId of Object.keys(typingLockTimers)) {
			clearTypingTimer(blockId);
		}
		typingLocks = {};
		activeUsers = {};
	}

	function scheduleLiveReconnect(targetPageID: string) {
		if (!targetPageID) return;
		if (liveReconnectTimer) return;

		liveConnectionState = 'reconnecting';
		const delay = Math.min(8000, 450 * 2 ** Math.min(liveRetryCount, 4));
		liveRetryCount += 1;

		liveReconnectTimer = setTimeout(() => {
			liveReconnectTimer = null;
			connectLiveEvents(targetPageID);
		}, delay);
	}

	function connectLiveEvents(targetPageID: string) {
		if (typeof window === 'undefined' || !targetPageID) return;
		if (liveEventsSource && liveEventsPageId === targetPageID) return;

		if (liveEventsPageId !== targetPageID) {
			liveRetryCount = 0;
		}
		if (liveReconnectTimer) {
			clearTimeout(liveReconnectTimer);
			liveReconnectTimer = null;
		}
		if (liveEventsSource) {
			liveEventsSource.close();
			liveEventsSource = null;
		}
		liveConnectionState = 'connecting';

		const source = new EventSource(`${apiUrl}/v1/pages/${encodeURIComponent(targetPageID)}/events`);
		source.onopen = () => {
			liveConnectionState = 'live';
			liveRetryCount = 0;
			startPresenceHeartbeat();
		};

		source.addEventListener('page', (event) => {
			if (!(event instanceof MessageEvent)) return;
			let payload: { page?: ApiPage } | null = null;
			try {
				payload = JSON.parse(event.data);
			} catch {
				payload = null;
			}

			const incoming = payload?.page;
			if (!incoming?.id) return;
			if (incoming.id !== pageId) return;
			if (incoming.updated_at && incoming.updated_at === pageRevision) return;
			if (hasUnsyncedChanges || hasUnsyncedMeta || syncInFlight || metaSyncInFlight) return;

			title = incoming.title || title;
			if (titleEl) titleEl.textContent = title;
			cover = incoming.cover || null;
			isPublished = !!incoming.published;
			darkMode = incoming.dark_mode ?? darkMode;
			cinematicEnabled = incoming.cinematic ?? cinematicEnabled;
			moodStrength = incoming.mood ?? moodStrength;
			bgColor = incoming.bg_color ?? bgColor;
			blocks = incoming.blocks || blocks;
			syncTocNow();
			if (incoming.updated_at) {
				pageRevision = incoming.updated_at;
			}
			void applyCoverPalette(cover);
			status = 'Live updated';
			setTimeout(() => {
				if (status === 'Live updated') status = '';
			}, 1200);
		});

		source.addEventListener('typing', (event) => {
			if (!(event instanceof MessageEvent)) return;
			let payload: { typing?: { page_id?: string; block_id?: string; session_id?: string; user_name?: string; is_typing?: boolean } } | null = null;
			try {
				payload = JSON.parse(event.data);
			} catch {
				payload = null;
			}

			const typing = payload?.typing;
			if (!typing) return;
			if (typing.page_id !== pageId) return;
			if (!typing.block_id || !typing.session_id || !typing.user_name) return;
			if (typing.session_id === viewerSessionId) return;

			if (!typing.is_typing) {
				clearTypingTimer(typing.block_id);
				const next = { ...typingLocks };
				delete next[typing.block_id];
				typingLocks = next;
				return;
			}

			const expiresAt = Date.now() + TYPING_LOCK_TTL_MS;
			typingLocks = {
				...typingLocks,
				[typing.block_id]: {
					sessionId: typing.session_id,
					userName: typing.user_name,
					expiresAt
				}
			};
			scheduleTypingExpiry(typing.block_id, expiresAt);
		});

		source.addEventListener('presence', (event) => {
			if (!(event instanceof MessageEvent)) return;
			let payload: { presence?: { page_id?: string; session_id?: string; user_name?: string; is_online?: boolean } } | null = null;
			try {
				payload = JSON.parse(event.data);
			} catch {
				payload = null;
			}

			const presence = payload?.presence;
			if (!presence) return;
			if (presence.page_id !== pageId) return;
			if (!presence.session_id || !presence.user_name) return;

			if (!presence.is_online) {
				const next = { ...activeUsers };
				delete next[presence.session_id];
				activeUsers = next;
				return;
			}

			activeUsers = {
				...activeUsers,
				[presence.session_id]: {
					sessionId: presence.session_id,
					userName: presence.user_name,
					lastSeen: Date.now()
				}
			};
			pruneStaleUsers();
		});

		source.onerror = () => {
			if (liveEventsSource === source) {
				source.close();
				liveEventsSource = null;
			}
			scheduleLiveReconnect(targetPageID);
		};

		liveEventsSource = source;
		liveEventsPageId = targetPageID;
	}

	onMount(() => {
		setupViewerIdentity();
		markSelfActive();
		window.addEventListener('beforeunload', handleBeforeUnload);
		if (titleEl) titleEl.textContent = title;
		syncTocNow();
	});

	$: {
		const routePageId = $page.url.searchParams.get('pageId')?.trim() || '';
		if (!routePageId) {
			loadedRoutePageId = '';
		} else if (routePageId !== loadedRoutePageId && routePageId !== pageId) {
			loadedRoutePageId = routePageId;
			pageId = routePageId;
			void loadPage();
		}
	}

	$: {
		if (!pageId) {
			closeLiveEvents();
		} else {
			connectLiveEvents(pageId);
		}
	}

	onDestroy(() => {
		void publishPresence(false);
		stopPresenceHeartbeat();
		for (const blockId of Object.keys(typingHeartbeat)) {
			void publishTyping(blockId, false);
		}
		closeLiveEvents();
		if (syncTimer) clearTimeout(syncTimer);
		if (syncRetryTimer) clearTimeout(syncRetryTimer);
		if (metaSyncTimer) clearTimeout(metaSyncTimer);
		if (tocTimer) clearTimeout(tocTimer);
		if (typeof window !== 'undefined') {
			window.removeEventListener('beforeunload', handleBeforeUnload);
		}
	});
</script>

<div class="editor-shell">
	<aside class="cover-rail" class:has-cover={!!cover}>
		<Cover {cover} {apiUrl} {title} blocks={tocBlocks} on:change={handleCoverChange} />
	</aside>

	<main class="editor-main" class:dark={darkMode} class:cinematic-on={cinematicEnabled} class:has-bg-color={!!bgColor} style="{themeStyle}{bgColor ? `--note-user-bg:${bgColor};` : ''}">
		<button class="controls-fab" type="button" on:click={() => (controlsOpen = !controlsOpen)}>
			{controlsOpen ? 'Close' : 'Controls'}
		</button>

		{#if controlsOpen}
			<div class="controls-panel" role="dialog" aria-label="Editor controls">
				<div class="controls-top">
					<div class="live-pill" class:live={liveConnectionState === 'live'} class:reconnecting={liveConnectionState === 'reconnecting'}>
						{#if liveConnectionState === 'live'}
							Live
						{:else if liveConnectionState === 'reconnecting'}
							Reconnecting…
						{:else if liveConnectionState === 'connecting'}
							Connecting…
						{:else}
							Offline
						{/if}
					</div>
					{#if status}
						<div class="panel-status">{status}</div>
					{/if}
				</div>

				<div class="publish-row">
					<button type="button" class="publish-btn" class:published={isPublished} on:click={togglePublish}>
						{isPublished ? 'Published' : 'Publish page'}
					</button>
					{#if isPublished && pageId}
						<a class="public-link" href={`/public/${pageId}`} target="_blank" rel="noreferrer">Open public</a>
					{/if}
				</div>

				<div class="mood-control" class:enabled={cinematicEnabled}>
					<div class="mood-row">
						<span class="mood-label">Cinematic mood</span>
						<label class="mood-toggle">
							<input type="checkbox" checked={cinematicEnabled} on:change={handleCinematicToggle} />
							<span>{cinematicEnabled ? 'On' : 'Off'}</span>
						</label>
					</div>
					<div class="mood-row">
						<span class="mood-label">Dark mode</span>
						<label class="mood-toggle">
							<input type="checkbox" checked={darkMode} on:change={handleDarkModeToggle} />
							<span>{darkMode ? 'On' : 'Off'}</span>
						</label>
					</div>
					<div class="mood-slider-wrap">
						<input
							type="range"
							min="0"
							max="100"
							step="1"
							value={moodStrength}
							on:input={handleMoodInput}
							disabled={!cinematicEnabled}
						/>
					</div>
				</div>

				<div class="bg-color-control">
					<div class="mood-row">
						<span class="mood-label">Background</span>
						{#if bgColor}
							<button type="button" class="bg-clear-btn" on:click={clearBgColor}>Clear</button>
						{/if}
					</div>
					<div class="bg-color-picker-row">
						<input
							type="color"
							class="bg-color-swatch"
							value={bgColor || '#ffffff'}
							on:input={handleBgColorInput}
						/>
						<input
							type="text"
							class="bg-color-hex"
							value={bgColor || ''}
							placeholder="none"
							on:change={(e) => { bgColor = (e.target as HTMLInputElement).value.trim(); markMetaDirtyAndScheduleSync(); }}
						/>
					</div>
				</div>

				{#if visibleUsers.length > 0}
					<div class="users-on-page" aria-label="Users on page">
						{#each visibleUsers as user (user.sessionId)}
							<span class="user-chip" class:self={user.sessionId === viewerSessionId}>
								{user.sessionId === viewerSessionId ? `${user.userName} (You)` : user.userName}
							</span>
						{/each}
					</div>
				{/if}
			</div>
		{/if}

		<div class="editor-wrapper">
			<div class="page-header">
				<div class="page-title-wrap">
					<!-- svelte-ignore a11y-no-static-element-interactions -->
					<div
						bind:this={titleEl}
						class="page-title"
						class:cinematic={cinematicEnabled}
						contenteditable="true"
						role="textbox"
						aria-label="Page title"
						data-placeholder="Untitled"
						on:input={updateTitle}
						on:keydown={handleTitleKeydown}
						on:paste={handleTitlePaste}
					></div>
					{#if status}
						<div class="inline-status">{status}</div>
					{/if}
				</div>
			</div>

			<div class="blocks-container">
				{#each blocks as block, idx (block.id)}
					{@const listNumber = block.type === 'numbered'
						? (() => { let n = 1; for (let i = idx - 1; i >= 0; i--) { if (blocks[i].type === 'numbered') n++; else break; } return n; })()
						: 1}
					<div
						class="block-wrapper"
						role="group"
						aria-label="Editor block wrapper"
						data-block-id={block.id}
						on:dragover={(e) => handleDragOver(e, block.id!)}
						on:drop={(e) => handleGalleryCardDrop(e, block.id!)}
						on:dragend={handleDragEnd}
					>
						<Block
							id={block.id!}
							type={block.type}
							data={block.data}
							{apiUrl}
							{pageId}
							{listNumber}
							published={isPublished}
							{viewerSessionId}
							lockOwner={block.id ? typingLocks[block.id] || null : null}
							isDragging={draggedBlockId === block.id}
							on:update={updateBlock}
							on:typing={handleBlockTyping}
							on:delete={deleteBlock}
							on:addAfter={addBlockAfter}
							on:transform={transformBlock}
							on:mergeToGallery={mergeToGallery}
							on:dragstart={handleDragStart}
							on:galleryCardDragStart={() => (draggedBlockId = null)}
						/>
					</div>
				{/each}

				<div class="click-to-add" on:click={addEmptyBlock} on:dragover|preventDefault on:drop={(e) => handleGalleryCardDrop(e)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && addEmptyBlock()}></div>
			</div>
		</div>
	</main>
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
		background: var(--note-surface, #fafafa);
		border-right: 1px solid var(--note-border, transparent);
		overflow: hidden;
		transition: width 0.25s ease, min-width 0.25s ease, flex-basis 0.25s ease, border-color 0.2s ease;
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
		max-width: 900px;
		margin: 0 auto;
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		padding: 0 64px;
		background: transparent;
	}

	.page-header {
		margin: 32px 0 24px;
		display: flex;
		align-items: flex-end;
		justify-content: flex-start;
		flex-wrap: wrap;
		gap: 20px;
	}

	.controls-fab {
		position: fixed;
		top: 18px;
		right: 18px;
		z-index: 12;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 2px solid var(--note-title, #1a1a1a);
		background: var(--note-surface, #fff);
		color: var(--note-title, #1a1a1a);
		font-size: 13px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		padding: 8px 16px;
		border-radius: 8px;
		cursor: pointer;
	}

	.controls-fab:hover {
		transform: translateY(-2px);
		box-shadow: 4px 4px 0 var(--note-title, #1a1a1a);
	}

	.controls-panel {
		position: fixed;
		top: 56px;
		right: 18px;
		z-index: 12;
		width: min(360px, calc(100vw - 24px));
		padding: 14px;
		border-radius: 8px;
		border: 2px solid var(--note-title, #1a1a1a);
		background: var(--note-surface, #fff);
		display: grid;
		gap: 12px;
		box-shadow: 6px 6px 0 var(--note-title, #1a1a1a);
	}

	.controls-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
	}

	.panel-status,
	.inline-status {
		font-size: 12px;
		font-weight: 600;
		color: var(--note-muted, #6b7280);
	}

	.page-title-wrap {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.live-pill {
		display: inline-flex;
		align-items: center;
		width: fit-content;
		font-size: 11px;
		font-weight: 800;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		padding: 4px 10px;
		border-radius: 6px;
		border: 2px solid var(--note-title, #1a1a1a);
		background: var(--note-surface, #fff);
		color: var(--note-title, #1a1a1a);
	}

	.live-pill.live {
		color: #166534;
		border-color: #166534;
		background: #ecfdf3;
	}

	.live-pill.reconnecting {
		color: #92400e;
		border-color: #92400e;
		background: #fffbeb;
	}

	.editor-main.dark .live-pill.live {
		color: #bbf7d0;
		border-color: #166534;
		background: #0a2e1a;
	}

	.editor-main.dark .live-pill.reconnecting {
		color: #fde68a;
		border-color: #92400e;
		background: #2e1a05;
	}

	.publish-row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.publish-btn {
		display: inline-flex;
		align-items: center;
		border: 2px solid var(--note-title, #1a1a1a);
		background: var(--note-title, #1a1a1a);
		color: var(--note-surface, #fff);
		font-size: 13px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		padding: 8px 16px;
		border-radius: 6px;
		cursor: pointer;
		transition: transform 0.12s, box-shadow 0.12s;
	}

	.publish-btn:hover {
		transform: translateY(-2px);
		box-shadow: 4px 4px 0 var(--note-title, #1a1a1a);
	}

	.publish-btn.published {
		background: var(--note-surface, #fff);
		color: var(--note-title, #1a1a1a);
	}



	.public-link {
		font-size: 12px;
		font-weight: 600;
		color: var(--note-accent, #7c5cff);
		text-decoration: none;
	}

	.users-on-page {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.user-chip {
		display: inline-flex;
		align-items: center;
		font-size: 11px;
		font-weight: 700;
		padding: 4px 9px;
		border-radius: 6px;
		border: 2px solid var(--note-title, #1a1a1a);
		background: var(--note-surface, #fff);
		color: var(--note-title, #1a1a1a);
	}

	.user-chip.self {
		background: var(--note-title, #1a1a1a);
		color: var(--note-surface, #fff);
	}

	.mood-control {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.mood-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.mood-label {
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.03em;
		text-transform: uppercase;
		color: var(--note-muted, #6b7280);
	}

	.mood-slider-wrap {
		opacity: 0;
		max-height: 0;
		overflow: hidden;
		transition: opacity 0.18s ease, max-height 0.18s ease;
	}

	.mood-control.enabled:hover .mood-slider-wrap,
	.mood-control.enabled:focus-within .mood-slider-wrap {
		opacity: 1;
		max-height: 40px;
	}

	.mood-slider-wrap input[type='range'] {
		width: 100%;
		accent-color: var(--note-accent, #7c5cff);
		cursor: pointer;
	}

	.mood-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--note-muted, #6b7280);
		cursor: pointer;
	}

	.mood-toggle input {
		accent-color: var(--note-accent, #7c5cff);
	}

	.page-title {
		font-size: clamp(24px, 5vw, 40px);
		font-weight: 700;
		line-height: 1.2;
		border: none;
		background: transparent;
		padding: 0;
		margin: 0;
		width: 100%;
		max-width: 100%;
		min-width: 0;
		font-family: var(--font-display);
		letter-spacing: 0.01em;
		outline: none;
		color: var(--note-title, #111827);
		border-radius: 0;
		text-shadow: 0 0 28px color-mix(in srgb, var(--note-title-glow, transparent) 20%, transparent);
		word-wrap: break-word;
		overflow-wrap: break-word;
		word-break: break-word;
		white-space: pre-wrap;
		cursor: text;
		-webkit-user-modify: read-write-plaintext-only;
	}

	.page-title:empty::before {
		content: attr(data-placeholder);
		color: color-mix(in srgb, var(--note-muted, #6b7280) 55%, #ffffff);
		pointer-events: none;
	}

	.blocks-container {
		display: flex;
		flex-direction: column;
		gap: 0;
		width: 100%;
		min-width: 0;
		padding-bottom: 100px;
	}

	.block-wrapper {
		position: relative;
		display: flex;
		align-items: flex-start;
		gap: 8px;
		margin: 0;
	}

	.click-to-add {
		min-height: 200px;
		cursor: text;
	}

	/* ── Background color override ── */

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

	/* ── Background color control ── */

	.bg-color-control {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.bg-color-picker-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.bg-color-swatch {
		width: 32px;
		height: 32px;
		border: 2px solid var(--note-title, #1a1a1a);
		border-radius: 6px;
		padding: 0;
		cursor: pointer;
		background: none;
		flex-shrink: 0;
	}

	.bg-color-swatch::-webkit-color-swatch-wrapper {
		padding: 2px;
	}

	.bg-color-swatch::-webkit-color-swatch {
		border: none;
		border-radius: 3px;
	}

	.bg-color-hex {
		flex: 1;
		min-width: 0;
		border: 2px solid var(--note-title, #1a1a1a);
		border-radius: 6px;
		padding: 6px 8px;
		font-size: 12px;
		font-weight: 600;
		font-family: 'SF Mono', 'Menlo', monospace;
		letter-spacing: 0.04em;
		background: var(--note-surface, #fff);
		color: var(--note-title, #1a1a1a);
		outline: none;
	}

	.bg-color-hex::placeholder {
		color: #999;
		font-weight: 400;
	}

	.bg-clear-btn {
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 3px 8px;
		border: 2px solid var(--note-title, #1a1a1a);
		border-radius: 5px;
		background: var(--note-surface, #fff);
		color: var(--note-title, #1a1a1a);
		cursor: pointer;
	}

	.bg-clear-btn:hover {
		background: var(--note-title, #1a1a1a);
		color: var(--note-surface, #fff);
	}



	@media (max-width: 980px) {
		.editor-shell {
			flex-direction: column;
		}

		.cover-rail {
			width: 100%;
			min-width: 0;
			height: 220px;
		}

		.cover-rail:hover,
		.cover-rail.has-cover {
			width: 100%;
			min-width: 0;
			flex: 0 0 auto;
		}

		.editor-wrapper {
			padding: 0 16px;
		}

		.page-header {
			flex-direction: column;
			align-items: stretch;
		}

		.controls-fab {
			top: 10px;
			right: 10px;
		}

		.controls-panel {
			top: 50px;
			right: 10px;
			left: 10px;
			width: auto;
		}

		.mood-control {
			min-width: 0;
		}

		.mood-control.enabled .mood-slider-wrap {
			opacity: 1;
			max-height: 40px;
		}
	}

	@media (max-width: 680px) {
		.editor-wrapper {
			padding: 0 8px;
		}

		.block-wrapper {
			gap: 2px;
		}

		.blocks-container {
			padding-bottom: 60px;
		}

		.click-to-add {
			min-height: 120px;
		}
	}
</style>
