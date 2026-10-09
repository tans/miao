# Vendored frontend assets

These files are self-hosted so the UI never loads anything from an external
CDN. Reference them from the pinned file names above; replace them (and update
the references in `index.html` / `site.html`) as one unit when upgrading.

- `daisyui-5.7.47.css` — daisyUI 5.7.47, MIT License. Mirror of
  `https://cdn.jsdelivr.net/npm/daisyui@5` (resolves to `daisyui.css`,
  upstream: https://github.com/saadeghi/daisyui).
- `tailwindcss-browser-4.3.3.js` — @tailwindcss/browser 4.3.3, MIT License.
  Mirror of `https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4` (resolves to
  the jsDelivr-minified `dist/index.global.js`,
  upstream: https://github.com/tailwindlabs/tailwindcss).

Vendored on 2026-10-09 from the exact bytes those CDN URLs served.
