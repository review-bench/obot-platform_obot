import type { ClientConfig, ProviderShortKey, RenderContext, SnippetBlock } from './types';

function loginSubstitution(obotURL: string): string {
	return `$(obot login --url ${obotURL} --print-token)`;
}

export const claudeCodeClient: ClientConfig = {
	id: 'claude-code',
	label: 'Claude Code',
	render(ctx: RenderContext): SnippetBlock[] {
		const code = [
			`export ANTHROPIC_BASE_URL="${ctx.baseURL}"`,
			`export ANTHROPIC_API_KEY="${loginSubstitution(ctx.obotURL)}"`,
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
		const config = {
			$schema: 'https://opencode.ai/config.json',
			provider: {
				[ctx.provider.shortKey]: {
					options: {
						baseURL: ctx.baseURL
					}
				}
			}
		};

		const code = [
			"cat > opencode.json <<'EOF'",
			JSON.stringify(config, null, 2),
			'EOF',
			'',
			`export ${ctx.provider.envKey}="${loginSubstitution(ctx.obotURL)}"`
		].join('\n');

		return [{ language: 'bash', code }];
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
				`export ANTHROPIC_API_KEY="${loginSubstitution(ctx.obotURL)}"`,
				'',
				`curl ${ctx.baseURL}/messages \\`,
				`  -H "x-api-key: $ANTHROPIC_API_KEY" \\`,
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
			`export OPENAI_API_KEY="${loginSubstitution(ctx.obotURL)}"`,
			'',
			`curl ${ctx.baseURL}/chat/completions \\`,
			`  -H "Authorization: Bearer $OPENAI_API_KEY" \\`,
			`  -H "Content-Type: application/json" \\`,
			`  -d '${body}'`
		].join('\n');
		return [{ language: 'bash', code }];
	}
};

export const clientsByProvider: Record<ProviderShortKey, ClientConfig[]> = {
	anthropic: [claudeCodeClient, openCodeClient, curlClient],
	openai: [openCodeClient, curlClient]
};
