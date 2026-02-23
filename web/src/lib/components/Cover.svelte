<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { env } from '$env/dynamic/public';

	export let cover: string | null;
	export let apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';
	export let readonly = false;

	const dispatch = createEventDispatcher();

	let fileInput: HTMLInputElement;

	async function uploadImage(file: File): Promise<string> {
		const formData = new FormData();
		formData.append('file', file);

		const response = await fetch(`${apiUrl}/v1/media/images`, {
			method: 'POST',
			body: formData
		});
		if (!response.ok) {
			throw new Error('cover upload failed');
		}

		const payload = await response.json();
		const url = payload?.url;
		if (typeof url !== 'string' || !url) {
			throw new Error('invalid upload response');
		}

		return url;
	}

	async function handleImageChange(e: Event) {
		const input = e.target as HTMLInputElement;
		if (input.files?.[0]) {
			try {
				const uploadedCover = await uploadImage(input.files[0]);
				dispatch('change', { cover: uploadedCover });
			} catch {
				// ignore upload failure here
			}
			input.value = '';
		}
	}

	function handleRemove() {
		if (readonly) return;
		dispatch('change', { cover: null });
	}

	function triggerUpload() {
		if (readonly) return;
		fileInput?.click();
	}
</script>

<div class="cover-area" class:has-cover={!!cover}>
	{#if cover}
		<div class="cover-image-wrapper">
			<img src={cover} alt="page cover" class="cover-image" />
			{#if !readonly}
				<div class="cover-actions">
					<button class="cover-action-btn" on:click={triggerUpload}>
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
							<circle cx="8.5" cy="8.5" r="1.5"></circle>
							<polyline points="21 15 16 10 5 21"></polyline>
						</svg>
						Change cover
					</button>
					<button class="cover-action-btn" on:click={handleRemove}>
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<polyline points="3 6 5 6 21 6"></polyline>
							<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
						</svg>
						Remove
					</button>
				</div>
			{/if}
		</div>
	{:else}
		{#if !readonly}
			<div class="empty-cover">
				<button class="hover-btn" on:click={triggerUpload}>
					<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
						<circle cx="8.5" cy="8.5" r="1.5"></circle>
						<polyline points="21 15 16 10 5 21"></polyline>
					</svg>
					Add side cover
				</button>
			</div>
		{/if}
	{/if}

	{#if !readonly}
		<input
			bind:this={fileInput}
			type="file"
			accept="image/*"
			on:change={handleImageChange}
			class="hidden-input"
		/>
	{/if}
</div>

<style>
	.cover-area {
		position: relative;
		width: 100%;
		height: 100%;
		min-height: 100%;
		background: transparent;
		overflow: hidden;
	}

	.empty-cover {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: flex-end;
		justify-content: center;
		padding: 24px;
		opacity: 0;
		pointer-events: none;
		transition: opacity 0.15s;
	}

	.cover-area:hover .empty-cover {
		opacity: 1;
		pointer-events: auto;
	}

	.hover-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 12px;
		background: rgba(255, 255, 255, 0.15);
		border: 1px solid rgba(255, 255, 255, 0.25);
		border-radius: 4px;
		color: #e5e7eb;
		font-size: 13px;
		cursor: pointer;
		transition: background 0.15s, color 0.15s;
		backdrop-filter: blur(4px);
	}

	.hover-btn:hover {
		background: rgba(255, 255, 255, 0.24);
		color: #ffffff;
	}

	.cover-image-wrapper {
		position: relative;
		width: 100%;
		height: 100%;
		min-height: 100%;
		overflow: hidden;
	}

	.cover-image {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}

	.cover-actions {
		position: absolute;
		bottom: 16px;
		left: 16px;
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		opacity: 0;
		transition: opacity 0.15s;
	}

	.cover-image-wrapper:hover .cover-actions {
		opacity: 1;
	}

	.cover-action-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 12px;
		background: rgba(0, 0, 0, 0.5);
		color: #ffffff;
		border: none;
		border-radius: 4px;
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: background 0.15s;
		backdrop-filter: blur(4px);
	}

	.cover-action-btn:hover {
		background: rgba(0, 0, 0, 0.8);
	}

	.cover-action-btn svg {
		stroke: currentColor;
	}

	.hidden-input {
		display: none;
	}
</style>
