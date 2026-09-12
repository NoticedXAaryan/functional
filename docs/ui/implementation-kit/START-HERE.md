# Give this file to the implementing AI first

This is a **build kit**, not a deployed implementation. Do not treat pictures, examples or declarative action names as working integrations.

## Exact handoff

Give the AI the repository, not just screenshots. Tell it:

> Read docs/ui/implementation-kit/START-HERE.md. Complete TASKS.md one task at a time. Use the supplied assets, tokens.css, component contracts, screens.json and individual screen recipes. Do not redesign the product or invent missing backend behaviour. Before changing any code, report the deployed commit, existing staged changes and the first task's file list. Preserve those staged changes. Finish one task and update the completion ledger before beginning another. If a required provider or contract is unavailable, record the blocker and keep that capability unavailable; never create fake success data. Keep credentials private. Respect the user's no-testing instruction: write acceptance requirements but do not run application tests or create sample reports. You may build the code and report build results separately from behaviour. Do not claim a feature works without evidence. Do not deploy a broken partial flow or replace the real database with fixtures.

## Required reading order

1. This file and [DECISIONS.md](DECISIONS.md): fixed choices and unresolved dependencies.
2. [TASKS.md](TASKS.md): task order, exact deliverable and stopping conditions.
3. [ASSETS.md](ASSETS.md): filenames, placement, licensing/provenance and asset limitations.
4. [COMPONENTS.md](COMPONENTS.md): component responsibilities and data boundaries.
5. [screens.json](screens.json), then ONLY the individual [screen recipe](screens/) for the task being implemented.
6. Corresponding domain specification in [the parent UI pack](../README.md).
7. Actual deployment strategy, live URLs and current source/migrations. The strategy has not yet been supplied to this documentation pass; request its location if still absent.

## What is already provided

- 24 original SVG interface icons, 12 optional word-picture icons, and an SVG sprite. Picture names are visual resources, not an approved credential dictionary.
- A generated welcoming bridge illustration and simplified app-icon master derived from the supplied logo.
- The unchanged original logo in `../assets/bal-setu-logo-source.jpeg`.
- Scoped CSS tokens and component styles, with a system-font baseline that needs no external font downloads.
- Five reusable React presentation components; they deliberately have no fake backend or auth logic.
- 38 screen recipes, a machine-readable screen catalog, English copy catalog and seven composition boards in both SVG and PNG.
- A static [visual gallery](gallery.html), a deterministic generation script and an asset manifest.
- Per-feature failure rules, integration dependencies, proposed API/data contracts and acceptance requirements in the parent specifications.

## What is not represented as complete

- These files have not been wired into the application. No live sign-in, voice upload or real-device push is demonstrated by the kit.
- Hindi/Gujarati translations, regional wordlists, recorded language prompts and clinical/legal wording have not received human review. Do not activate unfinished language options.
- No actual operator identity, consented child photograph, real jurisdiction map, private audio or functioning QR venue is invented here.
- The app-icon master is raster, not a hand-traced vector logo. The editable SVGs are interface icons. Do not claim an SVG wrapper around the raster makes the logo vector.
- Pixel-perfect layouts for every validation state are not generated; the screen recipes and shared state rules specify them. Do not use the static boards as image backgrounds for a working page.

## How to use the visual references

Open `gallery.html` directly in a browser. It is a local static reference with no APIs, forms or trackers. Boards show order, spacing, colour and emphasis. Labels such as “from API” identify dynamic content slots; they must never appear in the actual UI. The JSON and prose are authoritative for behaviour. Screen boards are not screenshots of deployed software.

Copy selected application assets into the frontend's public assets directory; never make production code fetch `docs/` paths. Import tokens once under `.bs-ui`; do not combine old dark/neon styling with the new components. Preserve historic URLs, notification subscriptions and valid return credentials during migration.

## Definition of an implementation response

At each task completion report: task ID, files changed, UI/API/database connection, checks actually performed, checks not performed, remaining blocker and rollback. “Done” cannot mean “component drawn.” Do not add `setTimeout` success, fake reports, hardcoded staff names, fake unread counts, or a promise of immediate help.
