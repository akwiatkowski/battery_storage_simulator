import eslintPluginSvelte from 'eslint-plugin-svelte';
import eslintConfigPrettier from 'eslint-config-prettier';

export default [
	...eslintPluginSvelte.configs['flat/recommended'],
	eslintConfigPrettier,
	{
		ignores: ['build/', '.svelte-kit/', 'node_modules/']
	}
];
