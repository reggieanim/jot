<script lang="ts">
	import { createEventDispatcher } from 'svelte';

	export let isOpen = false;

	const dispatch = createEventDispatcher();

	const blockTypes = [
		{ id: 'paragraph', label: 'Text', icon: '¶', description: 'Plain text' },
		{ id: 'heading', label: 'Heading 1', icon: 'H1', description: 'Large heading' },
		{ id: 'heading2', label: 'Heading 2', icon: 'H2', description: 'Medium heading' },
		{ id: 'heading3', label: 'Heading 3', icon: 'H3', description: 'Small heading' },
		{ id: 'bullet', label: 'Bullet List', icon: '•', description: 'Bulleted list' },
		{ id: 'quote', label: 'Quote', icon: '"', description: 'Quote block' },
		{ id: 'divider', label: 'Divider', icon: '—', description: 'Horizontal line' },
		{ id: 'image', label: 'Image', icon: '🖼', description: 'Upload image' },
		{ id: 'embed', label: 'Embed', icon: '◆', description: 'Embed URL' },
		{ id: 'music', label: 'Music', icon: '♫', description: 'Audio player with waveform' }
	];

	function selectType(typeId: string) {
		dispatch('select', { type: typeId });
		isOpen = false;
	}
</script>

{#if isOpen}
	<div class="block-menu">
		<div class="menu-header">Add block</div>
		<div class="menu-items">
			{#each blockTypes as blockType (blockType.id)}
				<button class="menu-item" on:click={() => selectType(blockType.id)}>
					<span class="icon">{blockType.icon}</span>
					<div class="info">
						<span class="label">{blockType.label}</span>
						<span class="desc">{blockType.description}</span>
					</div>
				</button>
			{/each}
		</div>
	</div>
{/if}

<style>
	.block-menu {
		position: absolute;
		bottom: 100%;
		left: 0;
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #e5e7eb);
		border-radius: 16px;
		box-shadow: 0 20px 48px rgba(15, 23, 42, 0.14);
		margin-bottom: 8px;
		z-index: 1000;
		min-width: 280px;
		max-height: 360px;
		overflow-y: auto;
	}

	.menu-header {
		padding: 12px 14px 8px;
		font-size: 11px;
		font-weight: 700;
		color: var(--note-muted, #9ca3af);
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.menu-items {
		display: flex;
		flex-direction: column;
		padding-bottom: 8px;
	}

	.menu-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 14px;
		background: transparent;
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background 0.1s;
	}

	.menu-item:hover {
		background: color-mix(in srgb, var(--note-surface, #ffffff) 85%, var(--note-accent, #7c5cff) 10%);
	}

	.icon {
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--note-surface, #ffffff);
		border: 1px solid var(--note-border, #e5e7eb);
		border-radius: 4px;
		font-size: 16px;
		flex-shrink: 0;
	}

	.info {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.label {
		font-size: 14px;
		font-weight: 600;
		color: var(--note-text, #1f2328);
	}

	.desc {
		font-size: 12px;
		color: var(--note-muted, #6b7280);
	}
</style>
