---
name: Conduit Design System
description: The warm, enterprise-grade visual language used by the Conduit dashboard, setup flow, extension guide, and browser extension surfaces.
colors:
  primary: "#315B8C"
  primary-hover: "#264972"
  primary-pressed: "#1D3858"
  secondary: "#F2765E"
  canvas: "#F5EBDD"
  surface-1: "#EFE4D4"
  ink: "#413333"
  ink-muted: "#685959"
  inverse-canvas: "#413333"
  inverse-ink: "#F5EBDD"
  inverse-ink-muted: "#CBBFA9"
  on-primary: "#F5EBDD"
  on-secondary: "#231818"
  semantic-error: "#B83A2E"
typography:
  display-xl:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "76px"
    fontWeight: 300
    lineHeight: 1.17
    letterSpacing: "-0.5px"
  display-lg:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "60px"
    fontWeight: 300
    lineHeight: 1.17
    letterSpacing: "-0.4px"
  display-md:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "42px"
    fontWeight: 300
    lineHeight: 1.20
    letterSpacing: "0px"
  headline:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "32px"
    fontWeight: 400
    lineHeight: 1.25
    letterSpacing: "0px"
  card-title:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "24px"
    fontWeight: 400
    lineHeight: 1.33
    letterSpacing: "0px"
  subhead:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "20px"
    fontWeight: 400
    lineHeight: 1.40
    letterSpacing: "0px"
  body-lg:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "18px"
    fontWeight: 400
    lineHeight: 1.50
    letterSpacing: "0px"
  body:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.50
    letterSpacing: "0.16px"
  body-sm:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.29
    letterSpacing: "0.16px"
  body-emphasis:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 600
    lineHeight: 1.29
    letterSpacing: "0.16px"
  caption:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.33
    letterSpacing: "0.32px"
  button:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.29
    letterSpacing: "0.16px"
  eyebrow:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.29
    letterSpacing: "0.16px"
  mono:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.33
    letterSpacing: "0px"
rounded:
  none: "0px"
  xs: "2px"
  sm: "4px"
  md: "6px"
  lg: "8px"
  pill: "9999px"
spacing:
  xxs: "4px"
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "24px"
  xl: "32px"
  xxl: "48px"
  section: "96px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-primary-pressed:
    backgroundColor: "{colors.primary-pressed}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-secondary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.inverse-ink}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-accent:
    backgroundColor: "{colors.secondary}"
    textColor: "{colors.on-secondary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-tertiary:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.primary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-ghost:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.primary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  button-danger:
    backgroundColor: "{colors.semantic-error}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.none}"
    padding: "12px 16px"
  badge-accent:
    backgroundColor: "{colors.secondary}"
    textColor: "{colors.on-secondary}"
    typography: "{typography.caption}"
    rounded: "{rounded.none}"
    padding: "4px 8px"
  feature-card:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "24px"
  feature-card-elevated:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "24px"
  product-card:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "32px"
  hero-card:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.display-md}"
    rounded: "{rounded.none}"
    padding: "48px"
  cta-banner:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.headline}"
    rounded: "{rounded.none}"
    padding: "48px"
  resource-tile:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body-sm}"
    rounded: "{rounded.none}"
    padding: "16px"
  customer-logo-tile:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink-muted}"
    typography: "{typography.caption}"
    rounded: "{rounded.none}"
    padding: "24px"
  text-input:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "11px 16px"
  top-nav:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body-sm}"
    height: "48px"
  utility-bar:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink-muted}"
    typography: "{typography.caption}"
    height: "32px"
  footer:
    backgroundColor: "{colors.inverse-canvas}"
    textColor: "{colors.inverse-ink-muted}"
    typography: "{typography.body-sm}"
    padding: "64px 32px"
  telemetry-cell:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.mono}"
    rounded: "{rounded.none}"
    padding: "4px 8px"
---

# Conduit Design System

## Overview

This design system combines the flat geometric discipline of **Carbon Design System** with a warm, organic architectural palette:
- **Warm Parchment Canvas** (`{colors.canvas}` `#F5EBDD`) as the primary ground.
- **Warm Alabaster** (`{colors.surface-1}` `#EFE4D4`) for elevated cards, inputs, and section rhythm.
- **Deep Roasted Espresso** (`{colors.ink}` `#413333`) for authoritative, high-contrast typography (achieving a 10.2:1 contrast ratio that exceeds WCAG AAA).
- **Deep Ocean Steel Blue** (`{colors.primary}` `#315B8C`) as the primary brand accent for links, key CTAs, and focus indicators (5.93:1 contrast on canvas).
- **Warm Terracotta Coral** (`{colors.secondary}` `#F2765E`) as the energetic secondary accent for badges, telemetry indicators, notification states, and warm callouts (paired with `{colors.on-secondary}` `#231818` for 6.19:1 contrast, or `#413333` for 4.31:1 contrast).

The defining choice is **flat geometry**: every CTA, every card, every input, and every container uses square corners (`{rounded.none}` 0px) with thin 1px hairline borders (`#DFD0BD`). There are no rounded pills, no soft diffuse drop shadows, and no atmospheric noise. The system is engineered, tactile, and restrained.

**IBM Plex Sans** carries the entire typographic hierarchy. Display sizes (76 / 60 / 42px) run at weight **300** — a signature light display treatment that makes large headlines feel calm, deliberate, and authoritative. Body type sits at weight 400 with `letter-spacing: 0.16px` (a Carbon precision detail) and line-height 1.50.

The palette balances warmth with precision. Steel Blue anchors structural interactions while Terracotta Coral brings energetic focus to critical data points. Roasted Espresso grounds the entire experience with deep contrast, and inverts at the footer (`{colors.inverse-canvas}` `#413333`) for structural grounding.

**Key Characteristics:**
- **Carbon Geometric Rigor** — Flat 0px square corners across all buttons, cards, and inputs with clean 1px hairline strokes.
- **Warm Architectural Palette** — Warm Cream (`#F5EBDD`), Deep Espresso (`#413333`), Deep Steel Blue (`#315B8C`), and Terracotta Coral (`#F2765E`).
- **Plex Sans 300 Light Display** — Weight 300 for 42–76px headlines delivers quiet authority without aggressive heavy weights.
- **Dual Accent Roles** — `{colors.primary}` Deep Steel Blue owns navigational action and primary CTAs; `{colors.secondary}` Warm Coral owns telemetry, notifications, and alert badges.
- **High-Contrast Grounding** — 10.2:1 text contrast on light ground; footer inverts to `{colors.inverse-canvas}` (`#413333`) with `{colors.inverse-ink-muted}` (`#CBBFA9`).
- **Precision Tracking** — `letter-spacing: 0.16px` on body type provides crisp optical clarity.

## Colors

The palette establishes an organic, warm editorial tone while maintaining rigorous enterprise contrast standards.

### Brand & Accent
- **Deep Ocean Steel Blue** (`{colors.primary}`: `#315B8C`): The primary brand accent. Links, primary CTAs, CTA banner backgrounds, active tab rules, and focus rings. High contrast (5.93:1) on canvas.
- **Blue Hover** (`{colors.primary-hover}`: `#264972`): Hover state for primary buttons and interactive links.
- **Blue Pressed** (`{colors.primary-pressed}`: `#1D3858`): Pressed / active state for primary buttons.
- **Warm Terracotta Coral** (`{colors.secondary}`: `#F2765E`): Secondary brand accent. Urgent telemetry badges, active notification dots, highlight chips, and warm action accents.
- **Coral Hover** (`{colors.secondary-hover}`: `#DF634A`): Hover state for secondary coral buttons and interactive tags.
- **Coral Pressed** (`{colors.secondary-pressed}`: `#C75139`): Pressed state for coral interactive elements.

### Surface
- **Canvas** (`{colors.canvas}`: `#F5EBDD`): Warm cream / parchment foundational page background.
- **Surface 1** (`{colors.surface-1}`: `#EFE4D4`): Subtle warm alabaster — input fields, alternate-row stripes, subtle section bands, elevated cards.
- **Surface 2** (`#E4D6C4`): Slightly deeper warm tint — disabled fields, separator fills.
- **Hairline** (`#DFD0BD`): 1px structural borders on cards, inputs, dividers.
- **Hairline Strong** (`#413333`): 1px espresso underline and strong structural dividers.
- **Inverse Canvas** (`{colors.inverse-canvas}`: `#413333`): Deep roasted espresso — footer surface, inverted full-bleed sections, dark notification bars.
- **Inverse Surface 1** (`#4E3E3E`): One step lighter than inverse canvas — footer column dividers, hovered footer items.

### Text & Ink
- **Ink** (`{colors.ink}`: `#413333`): All headlines, primary labels, and emphasized body type — deep roasted espresso charcoal. Superb contrast on Canvas (10.20:1, exceeding WCAG AAA).
- **Ink Muted** (`{colors.ink-muted}`: `#685959`): Secondary type — metadata, sub-headlines, timestamps, captions (contrast > 5.5:1 on Canvas).
- **Ink Subtle** (`#8C7C7C`): Tertiary type — helper text, placeholder text, disabled states (contrast > 4.5:1 on Canvas).
- **Inverse Ink** (`{colors.inverse-ink}`: `#F5EBDD`): Warm cream on dark espresso — footer headings, inverted hero text (10.20:1 contrast).
- **Inverse Ink Muted** (`{colors.inverse-ink-muted}`: `#CBBFA9`): Warm muted sand on dark espresso — footer links, secondary footer body.

### Contrast & On-Colors
- **On Primary** (`{colors.on-primary}`: `#F5EBDD`): Warm cream on deep slate blue (5.93:1 contrast, passing WCAG AA for normal text and AAA for large text).
- **On Secondary** (`{colors.on-secondary}`: `#231818`): Deep roasted ink on coral (6.19:1 contrast, passing WCAG AA for normal and large text). High-contrast UI pairing `#413333` achieves 4.31:1.

### Semantic
- **Success Green** (`#2D7A54`): Deep earthy green for completed tasks and online states (contrast > 4.5:1 on Canvas).
- **Warning Amber** (`#D97706`): Warm amber for paused states and warnings.
- **Error Red** (`{colors.semantic-error}`: `#B83A2E`): Burnt crimson red for destructive actions, error badges, danger buttons.
- **Info Blue** (`#315B8C`): Aligned with primary slate blue.

## Typography

### Font Family

- **IBM Plex Sans** — Free and open-source under the SIL Open Font License. Geometric, slightly humanist, designed specifically for enterprise UI. Fallback: `Helvetica Neue, Arial, sans-serif`.

The same family carries display, body, and caption. Hierarchy is carried by **size + weight** rather than by family mixing.

### Hierarchy

| Token | Size | Weight | Line Height | Letter Spacing | Use |
|---|---|---|---|---|---|
| `{typography.display-xl}` | 76px | 300 | 1.17 | -0.5px | Largest hero headline |
| `{typography.display-lg}` | 60px | 300 | 1.17 | -0.4px | Section opener headlines |
| `{typography.display-md}` | 42px | 300 | 1.20 | 0px | Sub-section headlines, hero card title |
| `{typography.headline}` | 32px | 400 | 1.25 | 0px | Card collection heading, FAQ category |
| `{typography.card-title}` | 24px | 400 | 1.33 | 0px | Feature card title |
| `{typography.subhead}` | 20px | 400 | 1.40 | 0px | Lead body next to display headlines |
| `{typography.body-lg}` | 18px | 400 | 1.50 | 0px | Hero subhead, lead paragraphs |
| `{typography.body}` | 16px | 400 | 1.50 | 0.16px | Default body |
| `{typography.body-sm}` | 14px | 400 | 1.29 | 0.16px | Card body, footer columns |
| `{typography.body-emphasis}` | 14px | 600 | 1.29 | 0.16px | Selected tab label, emphasized body line |
| `{typography.caption}` | 12px | 400 | 1.33 | 0.32px | Captions, meta, utility bar |
| `{typography.button}` | 14px | 400 | 1.29 | 0.16px | All button labels |
| `{typography.eyebrow}` | 14px | 400 | 1.29 | 0.16px | Section eyebrows (sentence case 14px) |
| `{typography.mono}` | 12px | 400 | 1.33 | 0px | Tabular telemetry, speed counters, and code blocks |

### Principles

- **Light-weight display is the brand voice.** Plex Sans at weight 300 for 76px headlines reads as quietly authoritative — switching to 700 would make it look like every other generic tech site.
- **Carbon`s `letter-spacing: 0.16px`** on body sizes is a precision detail. Do not remove it.
- **Eyebrow typography uses sentence case 14px** — avoids aggressive all-caps tracking.
- **Line-heights tighten on display, relax on body**: 1.17 at display-xl, 1.50 at body.

## Layout

### Spacing System

- **Base unit**: 4px (Carbon signature 4-pixel grid).
- **Tokens**: `{spacing.xxs}` 4px · `{spacing.xs}` 8px · `{spacing.sm}` 12px · `{spacing.md}` 16px · `{spacing.lg}` 24px · `{spacing.xl}` 32px · `{spacing.xxl}` 48px · `{spacing.section}` 96px.
- Card interior padding: `{spacing.lg}` 24px on feature cards; `{spacing.xl}` 32px on product cards; `{spacing.xxl}` 48px on hero cards and CTA banners.
- Button padding: 12px vertical · 16px horizontal.
- Form input padding: 11px vertical · 16px horizontal.

### Grid & Container

- 16-column grid at desktop, scaling to 8 / 4 columns at tablet / mobile.
- Max content width sits around 1584px.
- Card grids are 4-up at desktop, 2-up at tablet, 1-up at mobile.

### Whitespace Philosophy

Alignment adheres to a 4-pixel grid. Sections separate via thin warm alabaster rows (`{colors.surface-1}`) and 1px hairlines (`#DFD0BD`) rather than excessive vertical gaps.

## Elevation & Depth

| Level | Treatment | Use |
|---|---|---|
| 0 (flat) | No shadow, no border | Default for body type, hero text, footer body |
| 1 (hairline) | 1px `#DFD0BD` border on canvas | Feature cards, inputs, list items |
| 2 (surface lift) | `{colors.surface-1}` background on canvas | Alternate-row banners, hovered cards |
| 3 (focus ring) | 2px `{colors.primary}` outline + 1px `#413333` underline | Focused input, focused button |

Surfaces are flat by default. Depth is carried by surface change (`{colors.canvas}` → `{colors.surface-1}`) and 1px hairlines.

### Decorative Depth

- **Soft slate or coral tint washes** appear behind select hero illustrations without competing with typography.
- **No diffuse drop shadows.** Cards do not float; they sit grounded on the warm canvas.

## Shapes

### Border Radius Scale

| Token | Value | Use |
|---|---|---|
| `{rounded.none}` | 0px | Default — every button, card, input, container |
| `{rounded.xs}` | 2px | Small badges (rare exception) |
| `{rounded.sm}` | 4px | Avatars squared, dropdown menus |
| `{rounded.md}` | 6px | Documented for system completeness |
| `{rounded.lg}` | 8px | Documented for system completeness |
| `{rounded.pill}` | 9999px | Telemetry badges and status pills |

The system commits to flat 0px corners as its primary architectural signature.

## Components

### Buttons

**`button-primary`** — Deep Slate Blue solid CTA.
- Background `{colors.primary}`, text `{colors.on-primary}`, type `{typography.button}`, padding 12px 16px, rounded `{rounded.none}`.
- Pressed state: background `{colors.primary-pressed}`.
- Hover state: background `{colors.primary-hover}`.

**`button-secondary`** — Roasted Espresso solid button.
- Background `{colors.ink}`, text `{colors.inverse-ink}`, type `{typography.button}`, padding 12px 16px, rounded `{rounded.none}`.

**`button-accent`** — Warm Terracotta Coral solid CTA.
- Background `{colors.secondary}`, text `{colors.on-secondary}`, type `{typography.button}`, padding 12px 16px, rounded `{rounded.none}`.
- Pressed state: background `{colors.secondary-pressed}`.
- Hover state: background `{colors.secondary-hover}`.

**`button-tertiary`** — Warm cream button with Deep Slate Blue 1px border and text.
- Background `{colors.canvas}`, text `{colors.primary}`, type `{typography.button}`, rounded `{rounded.none}`, padding 12px 16px. Border: 1px `{colors.primary}`.

**`button-ghost`** — Plain text + chevron, transparent background until hover.
- Background `{colors.canvas}`, text `{colors.primary}`, type `{typography.button}`, rounded `{rounded.none}`, padding 12px 16px.

**`button-danger`** — Crimson red destructive variant.
- Background `{colors.semantic-error}`, text `{colors.on-primary}`, type `{typography.button}`, rounded `{rounded.none}`, padding 12px 16px.

### Badges & Chips

**`badge-accent`** — Coral highlight pill / badge.
- Background `{colors.secondary}`, text `{colors.on-secondary}`, type `{typography.caption}`, rounded `{rounded.none}`, padding 4px 8px.

### Cards & Containers

**`feature-card`** — Default feature tile.
- Background `{colors.canvas}`, text `{colors.ink}`, type `{typography.body}`, rounded `{rounded.none}`, padding 24px. Stroked with 1px `#DFD0BD`.

**`feature-card-elevated`** — Elevated card on `{colors.surface-1}` ground.
- Background `{colors.surface-1}`, text `{colors.ink}`, type `{typography.body}`, rounded `{rounded.none}`, padding 24px.

**`product-card`** — Product showcase tile.
- Background `{colors.canvas}`, text `{colors.ink}`, type `{typography.body}`, rounded `{rounded.none}`, padding 32px.

**`hero-card`** — Hero composition card.
- Background `{colors.canvas}`, text `{colors.ink}`, type `{typography.display-md}`, rounded `{rounded.none}`, padding 48px.

**`cta-banner`** — Full-width blue CTA banner.
- Background `{colors.primary}`, text `{colors.on-primary}`, type `{typography.headline}`, rounded `{rounded.none}`, padding 48px.

**`resource-tile`** — Compact resource / article tile.
- Background `{colors.canvas}`, text `{colors.ink}`, type `{typography.body-sm}`, rounded `{rounded.none}`, padding 16px.

**`customer-logo-tile`** — Customer marquee tile.
- Background `{colors.canvas}`, text `{colors.ink-muted}`, type `{typography.caption}`, rounded `{rounded.none}`, padding 24px. 1px `#DFD0BD` border.

### Inputs & Forms

**`text-input`** — Input field with warm alabaster fill.
- Background `{colors.surface-1}`, text `{colors.ink}`, type `{typography.body}`, rounded `{rounded.none}`, padding 11px 16px.
- Focus: 2px `{colors.primary}` underline.
- Error: 2px `{colors.semantic-error}` bottom underline.

### Navigation

**`top-nav`** — Sticky warm cream bar with brand mark and navigation links.
- Background `{colors.canvas}`, text `{colors.ink}`, type `{typography.body-sm}`, height 48px. 1px bottom `#DFD0BD`.

**`utility-bar`** — Slim warm alabaster ribbon above top nav.
- Background `{colors.surface-1}`, text `{colors.ink-muted}`, type `{typography.caption}`, height 32px.

### Footer

**`footer`** — Inverted Roasted Espresso footer.
- Background `{colors.inverse-canvas}`, text `{colors.inverse-ink-muted}`, type `{typography.body-sm}`, padding 64px 32px.

## Do`s and Don`ts

### Do

- Anchor the canvas in warm cream `{colors.canvas}` (`#F5EBDD`) and headlines in roasted espresso `{colors.ink}` (`#413333`) for a 10.2:1 AAA contrast foundation.
- Use `{rounded.none}` 0px on every CTA, card, input, and container.
- Pair Plex Sans weight 300 for display sizes (42px+) with weight 400 for body.
- Reserve `{colors.primary}` Deep Slate Blue (`#315B8C`) for primary CTAs, links, focused-input underlines, and CTA banners.
- Employ `{colors.secondary}` Warm Coral (`#F2765E`) for purposeful accent badges, status pings, and notification highlights with `{colors.on-secondary}` (`#231818` or `#413333`) text.
- Apply `letter-spacing: 0.16px` to body sizes for crisp legibility.
- Use surface stepping (`canvas` → `surface-1`) and 1px hairlines (`#DFD0BD`) for hierarchy. Skip diffuse drop shadows.
- Invert to `{colors.inverse-canvas}` (`#413333`) at the footer with `{colors.inverse-ink-muted}` (`#CBBFA9`).

### Don`t

- Don`t use pure white (`#FFFFFF`) or cold neutral grays (`#161616`, `#F4F4F4`); adhere strictly to the warm parchment and roasted espresso foundation.
- Don`t use light or white text on `{colors.secondary}` (`#F2765E`); always use `{colors.on-secondary}` (`#231818` / `#413333`) to ensure high contrast (>4.3:1).
- Don`t round corners on buttons, cards, or inputs. Flat 0px geometry is essential to the brand.
- Don`t bold display headlines. Plex Sans at weight 300 is the brand voice; weight 700 makes it generic.
- Don`t add atmospheric diffuse shadows or glossy glassmorphism.
- Don`t introduce arbitrary third or fourth accent hues outside the documented palette (`#F5EBDD`, `#F2765E`, `#315B8C`, `#413333`).

## Responsive Behavior

### Breakpoints

| Name | Width | Key Changes |
|---|---|---|
| Max | 1584px | Max grid; gutters expand |
| Desktop-XL | 1312px | Default desktop layout |
| Desktop | 1056px | Card grid 4-up maintained |
| Tablet | 672px | Card grid 4-up → 2-up; nav becomes hamburger |
| Mobile | 320px | Single-column; display-xl scales 76px → ~32px |

### Touch Targets

- Minimum 48px tap target on touch viewports for buttons and inputs.
- Top-nav links grow to 48px tap height on touch.

### Collapsing Strategy

- **Top nav**: links collapse to hamburger drawer below 672px.
- **Utility bar**: hides below 672px to reclaim vertical space.
- **Card grid**: 4-up → 2-up at 1056px → 1-up below 672px.
- **Display type**: `{typography.display-xl}` 76px scales toward 42px on mobile while preserving weight-300 treatment.
- **Footer**: 6-column link grid → 3-column at tablet → 1-column at mobile.

## Iteration Guide

1. Focus on ONE component at a time and reference it by its `components:` token name.
2. Default body to `{typography.body}` at weight 400 with `letter-spacing: 0.16px`.
3. When introducing a new section, decide whether it sits on `{colors.canvas}` (#F5EBDD) or on `{colors.surface-1}` (#EFE4D4).
4. Run `npx @google/design.md lint docs/DESIGN.md` after edits.
5. Add new variants as separate component entries (`button-primary-pressed`, `button-accent`, etc.).
6. Keep `{colors.primary}` (#315B8C) and `{colors.secondary}` (#F2765E) purposeful; avoid decorative color clutter.
7. Resist rounded corners — flat 0px geometry defines the system.

## Known Gaps

- Data visualization palettes (multi-series charts) should derive categorical steps from tonal variations of slate blue, coral, espresso, and warm amber.
- Full high-contrast dark mode inversion token set (inverting canvas to `#413333` throughout) can be defined as an alternate theme mapping.
