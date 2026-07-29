# Elastic Fruit Runner documentation

The user documentation uses the [Divio documentation system](https://docs.divio.com/documentation-system/). Keep each page focused on one type.

## Page types

* `tutorials/` helps a new user learn by reaching a reliable result.
* `how-to/` gives the steps for one clear task.
* `reference/` lists current behavior, fields, commands, states, and limits.
* `explanation/` gives background, reasons, and tradeoffs.

Development pages belong in the Development sidebar group. Do not mix internal test or release work into the user journey.

## Files and routes

Pages live under `src/content/docs/`.

Use lowercase file names with words separated by hyphens. The file path becomes the public route. Do not rename an existing public page without keeping its route available.

Put static images in `public/`. Use only test data. Remove tokens, private repository names, private key paths, personal names, and other private data. Give every image clear alt text.

Use English and common words. Keep commands ready to copy.

## Source of truth

Reference pages must match the current code.

* Config fields and validation come from `config/`.
* Commands and startup behavior come from `cmd/elastic-fruit-runner/`.
* Console behavior comes from `dashboard/src/` and `internal/api/`.
* Job history, host history, and cleanup rules come from `internal/management/`.
* Message fields and states come from `proto/controlplane/`.

Do not describe the Connect RPC service as a stable public API.

## Local checks

Run these commands from the repository root:

```sh
pnpm --dir doc-site install --frozen-lockfile
pnpm --dir doc-site build
make check
```

Before a commit, also run:

```sh
prek run --all-files
```

Check changed pages in a desktop browser and at 390px width. Check both light and dark themes when an image changes.
