<script lang="ts">
	import type { ClientConfig, RenderContext } from '$lib/services/llm-gateway/types';
	import LLMGatewayCodeBlock from './LLMGatewayCodeBlock.svelte';
	import { twMerge } from 'tailwind-merge';

	interface Props {
		clients: ClientConfig[];
		ctx: RenderContext;
	}

	let { clients, ctx }: Props = $props();

	let activeClientId = $state<string | undefined>(undefined);
	let activeClient = $derived(clients.find((c) => c.id === activeClientId) ?? clients[0]);
	let blocks = $derived(activeClient ? activeClient.render(ctx) : []);
</script>

{#if clients.length > 0}
	<div class="flex flex-col gap-3">
		<div class="border-base-300 dark:border-base-400 flex items-center gap-1 border-b">
			{#each clients as client (client.id)}
				<button
					type="button"
					class={twMerge(
						'-mb-px border-b-2 border-transparent px-3 py-2 text-sm transition-colors',
						activeClientId === client.id
							? 'border-primary text-primary font-medium'
							: 'text-muted-content hover:text-base-content'
					)}
					onclick={() => (activeClientId = client.id)}
				>
					{client.label}
				</button>
			{/each}
		</div>

		<div class="flex flex-col gap-3">
			{#each blocks as block, i (i)}
				<LLMGatewayCodeBlock {block} />
			{/each}
		</div>
	</div>
{/if}
