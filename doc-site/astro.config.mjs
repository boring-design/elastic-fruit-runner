// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://elastic-fruit-runner.boringboring.design',
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
						{ slug: 'how-to/use-console' },
						{ slug: 'how-to/set-up-console' },
						{ slug: 'how-to/reset-console-password' },
						{ slug: 'how-to/investigate-jobs' },
						{ slug: 'how-to/check-runner-capacity' },
						{ slug: 'how-to/monitor-host-resources' },
						{ slug: 'how-to/edit-config' },
						{ slug: 'how-to/recover-config' },
						{ slug: 'how-to/multiple-orgs-repos' },
						{ slug: 'how-to/prevent-macos-sleep' },
						{ slug: 'how-to/upgrade' },
						{ slug: 'how-to/troubleshooting' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ slug: 'reference/configuration' },
						{ slug: 'reference/cli' },
						{ slug: 'reference/environment-variables' },
						{ slug: 'reference/console' },
						{ slug: 'reference/history-and-storage' },
					],
				},
				{
					label: 'Explanation',
					items: [
						{ slug: 'explanation/what-is-elastic-fruit-runner' },
						{ slug: 'explanation/runner-lifecycle' },
						{ slug: 'explanation/console-design' },
					],
				},
				{
					label: 'Development',
					items: [
						{ slug: 'how-to/run-integration-tests' },
					],
				},
			],
		}),
	],
});
