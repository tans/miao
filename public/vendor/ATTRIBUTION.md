# Frontend assets

`ui.css` is generated locally from the checked-in source and pinned npm
packages. It contains only the DaisyUI components and Tailwind utilities found
in the MIAO frontend sources; it is not fetched from a CDN.

Regenerate with `npm run build:assets`. The source is
`scripts/ui-src.css`; package versions are pinned in `package.json` and
`package-lock.json`.

- Tailwind CSS 4.3.3 and `@tailwindcss/cli` 4.3.3 — MIT License.
- daisyUI 5.7.47 — MIT License.
