# TypeScript standard

TypeScript here is small client glue for server-rendered pages (Go + HTMX + Alpine.js +
Tailwind CSS), plus k6 scripts (see `k6.md`). Sources, in order of precedence:

1. [Google TypeScript Style Guide](https://google.github.io/styleguide/tsguide.html)
2. [TypeScript Do's and Don'ts](https://www.typescriptlang.org/docs/handbook/declaration-files/do-s-and-don-ts.html)
3. [MDN JavaScript code guidelines](https://developer.mozilla.org/en-US/docs/MDN/Writing_guidelines/Code_style_guide/JavaScript)

## Tooling

- [Biome](https://biomejs.dev) formats and lints (`make ts-lint PROJECT=<project>`). Its
  config, `biome.json` at the repository root, is added with the first TypeScript code.
- `tsc --noEmit` type-checks; Biome does not replace the compiler.
- [esbuild](https://esbuild.github.io) bundles `<service>/web/ts/` into
  `<service>/web/static/`, which the Go service embeds (see `go-package-structure.md`).
- `tsconfig.json` enables `strict`, `noUncheckedIndexedAccess`,
  `exactOptionalPropertyTypes`, `noImplicitOverride`, `noFallthroughCasesInSwitch`, and
  `verbatimModuleSyntax`.

## Language rules

- `const` by default, `let` when reassigned; `var` never appears.
- Named exports only; no default exports. Use `import type` for type-only imports.
- ES modules only; no `namespace` and no `require`.
- `unknown` instead of `any`, narrowed with type guards before use. `@ts-ignore` and
  `@ts-expect-error` need the user's agreement and a comment.
- Primitive types (`string`, `number`, `boolean`), never wrapper objects (`String`,
  `Number`, `Boolean`, `Object`, `Function`).
- Interfaces for object shapes; type aliases for unions, tuples, and mapped types.
- Optional fields (`name?: string`) rather than `| undefined` in the type.
- Prefer union types over overloads. Callbacks whose result is ignored return `void`.
- Type assertions (`x as T`) only where the compiler cannot know what you know; never the
  non-null assertion (`x!`) to silence a possible `undefined`.
- `===` and `!==` only. Template literals for string building. `for...of` for iteration.
- `async`/`await` over promise chains; every promise is awaited or explicitly handled.
- Naming: `UpperCamelCase` types and classes, `lowerCamelCase` values and functions,
  `CONSTANT_CASE` for module-level constants. No leading or trailing underscores.

## Browser rules

- The server renders HTML; TypeScript enhances it. Pages work for their core task before
  scripts load where practical.
- Alpine components are registered from TypeScript modules with
  `Alpine.data('name', () => ({ ... }))`, using the CSP build (`@alpinejs/csp`) so the
  Content Security Policy needs no `unsafe-eval`.
- Never assign untrusted data to `innerHTML`, `outerHTML`, or `insertAdjacentHTML`; use
  `textContent` or server-rendered fragments.
- Attach behaviour with `addEventListener` or Alpine directives, never inline `on*`
  attributes.
- Browser code has no logging library. User-facing errors render in the page; failures
  the server must know about are reported to a server endpoint that logs them.

## Comments

The repository comment rules in `AGENTS.md` apply. Exported functions, types, and
components get a `/** ... */` JSDoc comment describing what they do and how to use them;
types live in the signature, not in JSDoc tags.
