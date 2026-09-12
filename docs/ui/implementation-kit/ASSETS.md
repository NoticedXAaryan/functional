# Asset inventory and placement instructions

## Ready files

| Resource | Path | Usage |
| --- | --- | --- |
| Original supplied logo | `../assets/bal-setu-logo-source.jpeg` | Original provenance; optional About/footer. Keep unchanged |
| Simplified icon master | `assets/bal-setu-app-icon-master.png` | Raster shield/two-figure derivative; header/app icon source. White background is intentional |
| Welcoming illustration | `assets/welcome-bridge.png` | Optional home decoration; never required to understand an action |
| 24 editable action icons | `assets/icons/*.svg` | ViewBox 24×24, 1.8px rounded stroke; display 24–32px inside a 48px+ target |
| 12 word-picture icons | Same directory: boat, cloud, mango, leaf, moon, star, tree, book, cup, kite, ball, fish | Optional pictures beside reviewed word labels; these do not constitute a credential wordlist |
| Icon sprite | `assets/icons/sprite.svg` | Same symbols with currentColor; choose standalone files OR sprite, not duplicate downloads |
| Screen boards | `boards/board-01.svg` through `board-07.svg` | 38 screen layout references. Editable vector text/shapes, not application assets |
| Portable board images | `boards/board-01.png` through `board-07.png` | Rasterized copies for an AI that cannot display SVG; same design-only status |
| CSS | `tokens.css` | Copy/import into app and scope under `.bs-ui` |
| React primitives | `Primitives.tsx` | Presentation starters; see COMPONENTS.md for limits |
| Screen/copy data | `screens.json`, `copy.en.json` | Declarative implementation instructions and reviewed-design English strings; dynamic slots require API data |

Icon mapping: Ask for help→help; My messages→messages; Someone is missing→missing; Nearby alerts→alerts; record→microphone; listen→listen; place→location; leave→exit; privacy→privacy; send→send; no alerts→alerts; failed send→retry. Never use icons without visible text on critical actions. SVGs embedded by `<img>` use purple; use the currentColor sprite for white icons on dark buttons.

The app-icon master remains at its native generated dimensions. Actual image dimensions must be declared correctly in any manifest; do not label a large source file “192x192” without exporting it. Provide technical 192/512px raster exports during PWA integration using the normal asset pipeline, preserve the master, and inspect small-size readability and mask-safe area. No invented vector logo or generated favicon sizes are claimed here.

## Raster generation record

Generated with the built-in image generation tool; no external image API key or paid third-party asset library was configured. Both masters are saved inside this repository. The source logo was supplied by the user; no independent trademark/ownership investigation has been performed. Interface SVG geometry was authored for this project, not copied from a commercial icon pack. Generated images are not evidence of an endorsement or exclusive trademark rights.

**Welcome prompt:** Create one finished UI illustration for Bal Setu, a child-centred Indian help service. Optional small welcoming homepage illustration, not a logo or screenshot. A simple gently curved purple footbridge joining two soft rounded riverbanks with small teal leaves, a quiet pale sky blue stream. Flat contemporary cut-paper style, generous white background, few large legible shapes, purple #603B91, teal #176B70, lilac #F0EAF8, sky #EAF3FC, peach #FFF0E7. Wide horizontal composition, calm, approachable and non-infantile. No people, faces, text, shield, sirens, neon, glow or watermark. Work at 240px wide. New illustration, not an edit of the supplied logo.

**Icon prompt:** Edit the supplied Bal Setu logo into a square application icon master. Preserve the protective shield and two abstract figures connected by their arms. Remove lettering, tagline, underline and tiny decorative strokes. Simplify for small sizes. Muted purple #603B91 and teal #176B70, crisp rounded geometry, white background; no glow, neon, new symbols or text. Centre essential geometry within the central 70 percent, with generous padding. A simplified derivative, not a government seal.

Actual generated shading may not equal every flat colour token. Keep button/text colours defined by CSS, not sampled from raster images. The image masters are optional visual identity assets; they do not replace semantic controls.

## Deployment placement

Copy app assets to `apps/web/public/brand/` and, if independently deployed, `apps/ops/public/brand/`. Use a shared copy/build step to avoid divergent versions. Keep source paths stable in this kit; no production dependency on a developer's Downloads folder.

Use a 40px app mark beside live text in header; no tagline in the action area. Display the master with `object-fit:contain` and explicit width/height. Existing `/savera.svg` references need compatibility aliases until old workers update. Keep existing `savera.subscription.v1` storage until an explicit credential-preserving migration exists. Do not rename APIs/UUIDs to match the branding.

An Open Graph/social card is intentionally not supplied for private reports or active child identities. If public marketing needs one, use the brand and a neutral description only; no private case metadata. Real child portraits, locality maps and audio prompts are content requiring legitimate source and review, not decorative assets to invent.

## Rebuild instructions

Run `node docs/ui/implementation-kit/build-kit.mjs` to regenerate icons, screens, copy, boards, gallery and manifest from the committed definitions. It does not regenerate or overwrite the raster masters. Edit generator rows before regenerating screen files; otherwise hand edits will be overwritten. Commit both generator and generated outputs so a less capable AI can use the files without running tooling.

The PNG boards were rendered from the SVGs in a browser without contacting the application or API. Public-help and staff boards were visually inspected for clipping and hierarchy; this is design-artifact review, not application testing. If a board's SVG changes, re-export its PNG before handing it off. The project application was not tested or changed by this asset pass.
