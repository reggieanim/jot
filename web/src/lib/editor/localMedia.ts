const LOCAL_MEDIA_PREFIX = 'jot-local-media://';
const DB_NAME = 'jot-editor-local-media';
const DB_VERSION = 1;
const STORE_NAME = 'media_blobs';

type LocalMediaRecord = {
	id: string;
	blob: Blob;
	createdAt: number;
};

let openDbPromise: Promise<IDBDatabase> | null = null;
const objectUrlCache = new Map<string, string>();

function openDb(): Promise<IDBDatabase> {
	if (openDbPromise) return openDbPromise;
	openDbPromise = new Promise((resolve, reject) => {
		if (typeof indexedDB === 'undefined') {
			reject(new Error('indexedDB unavailable'));
			return;
		}
		const request = indexedDB.open(DB_NAME, DB_VERSION);
		request.onupgradeneeded = () => {
			const db = request.result;
			if (!db.objectStoreNames.contains(STORE_NAME)) {
				db.createObjectStore(STORE_NAME, { keyPath: 'id' });
			}
		};
		request.onsuccess = () => resolve(request.result);
		request.onerror = () => reject(request.error || new Error('failed to open local media db'));
	});
	return openDbPromise;
}

function randomId() {
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
		return crypto.randomUUID();
	}
	return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function parseRef(ref: string): string | null {
	if (!isLocalMediaRef(ref)) return null;
	return ref.slice(LOCAL_MEDIA_PREFIX.length).trim() || null;
}

function runWrite<T>(executor: (store: IDBObjectStore, resolve: (value: T) => void, reject: (reason?: unknown) => void) => void): Promise<T> {
	return openDb().then(
		(db) =>
			new Promise<T>((resolve, reject) => {
				const tx = db.transaction(STORE_NAME, 'readwrite');
				const store = tx.objectStore(STORE_NAME);
				executor(store, resolve, reject);
			})
	);
}

function runRead<T>(executor: (store: IDBObjectStore, resolve: (value: T) => void, reject: (reason?: unknown) => void) => void): Promise<T> {
	return openDb().then(
		(db) =>
			new Promise<T>((resolve, reject) => {
				const tx = db.transaction(STORE_NAME, 'readonly');
				const store = tx.objectStore(STORE_NAME);
				executor(store, resolve, reject);
			})
	);
}

export function isLocalMediaRef(value: string | null | undefined): value is string {
	return typeof value === 'string' && value.startsWith(LOCAL_MEDIA_PREFIX);
}

export function createLocalMediaRef(id: string): string {
	return `${LOCAL_MEDIA_PREFIX}${id}`;
}

export async function putLocalMediaBlob(blob: Blob): Promise<string> {
	const id = randomId();
	const record: LocalMediaRecord = { id, blob, createdAt: Date.now() };
	await runWrite<void>((store, resolve, reject) => {
		const request = store.put(record);
		request.onsuccess = () => resolve();
		request.onerror = () => reject(request.error || new Error('failed to store local media blob'));
	});
	return createLocalMediaRef(id);
}

export async function getLocalMediaBlobByRef(ref: string): Promise<Blob | null> {
	const id = parseRef(ref);
	if (!id) return null;
	return runRead<Blob | null>((store, resolve, reject) => {
		const request = store.get(id);
		request.onsuccess = () => {
			const result = request.result as LocalMediaRecord | undefined;
			resolve(result?.blob || null);
		};
		request.onerror = () => reject(request.error || new Error('failed to read local media blob'));
	});
}

export async function resolveLocalMediaObjectURL(ref: string): Promise<string | null> {
	const id = parseRef(ref);
	if (!id) return null;
	const cached = objectUrlCache.get(id);
	if (cached) return cached;
	const blob = await getLocalMediaBlobByRef(ref);
	if (!blob) return null;
	const objectUrl = URL.createObjectURL(blob);
	objectUrlCache.set(id, objectUrl);
	return objectUrl;
}

export async function deleteLocalMediaRef(ref: string): Promise<void> {
	const id = parseRef(ref);
	if (!id) return;
	const cached = objectUrlCache.get(id);
	if (cached) {
		URL.revokeObjectURL(cached);
		objectUrlCache.delete(id);
	}
	await runWrite<void>((store, resolve, reject) => {
		const request = store.delete(id);
		request.onsuccess = () => resolve();
		request.onerror = () => reject(request.error || new Error('failed to delete local media blob'));
	});
}
