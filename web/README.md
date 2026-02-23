# Jot Web

Minimal SvelteKit editor UI for the Jot backend. It supports block editing, simple nesting, drag reorder, and embeds.

## Setup

```sh
cd web
npm install
```

Create a local env file:

```sh
cp .env.example .env
```

## Development

```sh
npm run dev -- --open
```

## Configuration

The editor reads the backend URL from `PUBLIC_API_URL`.

```
PUBLIC_API_URL=http://localhost:8080
```
