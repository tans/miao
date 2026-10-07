# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Team members who build internal business tools and visitors who read explicitly published application pages.

## Product Purpose

MIAO lets a team describe a work application, shape its data in PocketBase, interact with an Agent through the browser interface, and selectively publish read-only pages to external visitors.

## Positioning

MIAO combines PocketBase's application backend with a server-side Agent harness and explicitly enabled background tasks. The Agent works through capabilities supplied by MIAO and limited by the current member's permissions.

## Capabilities and Constraints

- PocketBase is the source of truth for identity and application data. The MIAO API enforces workspace access.
- Backend architecture: one Go process embeds PocketBase, the UI, and immutable database migrations. Native Go model hooks atomically save business records, audit changes, and enqueue task events. Task execution and business permissions are shared by browser and background operations. Real database tests run without a separate PocketBase binary; live model execution remains to be validated.
- Background task delivery includes read-only previews, bounded confirmation waits, task archive/transfer, attempt history, and an in-app notification inbox. PocketBase hooks atomically save business records and enqueue event runs; real database regression tests cover the shared environment; live model validation remains pending.
- AI Gateway credentials are configured by the enterprise on the server; the browser agent sends requests through an authenticated MIAO proxy and never receives the long-lived key.
- Business settings (registration and verification policy, mail transport, AI/Jev providers and credentials, platform admin list, backup policy) are stored in the database as the authoritative source and managed by platform admins through the console; environment variables only bootstrap the data directory, listen address, settings encryption key, and the first-admin import, and are imported into the database exactly once.
- Agent capabilities do not include shell access or arbitrary application source execution.
- The independent Go harness owns multi-step execution, confirmations, budgets, cancellation, and durable receipts. Interactive application building and the background agent loop continue to mature; a complete CRM/CMS/collection workflow has not passed live acceptance.
- Background task code: server-side Agent tasks continue when the browser is closed and may act automatically within an explicitly pre-authorized scope. Interactive and background execution share application tool contracts and server-enforced permissions; they do not introduce separate Builder/User Agent product entities. The detailed target model and delivery requirements are maintained in `docs/OPERATIONS.md`, section 9.
- Applications share durable business notes and protected files; per-user conversations persist with permission-scope and revision checks.
- UI schema v3 stores controlled json-render components, data sources bound to real resources, and declared actions. UI versions are previewed and explicitly published.
- User-defined workflows bind a state machine to any application table and state field; states and transitions are configurable and are not tied to product, lead, or order domains.
- Public publication binds selected pages or source routes to explicit table and field read grants; anonymous runtime uses only the current published version and exposes no writes, attachments, relations, or business actions.
- Agents and collection scripts can read external HTTP(S) sources directly; runs remain bounded by request/response budgets and collection writes use idempotent receipts.
- CSV/XLSX imports require reviewed plans, limited to 100 rows. Record history supports conflict-checked restoration excluding files, deletion, and schema.
- Authenticated external events target enabled manual tasks and use event IDs for deduplication.
- Each account can create and own multiple workspaces and can invite other users to collaborate in them.
- Workspace membership and role checks are enforced by the MIAO API for every app, table, and record operation.

## Brand Commitments

- Product name: MIAO.
- The retained MIAO cat mascot assets are under `public/mascots/`.

## Evidence on Hand

- Existing MIAO landing page and app UI are in `public/index.html`, `public/app.js`, and `public/styles.css`.
- No verified customer testimonials, adoption data, pricing, or deployment claims are supplied.

## Product Principles

- Keep identity and application data in PocketBase; enforce workspace access in the MIAO API.
- Keep interactive agent use inside the browser application; support explicitly enabled unattended tasks through the server-side execution capability, pending runtime validation.
- Give interactive agents only the tools and data the signed-in member can access; background tasks are additionally limited by their explicit grant and the authorizer's current permissions.
- Keep the first product loop short: describe, shape, use.
- Avoid independently managed agent products and duplicate abstractions; background execution remains part of MIAO's application runtime.
