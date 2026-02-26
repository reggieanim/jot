<script lang="ts">
	import { goto } from '$app/navigation';
	import { env } from '$env/dynamic/public';
	import { login } from '$lib/stores/auth';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let email = '';
	let password = '';
	let loading = false;
	let error = '';
	let showPassword = false;

	async function handleLogin(e: SubmitEvent) {
		e.preventDefault();
		if (!email.trim() || !password) return;

		loading = true;
		error = '';

		try {
			await login(email.trim(), password);
			await goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Log in — Jot.</title>
</svelte:head>

<div class="login-page">
	<!-- decorative grid -->
	<div class="grid-bg" aria-hidden="true">
		{#each Array(6) as _}
			<div class="grid-line-h"></div>
		{/each}
		{#each Array(6) as _}
			<div class="grid-line-v"></div>
		{/each}
	</div>

	<!-- floating shapes -->
	<div class="shape shape-1" aria-hidden="true">✦</div>
	<div class="shape shape-2" aria-hidden="true">✎</div>
	<div class="shape shape-3" aria-hidden="true">¶</div>

	<div class="login-card">
		<div class="card-header">
			<a href="/" class="brand">Jot.</a>
			<p class="tagline">Welcome back</p>
		</div>

		{#if error}
			<div class="error-banner">
				<span class="error-icon">!</span>
				{error}
			</div>
		{/if}

		<form class="login-form" on:submit={handleLogin}>
			<div class="field">
				<label for="email">Email</label>
				<input
					id="email"
					type="email"
					autocomplete="email"
					placeholder="you@example.com"
					bind:value={email}
					required
					disabled={loading}
				/>
			</div>

			<div class="field">
				<label for="password">Password</label>
				<div class="password-wrap">
					<input
						id="password"
						type={showPassword ? 'text' : 'password'}
						autocomplete="current-password"
						placeholder="••••••••"
						bind:value={password}
						required
						disabled={loading}
					/>
					<button
						type="button"
						class="toggle-pw"
						tabindex="-1"
						on:click={() => (showPassword = !showPassword)}
						aria-label={showPassword ? 'Hide password' : 'Show password'}
					>
						{showPassword ? 'Hide' : 'Show'}
					</button>
				</div>
			</div>

			<button type="submit" class="submit-btn" disabled={loading}>
				{#if loading}
					<span class="btn-spinner"></span>
					Signing in…
				{:else}
					Sign in →
				{/if}
			</button>
		</form>

		<div class="divider-row">
			<span class="divider-line"></span>
			<span class="divider-text">or</span>
			<span class="divider-line"></span>
		</div>

		<button type="button" class="social-btn" on:click={() => (window.location.href = `${apiUrl}/v1/auth/github`)}>
			<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
			Continue with GitHub
		</button>

		<button type="button" class="social-btn" on:click={() => (window.location.href = `${apiUrl}/v1/auth/google`)}>
			<svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg>
			Continue with Google
		</button>

		<p class="footer-text">
			Don't have an account? <a href="/signup" class="footer-link">Sign up →</a>
		</p>
	</div>

	<p class="bottom-note">
		Built for writers, thinkers, and makers.
	</p>
</div>

<style>
	:global(body) {
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	.login-page {
		min-height: 100dvh;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 40px 20px;
		position: relative;
		overflow: hidden;
	}

	/* ---- decorative grid ---- */
	.grid-bg {
		position: fixed;
		inset: 0;
		pointer-events: none;
		z-index: 0;
	}

	.grid-line-h {
		position: absolute;
		left: 0;
		right: 0;
		height: 1px;
		background: #e0dfdc;
	}

	.grid-line-h:nth-child(1) { top: 16.66%; }
	.grid-line-h:nth-child(2) { top: 33.33%; }
	.grid-line-h:nth-child(3) { top: 50%; }
	.grid-line-h:nth-child(4) { top: 66.66%; }
	.grid-line-h:nth-child(5) { top: 83.33%; }
	.grid-line-h:nth-child(6) { top: 100%; }

	.grid-line-v {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 1px;
		background: #e0dfdc;
	}

	.grid-line-v:nth-child(7)  { left: 16.66%; }
	.grid-line-v:nth-child(8)  { left: 33.33%; }
	.grid-line-v:nth-child(9)  { left: 50%; }
	.grid-line-v:nth-child(10) { left: 66.66%; }
	.grid-line-v:nth-child(11) { left: 83.33%; }
	.grid-line-v:nth-child(12) { left: 100%; }

	/* ---- floating shapes ---- */
	.shape {
		position: fixed;
		font-size: 80px;
		color: #e0dfdc;
		pointer-events: none;
		z-index: 0;
		user-select: none;
		animation: float 8s ease-in-out infinite;
	}

	.shape-1 {
		top: 12%;
		left: 8%;
		font-size: 90px;
		animation-delay: 0s;
	}

	.shape-2 {
		top: 68%;
		right: 10%;
		font-size: 72px;
		animation-delay: -3s;
	}

	.shape-3 {
		bottom: 15%;
		left: 14%;
		font-size: 64px;
		animation-delay: -5s;
	}

	@keyframes float {
		0%, 100% { transform: translateY(0) rotate(0deg); }
		50%      { transform: translateY(-18px) rotate(6deg); }
	}

	/* ---- card ---- */
	.login-card {
		position: relative;
		z-index: 1;
		width: 100%;
		max-width: 400px;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 12px;
		padding: 40px 36px 36px;
		box-shadow: 8px 8px 0 #1a1a1a;
		transition: box-shadow 0.2s ease, transform 0.2s ease;
	}

	.login-card:hover {
		transform: translateY(-2px);
		box-shadow: 10px 10px 0 #1a1a1a;
	}

	.card-header {
		text-align: center;
		margin-bottom: 32px;
	}

	.brand {
		font-size: 36px;
		font-weight: 900;
		color: #1a1a1a;
		text-decoration: none;
		letter-spacing: -0.04em;
		display: inline-block;
		transition: opacity 0.15s;
	}

	.brand:hover {
		opacity: 0.6;
	}

	.tagline {
		font-size: 15px;
		color: #888;
		margin: 6px 0 0;
		font-weight: 500;
	}

	/* ---- error ---- */
	.error-banner {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		background: #fff5f5;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		font-size: 13px;
		font-weight: 600;
		color: #c00;
		margin-bottom: 20px;
	}

	.error-icon {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: #c00;
		color: #fff;
		font-size: 12px;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	/* ---- form ---- */
	.login-form {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.field label {
		font-size: 12px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: #555;
	}

	.field input {
		width: 100%;
		padding: 12px 14px;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		background: #fff;
		font-size: 15px;
		font-family: inherit;
		color: #1a1a1a;
		outline: none;
		transition: border-color 0.15s, box-shadow 0.15s;
		box-sizing: border-box;
	}

	.field input::placeholder {
		color: #bbb;
	}

	.field input:focus {
		border-color: #1a1a1a;
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.field input:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.password-wrap {
		position: relative;
	}

	.password-wrap input {
		padding-right: 60px;
	}

	.toggle-pw {
		position: absolute;
		right: 2px;
		top: 2px;
		bottom: 2px;
		padding: 0 12px;
		border: none;
		background: transparent;
		color: #888;
		font-size: 12px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		cursor: pointer;
		transition: color 0.15s;
	}

	.toggle-pw:hover {
		color: #1a1a1a;
	}

	/* ---- submit ---- */
	.submit-btn {
		width: 100%;
		padding: 14px 20px;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		background: #1a1a1a;
		color: #fff;
		font-size: 15px;
		font-weight: 700;
		font-family: inherit;
		cursor: pointer;
		transition: background 0.15s, box-shadow 0.15s, transform 0.15s;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		margin-top: 4px;
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.submit-btn:hover:not(:disabled) {
		background: #333;
		transform: translateY(-1px);
		box-shadow: 5px 5px 0 #1a1a1a;
	}

	.submit-btn:active:not(:disabled) {
		transform: translateY(1px);
		box-shadow: 2px 2px 0 #1a1a1a;
	}

	.submit-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-spinner {
		width: 16px;
		height: 16px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: #fff;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	/* ---- divider ---- */
	.divider-row {
		display: flex;
		align-items: center;
		gap: 14px;
		margin: 24px 0;
	}

	.divider-line {
		flex: 1;
		height: 1px;
		background: #e0dfdc;
	}

	.divider-text {
		font-size: 12px;
		font-weight: 600;
		color: #bbb;
		text-transform: uppercase;
		letter-spacing: 0.1em;
	}

	/* ---- social ---- */
	.social-btn {
		width: 100%;
		padding: 12px 20px;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		background: #fff;
		color: #1a1a1a;
		font-size: 14px;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		transition: background 0.15s, box-shadow 0.15s, transform 0.15s;
	}

	.social-btn:hover {
		background: #f5f5f3;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.social-btn:active {
		transform: translateY(0);
		box-shadow: none;
	}

	/* ---- footer ---- */
	.footer-text {
		text-align: center;
		font-size: 13px;
		color: #888;
		margin: 24px 0 0;
		font-weight: 500;
	}

	.footer-link {
		color: #1a1a1a;
		font-weight: 700;
		text-decoration: none;
		border-bottom: 2px solid #1a1a1a;
		padding-bottom: 1px;
		transition: opacity 0.15s;
	}

	.footer-link:hover {
		opacity: 0.7;
	}

	.bottom-note {
		position: relative;
		z-index: 1;
		margin-top: 40px;
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.14em;
		color: #bbb;
	}

	/* ---- responsive ---- */
	@media (max-width: 480px) {
		.login-card {
			padding: 32px 24px 28px;
			box-shadow: 6px 6px 0 #1a1a1a;
		}

		.login-card:hover {
			box-shadow: 6px 6px 0 #1a1a1a;
			transform: none;
		}

		.brand {
			font-size: 30px;
		}

		.shape {
			display: none;
		}
	}
</style>
