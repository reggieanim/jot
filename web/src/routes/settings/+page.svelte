<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { env } from '$env/dynamic/public';
	import { user as authUser, fetchMe } from '$lib/stores/auth';

	const apiUrl = env.PUBLIC_API_URL || 'http://localhost:8080';

	let displayName = '';
	let username = '';
	let bio = '';
	let avatarUrl = '';
	let avatarFile: File | null = null;
	let avatarPreview = '';

	let saving = false;
	let error = '';
	let success = '';

	onMount(async () => {
		const me = await fetchMe();
		if (!me) {
			goto('/login');
			return;
		}
		displayName = me.display_name || '';
		username = me.username || '';
		bio = me.bio || '';
		avatarUrl = me.avatar_url || '';
		avatarPreview = avatarUrl;
	});

	function handleAvatarSelect(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		avatarFile = file;
		avatarPreview = URL.createObjectURL(file);
	}

	function removeAvatar() {
		avatarFile = null;
		avatarPreview = '';
		avatarUrl = '';
	}

	async function uploadAvatar(): Promise<string> {
		if (!avatarFile) return avatarUrl;
		const formData = new FormData();
		formData.append('file', avatarFile);
		const res = await fetch(`${apiUrl}/v1/media/images`, {
			method: 'POST',
			credentials: 'include',
			body: formData
		});
		if (!res.ok) throw new Error('Failed to upload image');
		const data = await res.json();
		return data.url;
	}

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		error = '';
		success = '';

		try {
			// upload avatar if a new file was selected
			let finalAvatarUrl = avatarUrl;
			if (avatarFile) {
				finalAvatarUrl = await uploadAvatar();
			}

			const res = await fetch(`${apiUrl}/v1/auth/me`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				credentials: 'include',
				body: JSON.stringify({
					display_name: displayName.trim(),
					bio: bio.trim(),
					avatar_url: finalAvatarUrl
				})
			});

			if (!res.ok) {
				const body = await res.json().catch(() => null);
				throw new Error(body?.error || 'Failed to update profile');
			}

			avatarUrl = finalAvatarUrl;
			avatarFile = null;

			// refresh auth store
			await fetchMe();
			success = 'Profile updated!';
			setTimeout(() => (success = ''), 3000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Settings — Jot.</title>
</svelte:head>

<div class="settings-page">
	<!-- decorative grid -->
	<div class="grid-bg" aria-hidden="true">
		{#each Array(6) as _}
			<div class="grid-line-h"></div>
		{/each}
		{#each Array(6) as _}
			<div class="grid-line-v"></div>
		{/each}
	</div>

	<div class="settings-card">
		<div class="card-header">
			<a href="/" class="brand">Jot.</a>
			<h1 class="card-title">Edit Profile</h1>
			<p class="card-subtitle">Update your public profile information</p>
		</div>

		{#if error}
			<div class="error-banner">
				<span class="error-icon">!</span>
				<span>{error}</span>
			</div>
		{/if}

		{#if success}
			<div class="success-banner">
				<span>✓</span>
				<span>{success}</span>
			</div>
		{/if}

		<form on:submit={handleSave}>
			<!-- AVATAR -->
			<div class="avatar-section">
				<div class="avatar-preview">
					{#if avatarPreview}
						<img src={avatarPreview} alt="Avatar preview" />
					{:else}
						<span class="avatar-letter">{(displayName || username || '?').charAt(0).toUpperCase()}</span>
					{/if}
				</div>
				<div class="avatar-actions">
					<label class="avatar-upload-btn">
						<input type="file" accept="image/*" on:change={handleAvatarSelect} hidden />
						{avatarPreview ? 'Change photo' : 'Upload photo'}
					</label>
					{#if avatarPreview}
						<button type="button" class="avatar-remove-btn" on:click={removeAvatar}>Remove</button>
					{/if}
				</div>
			</div>

			<!-- DISPLAY NAME -->
			<div class="field">
				<label for="displayName" class="field-label">Display Name</label>
				<input
					id="displayName"
					type="text"
					class="field-input"
					placeholder="Your name"
					bind:value={displayName}
					maxlength="50"
				/>
			</div>

			<!-- USERNAME (read-only) -->
			<div class="field">
				<label for="username" class="field-label">Username</label>
				<div class="field-input field-readonly">@{username}</div>
				<span class="field-hint">Username cannot be changed yet</span>
			</div>

			<!-- BIO -->
			<div class="field">
				<label for="bio" class="field-label">Bio</label>
				<textarea
					id="bio"
					class="field-input field-textarea"
					placeholder="A short bio about yourself"
					bind:value={bio}
					maxlength="200"
					rows="3"
				></textarea>
				<span class="field-hint">{bio.length}/200</span>
			</div>

			<!-- ACTIONS -->
			<div class="form-actions">
				<a href="/" class="cancel-btn">Cancel</a>
				<button type="submit" class="save-btn" disabled={saving}>
					{#if saving}
						<span class="btn-spinner"></span> Saving…
					{:else}
						Save changes
					{/if}
				</button>
			</div>
		</form>

		<!-- VIEW PROFILE LINK -->
		{#if $authUser?.username}
			<div class="profile-link-wrap">
				<a href={`/user/${$authUser.username}`} class="profile-link">View your public profile →</a>
			</div>
		{/if}
	</div>
</div>

<style>
	:global(body) {
		margin: 0;
		background: #f5f5f3;
		color: #1a1a1a;
	}

	.settings-page {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 40px 20px;
		position: relative;
		overflow: hidden;
	}

	/* ---- GRID BG ---- */
	.grid-bg {
		position: absolute;
		inset: 0;
		pointer-events: none;
		z-index: 0;
	}

	.grid-line-h,
	.grid-line-v {
		position: absolute;
		background: #e0dfdc;
	}

	.grid-line-h {
		width: 100%;
		height: 1px;
	}
	.grid-line-h:nth-child(1) { top: 16%; }
	.grid-line-h:nth-child(2) { top: 33%; }
	.grid-line-h:nth-child(3) { top: 50%; }
	.grid-line-h:nth-child(4) { top: 66%; }
	.grid-line-h:nth-child(5) { top: 83%; }
	.grid-line-h:nth-child(6) { top: 95%; }

	.grid-line-v {
		height: 100%;
		width: 1px;
	}
	.grid-line-v:nth-child(7) { left: 10%; }
	.grid-line-v:nth-child(8) { left: 25%; }
	.grid-line-v:nth-child(9) { left: 50%; }
	.grid-line-v:nth-child(10) { left: 75%; }
	.grid-line-v:nth-child(11) { left: 90%; }
	.grid-line-v:nth-child(12) { left: 95%; }

	/* ---- CARD ---- */
	.settings-card {
		position: relative;
		z-index: 1;
		width: 100%;
		max-width: 480px;
		background: #fff;
		border: 2px solid #1a1a1a;
		border-radius: 12px;
		padding: 40px 36px;
		box-shadow: 6px 6px 0 #1a1a1a;
	}

	.card-header {
		text-align: center;
		margin-bottom: 28px;
	}

	.brand {
		font-size: 1.6rem;
		font-weight: 900;
		color: #1a1a1a;
		text-decoration: none;
		letter-spacing: -0.04em;
	}

	.card-title {
		font-size: 22px;
		font-weight: 900;
		letter-spacing: -0.03em;
		margin: 12px 0 4px;
	}

	.card-subtitle {
		font-size: 13px;
		color: #888;
		margin: 0;
	}

	/* ---- BANNERS ---- */
	.error-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		background: #fff0f0;
		border: 2px solid #c00;
		border-radius: 6px;
		padding: 10px 14px;
		margin-bottom: 20px;
		font-size: 13px;
		font-weight: 600;
		color: #c00;
	}

	.error-icon {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: #c00;
		color: #fff;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 12px;
		font-weight: 900;
		flex-shrink: 0;
	}

	.success-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		background: #f0fff4;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		padding: 10px 14px;
		margin-bottom: 20px;
		font-size: 13px;
		font-weight: 600;
		color: #1a1a1a;
	}

	/* ---- AVATAR SECTION ---- */
	.avatar-section {
		display: flex;
		align-items: center;
		gap: 20px;
		margin-bottom: 24px;
		padding-bottom: 24px;
		border-bottom: 1px solid #e0dfdc;
	}

	.avatar-preview {
		width: 80px;
		height: 80px;
		border-radius: 50%;
		border: 2px solid #1a1a1a;
		background: #1a1a1a;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		flex-shrink: 0;
	}

	.avatar-preview img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.avatar-letter {
		font-size: 30px;
		font-weight: 900;
		color: #fff;
		text-transform: uppercase;
	}

	.avatar-actions {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.avatar-upload-btn {
		font-family: inherit;
		font-size: 13px;
		font-weight: 700;
		color: #1a1a1a;
		background: #f5f5f3;
		border: 2px solid #1a1a1a;
		padding: 6px 16px;
		cursor: pointer;
		transition: background 0.15s, transform 0.15s, box-shadow 0.15s;
		text-align: center;
	}

	.avatar-upload-btn:hover {
		background: #e8e8e4;
		transform: translateY(-1px);
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.avatar-remove-btn {
		font-family: inherit;
		font-size: 12px;
		font-weight: 600;
		color: #888;
		background: none;
		border: none;
		padding: 4px 0;
		cursor: pointer;
		text-align: left;
		transition: color 0.15s;
	}

	.avatar-remove-btn:hover {
		color: #c00;
	}

	/* ---- FORM FIELDS ---- */
	.field {
		margin-bottom: 20px;
	}

	.field-label {
		display: block;
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: #1a1a1a;
		margin-bottom: 6px;
	}

	.field-input {
		width: 100%;
		padding: 10px 14px;
		font-family: inherit;
		font-size: 14px;
		border: 2px solid #1a1a1a;
		border-radius: 6px;
		background: #fff;
		color: #1a1a1a;
		outline: none;
		transition: border-color 0.15s, box-shadow 0.15s;
		box-sizing: border-box;
	}

	.field-input:focus {
		border-color: #1a1a1a;
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.field-readonly {
		background: #f5f5f3;
		color: #888;
		cursor: default;
		display: flex;
		align-items: center;
	}

	.field-textarea {
		resize: vertical;
		min-height: 60px;
		line-height: 1.5;
	}

	.field-hint {
		display: block;
		font-size: 11px;
		color: #aaa;
		margin-top: 4px;
	}

	/* ---- ACTIONS ---- */
	.form-actions {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 12px;
		margin-top: 28px;
		padding-top: 20px;
		border-top: 1px solid #e0dfdc;
	}

	.cancel-btn {
		font-family: inherit;
		font-size: 13px;
		font-weight: 600;
		color: #888;
		text-decoration: none;
		padding: 9px 18px;
		transition: color 0.15s;
	}

	.cancel-btn:hover {
		color: #1a1a1a;
	}

	.save-btn {
		font-family: inherit;
		font-size: 13px;
		font-weight: 700;
		color: #fff;
		background: #1a1a1a;
		border: 2px solid #1a1a1a;
		padding: 9px 24px;
		cursor: pointer;
		box-shadow: 3px 3px 0 #1a1a1a;
		transition: background 0.15s, transform 0.15s, box-shadow 0.15s;
		display: flex;
		align-items: center;
		gap: 6px;
		letter-spacing: 0.02em;
	}

	.save-btn:hover {
		background: #333;
		transform: translateY(-1px);
		box-shadow: 4px 4px 0 #1a1a1a;
	}

	.save-btn:disabled {
		opacity: 0.5;
		cursor: default;
		transform: none;
		box-shadow: 3px 3px 0 #1a1a1a;
	}

	.btn-spinner {
		width: 14px;
		height: 14px;
		border: 2px solid rgba(255,255,255,0.3);
		border-top-color: #fff;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
		display: inline-block;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	/* ---- PROFILE LINK ---- */
	.profile-link-wrap {
		text-align: center;
		margin-top: 24px;
		padding-top: 20px;
		border-top: 1px solid #e0dfdc;
	}

	.profile-link {
		font-size: 13px;
		font-weight: 600;
		color: #888;
		text-decoration: none;
		transition: color 0.15s;
	}

	.profile-link:hover {
		color: #1a1a1a;
	}

	/* ---- RESPONSIVE ---- */
	@media (max-width: 560px) {
		.settings-card {
			padding: 28px 20px;
		}

		.avatar-section {
			flex-direction: column;
			align-items: flex-start;
		}
	}
</style>
