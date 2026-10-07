# MIAO Frontend Design System

This is the working UI contract for MIAO. It is written for people and agents
making frontend changes in this repository. It records the visual decisions
that are already in use; it is not a request to rewrite the existing UI in one
pass.

The visual direction is **small, quiet, dense, and precise**: a neutral
workspace inspired by the MIAO prototypes and Vercel-style product surfaces.
Preserve real application capabilities and data while keeping the interface
calm.

## 1. Authority and scope

- `public/styles.css` is the implementation source of truth for the shared
  `miao` DaisyUI theme and application UI.
- `../miao.my/docs/prototypes/prototype.css` and its rendered prototypes are
  visual references, not a second component library.
- Reuse DaisyUI components and existing MIAO patterns before writing custom
  CSS. Configure theme tokens and compose components instead of redesigning
  DaisyUI components.
- This document governs the authentication screen, workspace, app runtime,
  data/settings views, assistant dock, and platform administration. Public
  published pages may need different content layouts, but should inherit the
  same palette, type, controls, and interaction rules.
- Product behavior and operations belong in `docs/OPERATIONS.md`, not here.

## 2. Core tokens

Use these values for new UI. Existing values outside the scale are migration
history; do not make broad visual changes just to normalize them.

### Color

```text
surface          #ffffff
surface-muted    #fafafa
surface-subtle   #f0f1f1
text             #171a1d
text-secondary   #62676d
text-tertiary    #73777c
border           #e6e7e9
border-strong    #d4d7da
primary          #171a1d
primary-text     #ffffff
```

Use color for status and feedback only. Semantic colors are allowed for
success, warning, error, and info; do not use gradients, colorful dashboard
cards, or colored navigation as decoration.

### Typography

Use the existing Chinese system stack; do not add a display-font dependency:

```css
"Noto Sans SC", "PingFang SC", "Microsoft YaHei",
-apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif
```

Use this ramp for new application UI:

```text
10px / 15px   labels, dense metadata
11px / 16px   micro actions and table metadata
12px / 18px   secondary text and compact controls
13px / 20px   default body and controls
14px / 22px   emphasized body and section headings
16px / 24px   page-section headings
20px / 28px   page titles and dialogs
28px / 35px   dashboard/hero title only
```

Prefer weights 400, 500, and 600. Do not introduce another type scale or use
large type to compensate for weak hierarchy. Use a monospace system stack for
IDs, code, tokens, and technical metadata when alignment matters.

Sole font exception: the brand wordmark `miao.my` inside `.brand strong`
renders in the self-hosted Caveat handwritten font
(`/fonts/caveat-latin-wght-normal.woff2`). Do not apply Caveat to any other
text or use it for headings.

### Spacing and dimensions

```text
spacing: 4 / 8 / 12 / 16 / 24 / 32 / 48 / 64px
icon-to-text: 8px       control-to-control: 8px
label-to-control: 6px   field-to-field: 16px
card padding: 16px      section gap: 24px
```

Current layout dimensions that are intentional:

```text
workspace sidebar: 216px       platform-admin sidebar: 224px
assistant dock: 360px           dashboard prompt: max 760px
runtime/content: max 1120-1160px
settings/form: max 480-820px    ordinary control: 30-32px high
```

Use the scale above for new spacing. Do not add one-off values such as 13px,
18px, 22px, or 30px without explaining the constraint in the change.

## 3. Surfaces, borders, and shape

- Prefer white surfaces, thin neutral borders, and whitespace over nested
  containers. A card is for an independent object, not a default wrapper.
- Default border is `1px solid #e6e7e9`; separators are also one-pixel and
  subtle. Do not border every subsection or nest bordered cards.
- New radii: 4px for small controls, 5-6px for fields/buttons, 7-8px for
  cards and boxes, and 10px for dialogs. Reserve pill shapes for statuses,
  tags, avatars, and compact segmented controls.
- The shared DaisyUI theme uses selector `4px`, field `6px`, box `8px`, one
  pixel borders, zero depth, and zero noise. Keep those tokens consistent.
- Page components have no shadow by default. Menus/popovers may use a subtle
  shadow; dialogs may use a restrained layered shadow. Never use heavy
  Material-style elevation or decorative gradient shadows.

## 4. Layout rules

### Authentication

Use a centered, narrow form on a white page: MIAO mark, title, short copy,
labeled fields, one black submit button, and secondary account links. Keep the
form focused; do not add dashboard decoration.

### Workspace

- Desktop uses a persistent 216px navigation rail, application content, and a
  360px assistant dock on overview and app views. The workspace header is a
  compact action row, not a second page-title system.
- The navigation rail keeps account and notifications at the bottom and can
  collapse through the existing toggle. Selected items use a subtle neutral
  background, never a bright brand color.
- Dashboard/app content is centered within the existing 1120-1160px bounds.
  Tables may use all available width and must scroll horizontally when needed.
- Forms and settings should stay narrow and readable. Prefer inline editing or
  a dedicated view over a modal for complex workflows.

### Assistant

The assistant is a product tool, not a consumer chat surface. Keep normal
conversation text at 12-14px, avoid decorative chat bubbles, and hide raw tool
calls unless details are explicitly opened. In the current workspace it is a
dark terminal column (`#171a1d`) with light text, muted activity, dark result
cards, and a white command composer. Preserve that contrast boundary when
editing the dock.

### Responsive behavior

- At phone widths (currently `max-width: 580px`), the navigation becomes a
  compact grid above the workspace and application cards become one column.
- At narrow desktop/tablet widths, hide or reflow the assistant according to
  the existing media queries rather than forcing horizontal overflow.
- Wide tables, record lists, and runtime previews keep their own horizontal
  scroll containers. Do not shrink data until it becomes unreadable.

## 5. Component rules

### Buttons and actions

- Reuse DaisyUI `btn` variants and the existing compact density. Ordinary
  controls should be 30-32px high; `xs`/`sm` are for dense metadata and table
  actions, while 36px is reserved for prominent entry points.
- Primary actions are near-black with white text. Secondary actions are white
  with a one-pixel border. Ghost actions are transparent. Destructive red is
  only for destructive intent.
- Normally give an area one primary action. Do not turn every action into a
  filled button; row actions usually belong in an overflow menu.
- Use transitions only on intended properties (`color`, `background-color`,
  `border-color`, `opacity`, or `transform`), normally 100-160ms. Never use
  `transition: all`.

### Inputs and forms

- Inputs/selects are compact, 30-32px high, with 10px horizontal padding and
  5-6px radius. Textareas start at roughly 80px and may grow with content.
- Labels are 12-13px, medium weight, and sit 6px above the control. Help text
  is 10-12px with a readable line height.
- Keep forms visually light: one clear label, one control, concise help, and
  no unnecessary panel around every field.

### Cards and tables

- Cards use 16px padding, a one-pixel border, 7-8px radius, and no shadow.
  Use the application initial for its icon; do not invent illustrations.
- Tables are compact: 36px headers, approximately 40px rows (36px in dense
  mode), 12px horizontal cell padding, and 11-13px text. Use tabular numbers
  for counts and operational values.
- Application cards keep the established structure: initial, name, purpose,
  then a separated footer for publication status and update date.

### Navigation, menus, and dialogs

- Navigation items are approximately 32-38px high, 8-10px horizontal padding,
  6px radius, 16px icons, and subtle neutral selected state.
- Tabs are compact (about 32px high) with a simple underline or quiet active
  background; avoid large boxed tab groups.
- Menus/popovers use about 4px padding, 8px radius, and 32px menu items.
  Dialogs are for focused confirmation or short forms; use roughly 400px,
  480px, or 640px widths and 24px internal padding.
- Use the existing inline SVG icon style at 14, 16, or
  20px. Icon-only controls need at least a 24px hit target and an accessible
  label.

## 6. Interaction and accessibility

- Preserve keyboard focus, visible `:focus-visible` outlines, dialog escape and
  backdrop behavior, loading/empty/error states, and `aria-live` updates.
- Never rely on color alone for status. Pair semantic color with text, icon, or
  an explicit label.
- Keep operation notices floating when possible so they do not shift the page.
- Maintain real data and permission boundaries. A visual change must not turn a
  disabled action into an enabled one or replace a server-backed state with a
  placeholder.

## 7. Agent implementation rule

When implementing a UI change, follow this order:

1. Reuse an existing MIAO pattern and DaisyUI component.
2. Apply the shared `miao` theme and compact density.
3. Use the token scales and layout dimensions in this document.
4. Remove unnecessary UI before adding a new wrapper or custom component.
5. Add custom CSS only for a product-specific interaction or a documented
   responsive constraint.

When uncertain, choose fewer elements, smaller type, less radius, less color,
and more whitespace. Inspect the rendered page at desktop and phone widths
before claiming a visual change is complete.

## 8. Reconciliation with the supplied proposal

The proposal was used as a design input, not copied wholesale:

| Proposal | MIAO decision | Reason |
| --- | --- | --- |
| Neutral palette, restrained borders/shadows, compact DaisyUI density | Adopt | Already matches the `miao` theme and prototypes. |
| 224px workspace sidebar | Adjust to 216px; keep 224px for platform admin | These are the current, intentional layouts. |
| Geist as the primary font | Do not adopt | MIAO is Chinese-first and already uses the prototype system stack. |
| One 12-24px application type scale | Adjust to the existing 10-28px ramp | Dense tables/metadata and the dashboard title already use these sizes. |
| Every ordinary control at 32px | Adopt as the default, with exceptions | Authentication and the command composer intentionally use larger controls. |
| 760px conversation column | Scope to conversation content/composer | The current assistant is a 360px dock; changing it would alter the workspace. |
| No cards by default; no decorative color | Adopt | Cards remain for independent objects and status colors remain semantic. |
| Avoid all radii above 10px | Adopt for new UI, not a cleanup mandate | Existing runtime/table surfaces contain compatibility radii. |
| Neutral light agent UI | Adjust | The current assistant's dark terminal column is an intentional product boundary. |
