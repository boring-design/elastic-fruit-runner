// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Google Analytics 4: property elastic-fruit-runner.pages.dev (Boring Design account).
const gaMeasurementId = 'G-L3KHNMZBD4';
// PostHog project "elastic-fruit-runner docs" (703388); the project key is public.
const posthogKey = 'phc_mXHX3Vw6EoYvWzm2oRpxj2aTUU9aTqfuPDqXtZzqgumV';
const posthogHost = 'https://us.i.posthog.com';

// https://astro.build/config
export default defineConfig({
	site: 'https://elastic-fruit-runner.boringboring.design',
	integrations: [
		starlight({
			title: 'Elastic Fruit Runner',
			head: [
				{
					tag: 'script',
					attrs: { async: true, src: `https://www.googletagmanager.com/gtag/js?id=${gaMeasurementId}` },
				},
				{
					tag: 'script',
					content: `window.dataLayer = window.dataLayer || [];
function gtag(){dataLayer.push(arguments);}
gtag('js', new Date());
gtag('config', '${gaMeasurementId}');`,
				},
				{
					tag: 'script',
					content: `!function(t,e){var o,n,p,r;e.__SV||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]),t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}(p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host.replace(".i.posthog.com","-assets.i.posthog.com")+"/static/array.js",(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r);var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="capture identify alias people.set people.set_once set_config register register_once unregister opt_out_capturing has_opted_out_capturing opt_in_capturing reset isFeatureEnabled onFeatureFlags getFeatureFlag getFeatureFlagPayload reloadFeatureFlags group updateEarlyAccessFeatureEnrollment getEarlyAccessFeatures getActiveMatchingSurveys getSurveys onSessionId".split(" "),n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);
posthog.init('${posthogKey}', { api_host: '${posthogHost}', person_profiles: 'identified_only' });`,
				},
				{
					// The "AI Agents" prompt: count copies and clicks of the skill.md line.
					tag: 'script',
					content: `(function(){
function track(how){
  try { window.posthog && window.posthog.capture('ai_agents_prompt', { how: how }); } catch (e) {}
  try { window.gtag && window.gtag('event', 'ai_agents_prompt', { how: how }); } catch (e) {}
}
document.addEventListener('copy', function(){
  var text = String(window.getSelection && window.getSelection());
  if (text.indexOf('skill.md') !== -1) track('copy');
});
document.addEventListener('click', function(e){
  var el = e.target && e.target.closest && e.target.closest('a[href$="/skill.md"], button[data-code*="skill.md"]');
  if (el) track(el.tagName === 'A' ? 'link' : 'copy_button');
});
})();`,
				},
			],
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
						{ slug: 'how-to/install-gvisor-ubuntu' },
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
						{ slug: 'how-to/cloud-mode' },
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
