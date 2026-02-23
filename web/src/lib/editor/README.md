# Editor Utilities

This folder contains non-UI logic extracted from `routes/editor/+page.svelte`.

## Files

- `types.ts`: shared editor types (`ApiBlock`, `ApiPage`, `GalleryItem`, `Rgb`).
- `blocks.ts`: gallery/block conversion and normalization helpers.
- `theme.ts`: theme generation and cover-palette extraction logic.

## Design Notes

- UI components stay in Svelte files (`Block.svelte`, `Cover.svelte`, route pages).
- Pure data/transform logic lives here to keep route code small and testable.
- Functions are side-effect free except `extractPaletteFromImage`, which reads image pixels in-browser.

## Usage Pattern

1. Route/components keep local UI state.
2. For data transforms, call helpers from `blocks.ts`.
3. For styling variables, call `buildThemeStyle(...)` from `theme.ts`.
4. Keep API response types in `types.ts` to avoid duplication.
