# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Enterprise teams that need to build and use internal business tools without waiting for a software delivery cycle.

## Product Purpose

MIAO lets enterprise teams describe an internal tool, shape its data in PocketBase, and use the resulting application with an AI agent embedded in the browser.

## Positioning

MIAO combines PocketBase's application backend with an in-browser fx agent. The agent works through capabilities explicitly supplied by the MIAO web application.

## Capabilities and Constraints

- PocketBase is the source of truth for identity, application data, files, and access rules.
- fx runs in the browser through its WebAssembly SDK. The host application supplies its interface, credentials, instructions, and tools.
- The browser agent does not inherit fx CLI filesystem, shell, keychain, or MCP configuration.
- Embedded fx browser execution requires browser support for JavaScript Promise Integration (JSPI).
- The previous DSH-hosted Agent Web and split Builder Agent/User Agent model are being retired.
- Model credential handling and enterprise organization/role administration remain undecided.

## Brand Commitments

- Product name: MIAO.
- Existing MIAO cat mascot assets are available under `public/mascots/`.

## Evidence on Hand

- Existing MIAO landing page and app UI are in `public/index.html`, `public/app.js`, and `public/styles.css`.
- A CRM application example exists under `apps/test/`; it is a sample, not customer evidence.
- No verified customer testimonials, adoption data, pricing, or deployment claims are supplied.

## Product Principles

- Keep application data and access control in PocketBase.
- Put the agent where the user works: inside the browser application.
- Give the agent only the tools and data the signed-in user can access.
- Keep the first product loop short: describe, shape, use.
- Remove independently hosted agent infrastructure and duplicate product abstractions.
