# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Enterprise employees who need to build internal business tools themselves.

## Product Purpose

MIAO lets an employee describe an internal tool, shape its data in PocketBase, and use the resulting application with an AI agent embedded in the browser.

## Positioning

MIAO combines PocketBase's application backend with an in-browser fx agent and explicitly enabled background tasks. The agent works through capabilities explicitly supplied by the MIAO web application.

## Capabilities and Constraints

- PocketBase is the source of truth for identity and application data. The MIAO API enforces workspace access.
- Confirmed backend architecture: keep Bun/Fastify and a separate PocketBase service connected over local HTTP. Task scheduling and run management belong to the Bun service; background Agent execution uses libfx native within Bun. This implementation has not completed runtime validation.
- Background task delivery includes read-only previews, bounded confirmation waits, task archive/transfer, attempt history, and an in-app notification inbox. PocketBase hooks atomically save business records and enqueue event runs; runtime validation remains pending.
- fx currently runs in the browser through its WebAssembly SDK. The host application supplies its interface, credentials, instructions, and tools.
- AI Gateway credentials are configured by the enterprise on the server; the browser agent sends requests through an authenticated MIAO proxy and never receives the long-lived key.
- The browser agent does not inherit fx CLI filesystem, shell, keychain, or MCP configuration.
- Embedded fx browser execution requires browser support for JavaScript Promise Integration (JSPI).
- The previous DSH-hosted Agent Web and split Builder Agent/User Agent model are being retired.
- Background task code: server-side Agent tasks continue when the browser is closed and may act automatically within an explicitly pre-authorized scope. Interactive and background execution share application tool contracts and server-enforced permissions; they do not introduce separate Builder/User Agent product entities. The detailed target model and delivery requirements are maintained in `docs/OPERATIONS.md`, section 9.
- Each account owns a personal workspace and can invite other users to collaborate in that workspace.
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
