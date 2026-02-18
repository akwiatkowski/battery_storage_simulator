<script lang="ts">
	import { helpTexts } from '$lib/help-texts';

	interface Props {
		key: string;
	}

	let { key }: Props = $props();
	let open = $state(false);

	let entry = $derived(helpTexts[key]);

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			open = false;
		}
	}
</script>

<svelte:window onkeydown={open ? handleKeydown : undefined} />

{#if entry}
	<button class="help-btn" onclick={() => (open = true)} aria-label="Help: {entry.title}">?</button>
{/if}

{#if open && entry}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="backdrop" onclick={() => (open = false)} onkeydown={handleKeydown}>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
			<button class="close-btn" onclick={() => (open = false)} aria-label="Close">&times;</button>
			<h3 class="modal-title">{entry.title}</h3>
			<p class="modal-body">{entry.description}</p>
			{#if entry.formula}
				<div class="modal-section">
					<span class="modal-section-title">How it's calculated</span>
					<p class="modal-body formula">{entry.formula}</p>
				</div>
			{/if}
			{#if entry.example}
				<div class="modal-section">
					<span class="modal-section-title">Example</span>
					<p class="modal-body">{entry.example}</p>
				</div>
			{/if}
			{#if entry.insight}
				<div class="modal-section">
					<span class="modal-section-title">Good to know</span>
					<p class="modal-body">{entry.insight}</p>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.help-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		border: 1px solid #e8ecf1;
		background: transparent;
		font-size: 10px;
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		color: #94a3b8;
		cursor: pointer;
		vertical-align: middle;
		line-height: 1;
		padding: 0;
		text-transform: none;
		letter-spacing: 0;
		flex-shrink: 0;
	}

	.help-btn:hover {
		border-color: #94a3b8;
		color: #64748b;
	}

	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.25);
		z-index: 1000;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.modal {
		position: relative;
		background: #fff;
		max-width: 420px;
		width: calc(100% - 32px);
		border: 1px solid #e8ecf1;
		border-radius: 14px;
		padding: 24px;
		box-shadow: 0 4px 24px rgba(0, 0, 0, 0.1);
	}

	.close-btn {
		position: absolute;
		top: 12px;
		right: 12px;
		width: 24px;
		height: 24px;
		display: flex;
		align-items: center;
		justify-content: center;
		border: none;
		background: transparent;
		font-size: 18px;
		color: #94a3b8;
		cursor: pointer;
		padding: 0;
		line-height: 1;
	}

	.close-btn:hover {
		color: #475569;
	}

	.modal-title {
		font-size: 16px;
		font-weight: 600;
		color: #222;
		margin: 0 0 12px 0;
		padding-right: 24px;
	}

	.modal-body {
		font-size: 14px;
		color: #475569;
		line-height: 1.5;
		margin: 0;
	}

	.modal-body.formula {
		font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
		font-size: 13px;
		background: #f8fafb;
		padding: 8px 10px;
		border-radius: 6px;
	}

	.modal-section {
		margin-top: 12px;
	}

	.modal-section-title {
		display: block;
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #94a3b8;
		margin-bottom: 4px;
	}
</style>
