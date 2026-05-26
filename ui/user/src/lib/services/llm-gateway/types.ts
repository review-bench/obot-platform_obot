import { CommonModelProviderIds } from '$lib/constants';
import type { Model } from '$lib/services';

export type ProviderShortKey = 'openai' | 'anthropic';

/** Env var the snippets export and reference via OpenCode's `{env:...}` substitution. */
export const OBOT_API_KEY_ENV = 'OBOT_API_KEY';

export interface ProviderConnection {
	id: string;
	shortKey: ProviderShortKey;
	envKey: 'OPENAI_API_KEY' | 'ANTHROPIC_API_KEY';
	displayName: string;
	/** Custom OpenCode provider id used in opencode.json (`provider.<id>`). */
	openCodeProviderId: string;
	/** npm package the custom OpenCode provider should load. */
	openCodeNpm: string;
	/** Human-readable name for OpenCode's UI. */
	openCodeProviderName: string;
}

export const PROVIDER_CONNECTIONS: Record<ProviderShortKey, ProviderConnection> = {
	openai: {
		id: CommonModelProviderIds.OPENAI,
		shortKey: 'openai',
		envKey: 'OPENAI_API_KEY',
		displayName: 'OpenAI',
		openCodeProviderId: 'obot-openai',
		openCodeNpm: '@ai-sdk/openai',
		openCodeProviderName: 'Obot Gateway OpenAI'
	},
	anthropic: {
		id: CommonModelProviderIds.ANTHROPIC,
		shortKey: 'anthropic',
		envKey: 'ANTHROPIC_API_KEY',
		displayName: 'Anthropic',
		openCodeProviderId: 'obot-anthropic',
		openCodeNpm: '@ai-sdk/anthropic',
		openCodeProviderName: 'Obot Gateway Anthropic'
	}
};

export const SUPPORTED_PROVIDER_IDS = new Set<string>([
	CommonModelProviderIds.OPENAI,
	CommonModelProviderIds.ANTHROPIC
]);

export interface RenderContext {
	provider: ProviderConnection;
	/** e.g. https://obot.example.com */
	obotURL: string;
	/** e.g. https://obot.example.com/api/llm-proxy/anthropic */
	baseURL: string;
	/** Active models the user has access to, used to populate client model maps. */
	models: Model[];
	/** First available model name for the provider, used in example invocations. */
	exampleModel?: string;
}

export interface SnippetBlock {
	/** Optional label rendered above the code block, e.g. 'opencode.json'. */
	title?: string;
	language: 'bash' | 'json' | 'toml';
	code: string;
}

export interface ClientConfig {
	id: string;
	label: string;
	render(ctx: RenderContext): SnippetBlock[];
}
