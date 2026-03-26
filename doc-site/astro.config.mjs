// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	integrations: [
		starlight({
			title: 'Elastic Fruit Runner',
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/boring-design/elastic-fruit-runner',
				},
			],
			sidebar: [
				{
					label: 'Tutorials',
					items: [
						{ slug: 'tutorials/getting-started' },
					],
				},
				{
					label: 'How-to Guides',
					items: [
						{ slug: 'how-to/install-macos' },
						{ slug: 'how-to/install-linux-docker' },
						{ slug: 'how-to/configure-github-app' },
						{ slug: 'how-to/multiple-orgs-repos' },
						{ slug: 'how-to/troubleshooting' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ slug: 'reference/configuration' },
						{ slug: 'reference/cli' },
						{ slug: 'reference/environment-variables' },
					],
				},
				{
					label: 'Explanation',
					items: [
						{ slug: 'explanation/what-is-efr' },
					],
				},
			],
		}),
	],
});
