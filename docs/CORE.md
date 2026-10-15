# MIAO Core

## Product idea

**Users operate the business. The assistant operates the application.**

MIAO is an agent-first platform for creating small internal business tools. A user describes the work; the assistant proposes and applies the application structure and interface; the team then uses the published interface to handle business records.

## The two surfaces

### 1. Assistant surface: build and maintain

All application design and administration should flow through the assistant, with a preview and confirmation when the change can write data, change permissions, or publish a version.

The assistant handles:

- Create an application from a request or template.
- Create and change tables, fields, relations, forms, pages, and layouts.
- Add or change business actions and status transitions.
- Add automations, scheduled tasks, collection plans, and notifications.
- Query and summarize business data.
- Prepare batch changes or deletions, show scope and samples, then execute after confirmation.
- Preview, publish, revise, and restore application interface versions.
- Configure read-only public pages and their allowed fields.
- Import or export a structure-only application template.

The assistant must work through server-validated capabilities. It must not execute arbitrary source code, SQL, HTML, or shell commands.

### 2. Published application surface: operate the business

The generated application is the user's everyday workspace. It should use business language, not database language.

Users can, when the published interface and their permission allow it:

- Move between business pages.
- Search, filter, sort, and paginate records.
- Open a record detail view and return to the list.
- Create, edit, and delete business records.
- Upload, view, and download supported attachments.
- Follow permitted status transitions and run declared business actions.
- Open related records and create related records.
- Copy a permitted record link.
- Read task results, notifications, and public read-only pages.

A user should see `客户`, `联系人`, `跟进记录`, `推进阶段`, and `新增客户`; they should not need to see collection names, schema, source IDs, JSON specs, or API parameters.

## What is not a normal application UI

Do not expose routine GUI editors for:

- Application name, structure, or schema.
- Table and field creation or deletion.
- Page composition or data-source binding.
- Business-action and workflow definitions.
- Automation definitions and schedules.
- Version publishing or public-field policy.
- Application permissions and membership.

These are assistant tasks. If an operator needs a change, they ask the assistant and receive a scoped proposal, validation result, and durable run/version record.

The application may show read-only context such as the current version, current permission, task status, or a link back to the assistant. The task panel is for inspecting runs and results, not for designing tasks. The application should not become a second application builder.

## Application navigation

The application shell should stay small. Keep the generated business pages and the assistant as the primary surfaces. A task/results view may remain available for inspecting background runs and their outcomes. Do not expose routine application-maintenance panels such as application settings, table management, access editors, template export, archive, or delete as everyday application actions; route those requests to the assistant.

## Permission boundary

- `viewer`: read published business interfaces.
- `editor`: read and edit permitted business records.
- `manager`: manage application drafts and application-level configuration through the assistant.
- `publisher`: manage and publish application versions through the assistant.
- Batch changes require the separate `can_batch` grant.

Permissions are enforced on the server. Hiding a button is not authorization.

## Core lifecycle

1. Describe the work to the assistant.
2. Assistant inspects the current application and permissions.
3. Assistant proposes a bounded change.
4. User reviews and confirms when required.
5. Server validates and records the change.
6. Assistant publishes or schedules the resulting version/task.
7. Team operates the generated interface.
8. Results, notifications, errors, and audit records remain inspectable.

## Safety and boundaries

- Published business CRUD must remain usable without an AI model.
- Assistant changes are versioned and recoverable; business records are not silently rolled back by a UI-version restore.
- Batch writes are previewed, bounded, permission-checked, and idempotent where applicable.
- Public pages are explicitly read-only and expose only approved pages, records, fields, and images.
- CSV/XLSX imports are bounded and require a reviewed plan; there is no unlimited import or OCR.
- Files are bounded uploads and storage; PDF/image storage is not OCR.
- Background tasks retain their declared scope and the creator's current authorization; they do not inherit owner privileges.

## UI rule of thumb

> If the user is deciding **what the application should be**, use the assistant.  
> If the user is deciding **what to do with a business record**, use the published application.

Detailed API, operational limits, task semantics, and deployment procedures live in [`OPERATIONS.md`](OPERATIONS.md). Product metadata lives in [`PRODUCT.md`](../PRODUCT.md).
