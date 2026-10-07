# MIAO interface

Visual authority: `../miao.my/docs/prototypes/prototype.css` and its rendered prototypes, especially `01-overview.png`. Use the prototypes' Vercel-style workspace grammar while retaining the live application's capabilities and real data.

## Shared system

- White surfaces, near-black text and primary actions (`#171a1d`), secondary text (`#62676d`), and thin neutral borders (`#e6e7e9`). Reserve semantic colors for status and feedback.
- Use the system Chinese sans-serif stack from the prototypes; avoid external display fonts.
- Keep buttons and fields compact with 5–6px corners, and application cards with 7px corners. Use borders for grouping and restrained shadows for menus.
- Retain the MIAO cat logo. Application icons use the application's initial rather than invented illustrations.
- Use the existing daisyUI components and the shared `miao` theme in `public/styles.css`.
- Type ramp (px): 10 uppercase micro labels, 11 meta and micro buttons, 12 secondary text and small controls, 13 default body and controls, 14 emphasized body and section headings, 16 page-section headings, 20 page titles and dialogs, 28 hero only. Avoid sizes off the ramp.

## Surfaces

- Authentication: a centered, narrow form on white, with the logo, page title, labeled fields, one black submit button, and secondary account links.
- Desktop workspace: a 216px sidebar (account entry and notification bell pinned at its bottom; collapsible through the floating panel toggle), the application content, and — on the overview and application views only — a 360px assistant chat column that is always expanded. The dock composer is a dark command bar (`#171a1d`, flush to the edges) holding a floating white input card and inverted white primary button; there is no standalone assistant page and no way to hide the column on those views. The workspace header is a single action row without breadcrumb or page titles.
- Application cards: initial, name, purpose, then a separated footer containing the actual publication status and update date.
- Data inspection, application runtime, dialogs, and platform administration inherit the shared neutral palette and controls.
- On phones, the sidebar becomes a compact navigation grid above the workspace. Application cards stack vertically; wide data tables retain their own horizontal scrolling.

Product behavior and operational procedures remain in `docs/OPERATIONS.md`.
