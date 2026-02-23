<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import moderatRegularWoff from '$lib/assets/Moderat-Regular.woff';

	let { children } = $props();

	const fontFaceCss = `@font-face {
		font-family: 'Moderat';
		src: url('${moderatRegularWoff}') format('woff');
		font-weight: 400;
		font-style: normal;
		font-display: swap;
	}`;
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	{@html `<style>${fontFaceCss}</style>`}
</svelte:head>

{@render children()}

<style>
	:global(body) {
		font-family: 'Moderat', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
		letter-spacing: -.025em;
	}
</style>
