# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Enterprise employees who need to build internal business tools themselves.

## Product Purpose

MIAO lets an employee describe an internal tool, shape its data in PocketBase, and use the resulting application with an AI agent embedded in the browser.

## Positioning

MIAO combines PocketBase's application backend with an in-browser fx agent. The agent works through capabilities explicitly supplied by the MIAO web application.

## Capabilities and Constraints

- PocketBase is the source of truth for identity and application data. The MIAO API enforces workspace access.
- fx runs in the browser through its WebAssembly SDK. The host application supplies its interface, credentials, instructions, and tools.
- The browser agent does not inherit fx CLI filesystem, shell, keychain, or MCP configuration.
- Embedded fx browser execution requires browser support for JavaScript Promise Integration (JSPI).
- The previous DSH-hosted Agent Web and split Builder Agent/User Agent model are being retired.
- The first version gives each account its own workspace. Workspace invitations and organization roles are outside the current product scope.
- The user provides a Vercel AI Gateway key for the browser agent; the key remains in the browser session.

## Brand Commitments

- Product name: MIAO.
- The retained MIAO cat mascot assets are under `public/mascots/`.

## Evidence on Hand

- Existing MIAO landing page and app UI are in `public/index.html`, `public/app.js`, and `public/styles.css`.
- No verified customer testimonials, adoption data, pricing, or deployment claims are supplied.

## Product Principles

- Keep identity and application data in PocketBase; enforce workspace access in the MIAO API.
- Put the agent where the user works: inside the browser application.
- Give the agent only the tools and data the signed-in user can access.
- Keep the first product loop short: describe, shape, use.
- Remove independently hosted agent infrastructure and duplicate product abstractions.
