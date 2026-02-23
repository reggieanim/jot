<script lang="ts">
	import { createEventDispatcher } from 'svelte';

	export let value = '';
	export let placeholder = '';
	export let className = '';
	export let blockId = '';

	const dispatch = createEventDispatcher<{ input: string; keydown: KeyboardEvent }>();
	let element: HTMLDivElement | null = null;

	function handleInput() {
		value = element?.innerText ?? '';
		dispatch('input', value);
	}

	function handleKeydown(event: KeyboardEvent) {
		dispatch('keydown', event);
	}

	$: if (element && element.innerText !== value) {
		element.innerText = value ?? '';
	}
</script>

<div
	class={`content-editable ${className}`}
	contenteditable
	bind:this={element}
	role="textbox"
	aria-multiline="true"
	tabindex="0"
	data-block-id={blockId}
	data-placeholder={placeholder}
	on:input={handleInput}
	on:keydown={handleKeydown}
></div>

<style>
	.content-editable {
		min-height: 28px;
		outline: none;
		white-space: pre-wrap;
	}

	.content-editable:empty::before {
		content: attr(data-placeholder);
		color: #8a8f98;
	}
</style>
