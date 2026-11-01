// Serves /skill.md and records the fetch in PostHog. Agents read this file
// without running page JavaScript, so the page analytics never see them.
const POSTHOG_KEY = 'phc_mXHX3Vw6EoYvWzm2oRpxj2aTUU9aTqfuPDqXtZzqgumV';
const POSTHOG_CAPTURE = 'https://us.i.posthog.com/i/v0/e/';

export async function onRequestGet(context) {
  const response = await context.env.ASSETS.fetch(context.request);
  const request = context.request;
  const event = {
    api_key: POSTHOG_KEY,
    event: 'skill_md_fetched',
    distinct_id: crypto.randomUUID(),
    properties: {
      $process_person_profile: false,
      user_agent: request.headers.get('user-agent') || '',
      referer: request.headers.get('referer') || '',
      country: (request.cf && request.cf.country) || '',
      status: response.status,
    },
  };
  context.waitUntil(
    fetch(POSTHOG_CAPTURE, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(event),
    }).catch(() => {}),
  );
  return response;
}
