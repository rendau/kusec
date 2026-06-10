# Secret Management System — Admin

Vue 3 admin SPA for the kusec service. Built with Vite, Vue Router, Pinia and Naive UI.

## Stack

- **Vue 3.5** (Composition API, `<script setup>`)
- **Vite 6** — dev server & build
- **Vue Router 4** — routing (lazy-loaded views, layout nesting)
- **Pinia** — state management
- **Naive UI** — component library
- **TypeScript** — strict mode

## Project structure

```
src/
  api/        HTTP client base (wraps the gRPC-gateway REST API)
  assets/     Global styles & static assets
  layouts/    App shell layouts (DefaultLayout)
  router/     Route definitions & typed meta
  stores/     Pinia stores
  views/      Route-level pages
  App.vue     Root: Naive UI providers + theme
  main.ts     App bootstrap
```

## Getting started

```bash
pnpm install
pnpm dev               # http://localhost:8000
```

Environment is configured via per-mode files loaded by Vite:

| File               | Loaded in        | Notes                                  |
| ------------------ | ---------------- | -------------------------------------- |
| `.env.development` | `pnpm dev`       | API points to `http://localhost:3000`  |
| `.env.production`  | `pnpm build`     | API is relative (`/api`); committed    |
| `.env.example`     | reference        | Template for local overrides           |

Local overrides go in `.env.local` / `.env.*.local` (git-ignored).
`VITE_PORT` / `VITE_HOST` control the dev & preview server (see `vite.config.ts`).

## Scripts

| Command           | Description                          |
| ----------------- | ------------------------------------ |
| `pnpm dev`        | Start the dev server                 |
| `pnpm build`      | Type-check + production build        |
| `pnpm preview`    | Preview the production build         |
| `pnpm type-check` | Run `vue-tsc` type checking          |
