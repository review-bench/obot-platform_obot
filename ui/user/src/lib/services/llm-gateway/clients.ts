import type { Model } from '$lib/services';
import {
	OBOT_API_KEY_ENV,
	type ClientConfig,
	type ProviderShortKey,
	type RenderContext,
	type SnippetBlock
} from './types';

function loginSubstitution(obotURL: string): string {
	return `$(obot login --url ${obotURL} --print-token)`;
}

function exportObotApiKey(obotURL: string): string {
	return `export ${OBOT_API_KEY_ENV}="${loginSubstitution(obotURL)}"`;
}

function buildModelsMap(models: Model[]): Record<string, { name: string }> {
	const sorted = [...models].sort((a, b) =>
		(a.displayName || a.name).localeCompare(b.displayName || b.name)
	);
	const out: Record<string, { name: string }> = {};
	for (const m of sorted) {
		out[m.name] = { name: m.displayName || m.name };
	}
	return out;
}

export const claudeCodeClient: ClientConfig = {
	id: 'claude-code',
	label: 'Claude Code',
	render(ctx: RenderContext): SnippetBlock[] {
		const code = [
			`export ANTHROPIC_BASE_URL="${ctx.baseURL}"`,
			`export ANTHROPIC_API_KEY="${loginSubstitution(ctx.obotURL)}"`,
			'export CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1',
			'',
			'claude'
		].join('\n');
		return [{ language: 'bash', code }];
	}
};

export const openCodeClient: ClientConfig = {
	id: 'opencode',
	label: 'OpenCode',
	render(ctx: RenderContext): SnippetBlock[] {
		const providerEntry: Record<string, unknown> = {
			npm: ctx.provider.openCodeNpm,
			name: ctx.provider.openCodeProviderName,
			options: {
				baseURL: ctx.baseURL,
				apiKey: `{env:${OBOT_API_KEY_ENV}}`
			},
			models: buildModelsMap(ctx.models)
		};

		const config = {
			$schema: 'https://opencode.ai/config.json',
			provider: {
				[ctx.provider.openCodeProviderId]: providerEntry
			}
		};

		// Single bash block: write opencode.json (quoted heredoc so JSON is preserved
		// verbatim, including the literal `$schema` key and `{env:OBOT_API_KEY}`
		// placeholder), then export OBOT_API_KEY so OpenCode can resolve it.
		const code = [
			"cat > opencode.json <<'EOF'",
			JSON.stringify(config, null, 2),
			'EOF',
			'',
			exportObotApiKey(ctx.obotURL)
		].join('\n');

		return [{ language: 'bash', code }];
	}
};

export const codexClient: ClientConfig = {
	id: 'codex',
	label: 'Codex',
	render(ctx: RenderContext): SnippetBlock[] {
		const code = [
			'# Add to ~/.codex/config.toml',
			'',
			'model_provider = "obot_openai"',
			'',
			'[model_providers.obot_openai]',
			'name = "OpenAI Obot LLM Gateway"',
			`base_url = "${ctx.baseURL}/v1"`,
			'wire_api = "responses"',
			'',
			'[model_providers.obot_openai.auth]',
			'command = "/usr/local/bin/obot"',
			`args = ["login", "--print-token", "--url", "${ctx.obotURL}"]`,
			'timeout_ms = 5000',
			'refresh_interval_ms = 300000'
		].join('\n');
		return [{ language: 'toml', code, title: '~/.codex/config.toml' }];
	}
};

export const curlClient: ClientConfig = {
	id: 'curl',
	label: 'Curl',
	render(ctx: RenderContext): SnippetBlock[] {
		const model = ctx.exampleModel ?? '<model-name>';
		if (ctx.provider.shortKey === 'anthropic') {
			const body = JSON.stringify(
				{
					model,
					max_tokens: 1024,
					messages: [{ role: 'user', content: 'hello' }]
				},
				null,
				2
			);
			const code = [
				exportObotApiKey(ctx.obotURL),
				'',
				`curl ${ctx.baseURL}/messages \\`,
				`  -H "x-api-key: $${OBOT_API_KEY_ENV}" \\`,
				`  -H "anthropic-version: 2023-06-01" \\`,
				`  -H "content-type: application/json" \\`,
				`  -d '${body}'`
			].join('\n');
			return [{ language: 'bash', code }];
		}

		// OpenAI
		const body = JSON.stringify(
			{
				model,
				messages: [{ role: 'user', content: 'hello' }]
			},
			null,
			2
		);
		const code = [
			exportObotApiKey(ctx.obotURL),
			'',
			`curl ${ctx.baseURL}/chat/completions \\`,
			`  -H "Authorization: Bearer $${OBOT_API_KEY_ENV}" \\`,
			`  -H "Content-Type: application/json" \\`,
			`  -d '${body}'`
		].join('\n');
		return [{ language: 'bash', code }];
	}
};

export const clientsByProvider: Record<ProviderShortKey, ClientConfig[]> = {
	anthropic: [claudeCodeClient, openCodeClient, curlClient],
	openai: [codexClient, openCodeClient, curlClient]
};
