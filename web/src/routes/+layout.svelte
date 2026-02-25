<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import moderatRegularWoff from '$lib/assets/Moderat-Regular.woff';
	import { onMount } from 'svelte';
	import { user, authLoading, fetchMe, logout } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';

	const fontFaceCss = `@font-face {
		font-family: 'Moderat';
		src: url('${moderatRegularWoff}') format('woff');
		font-weight: 400;
		font-style: normal;
		font-display: swap;
	}`;

	/** Pages where we do NOT show the global nav (they have their own) */
	$: hideNav = $page.url.pathname === '/' || $page.url.pathname.startsWith('/editor') || $page.url.pathname.startsWith('/public') || $page.url.pathname.startsWith('/proofread') || $page.url.pathname.startsWith('/user') || $page.url.pathname.startsWith('/settings') || $page.url.pathname.startsWith('/feed') || $page.url.pathname.startsWith('/embed');

	onMount(() => {
		fetchMe();
	});

	function handleLogout() {
		logout();
		goto('/login');
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	{@html `<style>${fontFaceCss}</style>`}
</svelte:head>

{#if !hideNav}
	<header class="global-nav">
		<a href="/" class="global-brand">Jot.</a>
		<nav class="global-links">
			{#if $authLoading}
				<!-- loading -->
			{:else if $user}
				<a href="/user/{$user.username}">{$user.display_name || $user.username}</a>
				<a href="/settings" class="global-settings-link">Settings</a>
				<button class="global-logout" on:click={handleLogout}>Log out</button>
			{:else}
				<a href="/login">Log in</a>
				<a href="/signup" class="global-signup-btn">Sign up</a>
			{/if}
		</nav>
	</header>
{/if}

<slot />

<style>
	:global(body) {
		font-family: 'Moderat', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
		letter-spacing: -.025em;
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	.global-nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 14px 28px;
		border-bottom: 2px solid #1a1a1a;
		background: #faf9f7;
	}
	.global-brand {
		font-size: 1.4rem;
		font-weight: 900;
		color: #1a1a1a;
		text-decoration: none;
		letter-spacing: -0.04em;
	}
	.global-links {
		display: flex;
		align-items: center;
		gap: 16px;
	}
	.global-links a {
		color: #1a1a1a;
		text-decoration: none;
		font-weight: 600;
		font-size: .95rem;
		transition: opacity .15s;
	}
	.global-links a:hover {
		opacity: 0.5;
	}
	.global-signup-btn {
		background: #1a1a1a !important;
		color: #fff !important;
		padding: 7px 18px;
		border: 2px solid #1a1a1a !important;
		font-weight: 700;
		font-size: .85rem;
		letter-spacing: .02em;
		cursor: pointer;
		box-shadow: 3px 3px 0 #1a1a1a;
		transition: background .15s, transform .15s, box-shadow .15s;
	}
	.global-signup-btn:hover {
		background: #333 !important;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}
	.global-logout {
		background: none;
		border: 2px solid #1a1a1a;
		padding: 5px 14px;
		font-family: inherit;
		font-weight: 600;
		font-size: .9rem;
		cursor: pointer;
		color: #1a1a1a;
		transition: background .15s, color .15s;
	}
	.global-logout:hover {
		background: #1a1a1a;
		color: #faf9f7;
	}
</style>
