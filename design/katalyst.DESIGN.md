---
name: Katalyst Design System
short_name: KDS
package: "@kalvi-education/katalyst"
description: >
  Visual language for Kalvium product surfaces. Monochrome by default, hue reserved
  for meaning, hairline elevation, one radius knob. This document names tokens and
  states rules; it never restates token values.

# Token sources. This document REFERENCES these; it does not redefine them.
# If a value is what you need, read the source — do not copy it here.
tokens:
  semantic:
    source: src/styles/globals.css
    published_as: "@kalvi-education/katalyst/globals.css"
    selectors: [":root", ".dark"]
    role: Role tokens. Theme-aware. Components consume only these.
  palette:
    source: src/styles/palette.css
    generated_by: "@121arun/nds-tokens (from Figma) — do not hand-edit"
    pattern: "--color-{hue}-{soft|medium|main|bold}"
    role: Categorical accent. Apps reach here only to encode meaning.
  figma_mapping:
    roles: "Figma `base/*` → semantic role tokens (base/card → --card)"
    palette: "Figma `custom/{hue}-{step}` → --color-{hue}-{step}"
    icons: HugeIcons
  theme_mapping:
    source: "src/styles/globals.css → @theme inline"
    role: Binds semantic tokens to Tailwind v4 utilities and derives the radius scale.
  typography:
    root: "--font-sans"
    heading: "--font-heading"
  radius:
    root: "--radius"
    derived: "--radius-sm … --radius-4xl, via calc() multipliers off --radius"
  spacing:
    source: Tailwind v4 --spacing(n)
    note: KDS defines no custom spacing scale.

authoring_rule: >
  Reference tokens by name (--muted-foreground) or by the utility that resolves to
  them (text-muted-foreground). Never write a colour literal, and never write a
  radius in px. Enforced by scripts/check-design-tokens.mjs on every build.

related:
  contracts: "@kalvi-education/katalyst/contracts — per-component API and a11y obligations"
  storybook: Storybook MCP server, live while `yarn storybook` runs
---

# Katalyst Design System

## 1. Overview

KDS is a **monochrome system with categorical accent**. Chrome — surfaces, text,
borders, controls — is achromatic. Hue appears only where it carries meaning:
status, category, or a semantic callout. A screen that reads as colourful is
almost always a screen that has reached past its role tokens.

Tokens live in three layers. Each layer has one job, and code should touch exactly
one of them.

| Layer | Where | What it is | Who consumes it |
|---|---|---|---|
| **Palette** | `src/styles/palette.css` | 26 hues × 4 steps, generated from Figma | App code, for categorical accent only |
| **Roles** | `src/styles/globals.css` `:root` / `.dark` | Semantic tokens: what a colour is *for* | Every Katalyst component |
| **Theme** | `globals.css` `@theme inline` | Binds roles to Tailwind utilities; derives radius | Tailwind, at build time |

The load order matters: `globals.css` imports Tailwind, then `palette.css`, then
declares roles. Roles are declared *after* the palette, so a role token can never be
clobbered by a palette regeneration.

The layers exist in Figma under their own names, and the mapping is one-to-one:
Figma's `base/*` collection is the role layer (`base/card` is `--card`), and
`custom/{hue}-{step}` is the palette layer. Spacing in Figma is Tailwind's 4px scale,
unrenamed. When a design and this document disagree, check §11 before assuming the
design is wrong — some of the drift is real and known.

**The rule that follows from this:** a component styles itself with roles
(`bg-card`, `text-muted-foreground`, `border-input`). An application may additionally
use palette hues, but only to encode meaning a role token cannot express — a
category chip, a status callout, a chart series. Chrome is never hued.

## 2. Colour

### 2.1 Role tokens

Each token below is defined in both `:root` and `.dark`. Use the role, not the value;
the whole point of the layer is that the value changes under `.dark` and your code
does not.

| Token | Job |
|---|---|
| `--background` / `--foreground` | Page ground and its default text |
| `--card` / `--card-foreground` | Raised content surface and its text |
| `--popover` / `--popover-foreground` | Transient overlay surface — menus, tooltips, popovers |
| `--primary` / `--primary-foreground` | The single emphatic action on a view |
| `--secondary` / `--secondary-foreground` | Supporting action of equal structural weight, lower emphasis |
| `--muted` / `--muted-foreground` | Recessed surface; secondary and helper text |
| `--accent` / `--accent-foreground` | Transient interaction state — hover, highlighted menu row |
| `--destructive` / `--destructive-foreground` | Irreversible or damaging intent; the foreground is the text on a solid destructive fill |
| `--border` | Hairline separators and container edges |
| `--input` | Form control edges — deliberately distinct from `--border` |
| `--ring` | Focus indication. Only this token. |
| `--chart-1` … `--chart-5` | Ordered categorical series for data display; the hues differ between light and dark |
| `--sidebar*` | Parallel role set for persistent navigation chrome |

Three things worth knowing about how these are set:

- **`--primary` is near-achromatic.** The emphatic button in KDS is near-black in
  light mode and near-white in dark. There is no brand hue in the control layer.
  This is why the system reads as calm and why an accidental hued button is loud.
- **`--muted` and `--accent` match in light mode and differ in dark.** In dark, `--accent`
  is the lighter of the two, so a hover row reads against a recessed surface. They are
  still not interchangeable in *intent*: `--muted` is a resting recessed surface,
  `--accent` is a transient interaction state. In light mode they currently resolve to
  the same value, so keep using each for its meaning.
- **`--chart-*` is a categorical hue set, and it is not the same set in both modes.**
  The five series are ordered and picked per mode so each stays distinct on that mode's
  ground; the same index is therefore a different hue in light and dark. Never name a
  series by its colour — name it by its index, and let the data carry the label.
- **`--destructive` is a text and tint colour, not a fill by default.** Its foreground
  token exists for the rare solid fill; the standing rule in §8 still prefers the tinted
  treatment.

### 2.2 Palette layer

`palette.css` provides 26 hues, each on a four-step ramp, in light and dark:

```
--color-{hue}-soft     a faint translucent tint of the hue — a surface wash
--color-{hue}-medium   a stronger translucent tint — borders, dividers, disabled states
--color-{hue}-main     the hue at partial strength — icons, marks
--color-{hue}-bold     a solid dark shade of the hue — the only step that is opaque
```

`soft`, `medium` and `main` are **translucent**: they are one shade of the hue at a
set opacity, so they take their final colour from whatever sits behind them. The same
token reads as a pale wash on a white page and as a faint glow on a near-black one,
and a `soft` surface laid over another tint stacks. `bold` is solid and does not
adapt to its background. `soft` is slightly stronger in dark than in light so it stays
visible on the darker ground.

Hues: `slate` `gray` `zinc` `neutral` `stone` `taupe` `mauve` `mist` `olive`
`orange` `amber` `yellow` `lime` `green` `emerald` `teal` `cyan` `sky` `blue`
`indigo` `violet` `purple` `fuchsia` `pink` `rose` `red`.

Each is registered in `@theme inline`, so the Tailwind utilities exist directly:
`bg-violet-soft`, `text-violet-bold`, `border-amber-medium`.

**The pairing rule.** In light mode a tinted callout uses `soft` as its background and
`bold` as its text, from the same hue, which clears AA for almost every hue (yellow
and orange fall just short; check them before using them for body text). Never mix
ramps across hues in one element, and never put `main` text on a `soft` background —
that pairing does not clear contrast in either mode.

**In dark mode this pairing does not hold.** `bold` is a dark solid in both modes, and
`soft` is a dark tint in dark mode, so bold text on a soft surface is low contrast for
every hue. Until the dark ramp is revised (see §11), put text on a soft surface in
`--foreground` and let the tint carry the hue, or use `bold` as a fill, not as text.

```tsx
// Semantic callout — correct pairing
<div className="rounded-lg bg-amber-soft p-3 text-sm text-amber-bold">
  Please ensure your response is at least 1 minute long.
</div>
```

Three tokens sit outside the ramp system. `--color-outline` is a translucent hairline.
`--color-kalvium-red` is the **Kalvium brand red** — the colour of the logo, and the only
thing it is for; it is deliberately identical in light and dark, because a brand colour
does not invert, and it is consumed by `logo.tsx` alone. `--color-artifacts` has no
recorded meaning; see §11.

**The brand red is identity, never state.** It is not an error colour, not an accent,
and not a hue you may reach for to emphasise something — `--destructive` and the red
ramp exist for that. Anything tinted `--color-kalvium-red` should be the logo.

## 3. Typography

`--font-sans` is **Figtree**, with a system fallback stack. `--font-heading` aliases
`--font-sans` — KDS is a single-family system, and hierarchy comes from size and
weight rather than from a contrasting face.

The face is loaded by an `@import` that `scripts/prepend-font.mjs` prepends to the
compiled `dist/katalyst.css`, because CSS requires `@import` to come first.

Components use a deliberately narrow scale:

| Step | Where it is used |
|---|---|
| `text-xs` | Badges, metadata, dense secondary labels |
| `text-sm` | **The default.** Body copy, control labels, card body |
| `text-base` | Card titles; input text on small viewports |

Weight is narrower still: `font-medium` for anything that leads (titles, labels,
control text), `font-normal` for body. No Katalyst component ships `font-semibold` or
`font-bold`, except the `AppShell` header title (below).

Figma, however, does define a display step the component layer has never implemented:
`text-2xl` at 24/32 in semibold. Page titles in product surfaces use it. Until it is
encoded, treat it as a page-level treatment (`text-2xl font-semibold`) rather than
inventing a component variant for it.

The one exception is the `AppShell` header, which follows the Figma App shell frame: its surface
title is `text-3xl font-bold` on a 36px line, because it names the whole surface rather than a
page inside it. That is the only place a Katalyst component ships a weight above `font-medium`,
and it is a deliberate deviation from the rule above, not a precedent for others.

`CardTitle` is the reference treatment: `font-heading text-base leading-snug
font-medium`, stepping down to `text-sm` when the card is `data-size="sm"`.

**Input text is `text-base` below `md`, `text-sm` above.** This is not an
inconsistency — 16px is the threshold below which iOS Safari zooms on focus. Keep
it on any new text control.

## 4. Layout and spacing

KDS defines **no custom spacing scale**. Use Tailwind's `--spacing(n)` and its
utilities directly. A design system that re-baptises `4` as `sm` buys nothing and
costs everyone a lookup.

Density is instead controlled by a **component-scoped spacing variable**. `Card`
declares `[--card-spacing:--spacing(4)]` on the root, consumes it for gap and
padding throughout its parts, and retunes it to `--spacing(3)` under
`data-[size=sm]`:

```tsx
// card.tsx — one variable drives gap, py, and every part's px
"flex flex-col gap-(--card-spacing) py-(--card-spacing) [--card-spacing:--spacing(4)]
 data-[size=sm]:[--card-spacing:--spacing(3)]"
```

Follow this pattern for any new composite component with a density variant: declare
one `--{component}-spacing` on the root, have parts read it, and override that single
variable per size. Do not fan size variants out across every child.

## 5. Shape

**There is one radius knob: `--radius`.** Every other radius is derived from it in
`@theme inline`, so retuning the system's roundness is a one-line change.

| Token | Multiplier of `--radius` |
|---|---|
| `--radius-sm` | 0.6 |
| `--radius-md` | 0.8 |
| `--radius-lg` | 1.0 |
| `--radius-xl` | 1.4 |
| `--radius-2xl` | 1.8 |
| `--radius-3xl` | 2.2 |
| `--radius-4xl` | 2.6 |

These multipliers are the only numeric literals in this document, because the ratios
*are* the design decision and are recorded nowhere else.

Conventions in use: `rounded-lg` for controls (buttons, inputs), `rounded-xl` for
containers (cards, panels), `rounded-4xl` for pills (badges).

**Small controls clamp their radius.** At `xs` and `sm` sizes the derived radius
would be disproportionate on a short control, so `button.tsx` clamps it:

```tsx
"rounded-[min(var(--radius-md),10px)]"
```

Use the same clamp for any control under 28px tall. Note it reverts to `rounded-lg`
inside a button group, so grouped edges stay flush.

The clamp is defensive: at the shipped `--radius` the derived value is already below
the ceiling, so it binds only if `--radius` is raised. That is the point — it lets the
knob move without small controls turning into lozenges.

## 6. Elevation

**KDS has no drop shadows.** Depth is expressed with hairlines and rings. `Card`
sets its edge with `ring-1 ring-foreground/10` — a ring rather than a border, so
the edge does not participate in layout or shift the box.

The depth ladder, in order:

1. **Flush** — `bg-background`, no edge. Page ground.
2. **Delineated** — `border-border` hairline. Separators, table rows, list rows.
3. **Raised** — `bg-card` + `ring-1 ring-foreground/10`. Cards, panels.
4. **Overlay** — `bg-popover` + ring, plus a `fade-in` entry. Menus, dialogs, tooltips.

Do not add `box-shadow` to reach for a fifth level. If a surface needs to read as
further forward, it is an overlay and should behave like one.

**Dashed panel edges** appear throughout Kalvium product surfaces — section
containers, step separators, rail boundaries. This is a real and consistent house
pattern, but it is **not currently tokenized**. The one place a Katalyst component
applies it is the footer of a left or right `Sheet`, which sits under a dashed
hairline. Elsewhere, treat it as a product-tier convention until §11 is resolved.

## 7. Motion and focus

Motion is short and unobtrusive. Controls use `transition-all`; overlays animate in
at `duration-100` with `data-open:fade-in-0`. Pressed controls displace by exactly
one pixel — `active:not-aria-[haspopup]:translate-y-px` — which excludes menu
triggers, since a trigger that sinks while its menu opens reads as broken.

Because components animate in, **an assertion on visibility immediately after
opening an overlay reads the element mid-animation.** Wrap it in `waitFor`;
`findBy*` only retries until the element *exists*, which is not the same thing.

**Focus is uniform and non-negotiable.** Every focusable control in KDS uses:

```tsx
"focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
```

A 3px ring in `--ring` at 50% opacity, plus a border swap. Invalid controls mirror
the same shape in `--destructive` via `aria-invalid:`. Do not introduce a different
focus treatment, do not remove the ring to "clean up" a design, and do not rely on
the border swap alone — the ring is what carries the indicator against a
low-contrast surface.

## 8. Controls

Control heights form a compact ladder. Buttons and inputs share the `h-8` default,
so they align on a row without adjustment.

| Size | Height | Icon-only |
|---|---|---|
| `xs` | `h-6` | `size-6` |
| `sm` | `h-7` | `size-7` |
| `default` | `h-8` | `size-8` |
| `lg` | `h-9` | `size-9` |

Badges sit below the ladder at `h-5`, since they are content, not controls.

Button variants and their intents:

| Variant | Intent |
|---|---|
| `default` | The one emphatic action on the view. Solid `--primary`. |
| `outline` | Standard action on a bare surface. Bordered, transparent fill. |
| `secondary` | Standard action needing more presence than `outline`. |
| `ghost` | Tertiary or repeated action — toolbar, list row, icon button. |
| `destructive` | Irreversible intent. |
| `link` | Navigation rendered inline in prose. |

**`destructive` is tinted, not solid.** It renders `bg-destructive/10
text-destructive`, not a saturated red fill. A wall of solid red reads as an error
state rather than an available action; the tint keeps the control legible as a
control while still being unmistakable. Both `Button` and `Badge` follow this.

Every component sets `data-slot` on each part, and `Button` additionally sets
`data-variant` and `data-size`. These are the supported styling and testing hooks —
target them rather than class names, which are implementation detail.

## 8.1 Icons

Icons come from **HugeIcons** (`@hugeicons/react` with `@hugeicons/core-free-icons`),
which is also what the Figma libraries use. Components size them by context rather than
by prop: an unsized `svg` inside a `Button` gets `size-4`, dropping to `size-3` at `xs`
and `size-3.5` at `sm`; a `Badge` forces `size-3`. Pass an explicit `size-*` only to
override that, and mark leading and trailing icons with `data-icon="inline-start"` or
`"inline-end"` so the control tightens the padding on that side.

An icon that carries meaning alone needs an accessible name on its control. An icon
beside a text label is decorative and should not be announced.

## 9. Composition patterns

Observed across Kalvium product surfaces and worth keeping consistent:

**Rail and canvas.** A persistent instructional rail on one side, recessed
(`--muted` family, subdued text), against a primary work canvas. The rail carries
standing guidance that stays true for the whole flow; it never holds the primary
action. Content that changes per step belongs on the canvas.

**Numbered step header.** A step title paired with an icon breadcrumb showing the
whole sequence, with completed steps marked. The title carries the ordinal
("1. Connect a GIT Repository"); the icons carry position.

**List row with trailing action.** Icon, then a title over a secondary line, then a
trailing emphatic button. Rows separated by hairlines, not gaps. The row's affordance
is the trailing control, not the row itself, unless the whole row is a link.

**Terminal helper row.** A list ends with a non-interactive row in `--muted`
answering the obvious failure question ("Can't find your repository?"). It is part
of the list container, not floated below it.

**Checklist with live status.** Radio-style status marks against a task list, each
with a description and its own inline action. State is carried by the mark, not by
colouring the whole row.

**Semantic callout strip.** Short tinted cards in a row, each using the §2.2 pairing
rule, hue chosen for category. Used for at-a-glance guidance, not for errors.

**Footer action bar.** Cancel and primary, right-aligned, separated from content by
a hairline. Primary is always rightmost.

## 10. Do and don't

**Do**

- Style components with role tokens; reach for the palette only to encode meaning.
- In light mode, pair `bg-{hue}-soft` with `text-{hue}-bold` from the same hue. In dark mode, use `text-foreground` on a `soft` surface.
- Derive every radius from `--radius`; clamp it on controls under 28px.
- Express depth with hairlines and rings.
- Give icon-only buttons an accessible name — `aria-label` or visually hidden text.
- Target `data-slot` in tests and overrides.
- Read a component's contract before writing code against it. It carries the
  accessibility obligations that types cannot express.

**Don't**

- Don't hue the chrome. A hued button, border, or page surface is almost always wrong.
- Don't add `box-shadow`. If it needs to float, it is an overlay.
- Don't write a colour literal or a px radius in component code.
- Don't replace or remove the focus ring.
- Don't put `main`-step text on a `soft`-step background, or `bold`-step text on a `soft` background in dark mode.
- Don't hand-edit `palette.css` — it is generated, and your change will be lost.
- Don't use `--muted` and `--accent` interchangeably because they match in light mode.
- Don't fan size variants across child parts; retune one scoped spacing variable.

## 11. Known gaps

Recorded honestly so they get fixed rather than cargo-culted:

- **Dashed panel edges are untokenized.** The pattern is pervasive in product
  surfaces but lives in no component and no token. It needs either a token pair or
  a documented decision that it stays app-level.
- **`--color-artifacts` is unexplained.** It sits in `palette.css` outside the
  hue-ramp scheme, with no recorded meaning or usage rule.
- **`--muted` and `--accent` are still duplicates in light mode.** They now differ in
  dark. Either give them different light values or accept that the split is dark-only.
- **The dark palette fails the soft/bold pairing.** Measured with each ramp composited
  over the page ground, `bold` text on `soft` clears AA for 24 of 26 hues in light mode
  (yellow and orange fall just short) and for none in dark, where it lands between
  roughly 1.2:1 and 3.4:1. `bold` is a dark solid in both modes. The ramp needs a lighter
  dark-mode `bold`, or the dark pairing rule has to change permanently.
- **`--chart-*` hues differ by mode,** so a screenshot of a chart in light and dark does
  not compare series by colour. If that matters for a surface, fix the series colours at
  the app level.
- **Focus ring width is written two ways** — `ring-3` in `button.tsx`,
  `ring-[3px]` in `badge.tsx`. They compute the same; the inconsistency should go.
- **Radius disagrees between Figma and code.** Figma defines `rounded-md` at 8px and
  `rounded-lg` at 10px; the derived scale off the current `--radius` produces roughly
  5.8px and 7.2px. Every KDS surface is therefore slightly squarer than its design.
  One side has to move — most likely `--radius`, since a single knob change reconciles
  the whole scale.
- **The `text-2xl` semibold display step exists in Figma but in no component.**
- **No documented breakpoint or touch-target policy.** The only responsive rule
  encoded anywhere is the input's `md:text-sm` step.
