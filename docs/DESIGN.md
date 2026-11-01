---
name: Conduit Departures Board
description: The download queue as a departures board: a signal-yellow next arrival, departure lines, and status as a lamp and a word.
colors:
  board: "#F3F3EF"
  panel: "#FFFFFF"
  track: "#E4E4DE"
  hover: "#EAEAE4"
  hairline: "#CFCFC8"
  row-rule: "#E2E2DC"
  ink: "#111111"
  ink-secondary: "#4A4A46"
  ink-muted: "#5E5E58"
  signal: "#F5C400"
  signal-hover: "#FFD433"
  signal-pressed: "#D9AE00"
  complete: "#1E6B3E"
  paused: "#8A5A00"
  error: "#B42318"
  error-hover: "#91261D"
  night-board: "#0E0E0E"
  night-panel: "#1A1A1A"
  night-ink: "#F4F4F0"
  night-complete: "#7BC596"
  night-paused: "#FFA94D"
  night-error: "#FF7B6E"
typography:
  display:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "6rem"
    fontWeight: 700
    lineHeight: 1
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.25
  title:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.5
  body:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0"
  row-name:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "1rem"
    fontWeight: 700
    lineHeight: 1.35
  label:
    fontFamily: "IBM Plex Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica Neue, Arial, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "0.06em"
  mono:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.4
rounded:
  none: "0px"
  pill: "9999px"
spacing:
  xxs: "4px"
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0 16px"
    height: "36px"
  button-primary-hover:
    backgroundColor: "{colors.signal-hover}"
    textColor: "{colors.ink}"
  button-secondary:
    backgroundColor: "{colors.track}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0 16px"
    height: "36px"
  button-danger:
    backgroundColor: "{colors.error}"
    textColor: "#FFFFFF"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0 16px"
    height: "36px"
  tab:
    textColor: "{colors.ink-secondary}"
    typography: "{typography.body}"
    height: "44px"
  tab-selected:
    textColor: "{colors.ink}"
  next-arrival:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    typography: "{typography.display}"
    padding: "24px"
  queue-row:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    padding: "16px"
  status-badge:
    textColor: "{colors.ink}"
    typography: "{typography.label}"
  progress-rule:
    backgroundColor: "{colors.track}"
    height: "4px"
---

# Design System: Conduit Departures Board

## Overview

**Creative North Star: "The Departures Board"**

The download queue reads like a departures board. The next arrival is set large, in minutes, on a signal-yellow band. Every other download is a departure line: its name and folder on the left, its figures in plain numerals, and its state as a lamp and an uppercase word on the right. The visitor learns in one glance what finishes next and which downloads need them.

The system is precise and quiet. It refuses the category default of a filter sidebar over a table of progress bars. Colour does real jobs and nothing decorative: yellow means the next arrival or the thing to press, and the status colours mean the state of a line. Surfaces are flat. Depth comes from the board ground, a white panel, and hairline rules.

The night board is the same signage on black. The scheme follows the system until the user pins light or dark with the theme button in the top bar. That choice is one daemon setting, so the dashboard, the extension popup and the offline page all show it.

**Key Characteristics:**
- Signal yellow marks only the next arrival and the primary action.
- Status is a lamp plus an uppercase word, never colour alone.
- Figures are tabular, and monospaced where they count.
- Square corners, hairline rules, no shadows.
- Rows keep queue order. They move only when the queue changes.

## Colors

Warm-free and high-contrast: an off-white board, white panels, and ink. Yellow is the only accent.

### Primary
- **Signal Yellow** (#F5C400): The next arrival band and the primary action button. Hover #FFD433, pressed #D9AE00. Ink text on yellow is 12:1 or better.
- **Signage Ink** (#111111): All primary text, the header rule, the focus ring in the light theme, and the selected tab underline.

### Secondary
- **Secondary Ink** (#4A4A46): Labels and file paths, 8:1 on the panel.
- **Muted Ink** (#5E5E58): Meta text and placeholders, 6:1 on the panel.

### Neutral
- **Board Ground** (#F3F3EF): The page ground.
- **Panel White** (#FFFFFF): Queue and control surfaces.
- **Track Grey** (#E4E4DE): Progress tracks, table headers, and secondary buttons. Hover #EAEAE4.
- **Hairline** (#CFCFC8): 1px structural borders.
- **Row Rule** (#E2E2DC): Rules between queue lines.

### Status
- **Delivered Green** (#1E6B3E): Completed status word, 6.6:1 on the panel.
- **Held Amber** (#8A5A00): Paused status word, 5.8:1 on the panel.
- **Returned Red** (#B42318): Failed status word and error text, 6.6:1 on the panel. The destructive button uses the same red with white text, and hover #91261D.

### Night board
- **Night Board** (#0E0E0E), **Night Panel** (#1A1A1A), **Night Ink** (#F4F4F0). Status: Night Delivered #7BC596, Night Held #FFA94D, Night Returned #FF7B6E. The focus ring turns signal yellow.

### Named Rules
**The One Yellow Rule.** Signal yellow marks the next arrival and the primary action. Nothing else on the page is yellow. Its rarity is what makes the next arrival readable.

## Typography

**Display Font:** IBM Plex Sans (self-hosted, latin subset), with the system UI stack as fallback.
**Figures Font:** the system monospace stack, used for counts, speeds, and times.

**Character:** a single humanist sans, set heavy for the things that must be read first and regular for everything else. The figures are monospaced so columns of numbers line up.

### Hierarchy
- **Display** (700, 6rem, line-height 1, tracking -0.02em): The next arrival's clock, minutes and seconds. Only that figure. Below 640px the same role sets at 4.5rem so the clock fits a phone.
- **Headline** (700, 1.5rem, line-height 1.25): The next arrival's filename.
- **Title** (600, 1.125rem, line-height 1.5): The page title and the queue heading.
- **Body** (400, 0.875rem, line-height 1.5, tracking 0): Meta text, labels, and buttons. Body text stays at zero tracking; only the uppercase labels and the monumental figure are tracked.
- **Row name** (700, 1rem, line-height 1.35): The filename on each queue line.
- **Label** (700, 0.75rem, tracking 0.06em, uppercase): Status words and metric labels.
- **Figures** (monospace, 0.75rem to 0.875rem, tabular numerals): Speed, time left, progress, counts.

### Named Rules
**The Monumental Minute Rule.** The next arrival figure is the largest type on the screen. No other figure is set above 1.5rem.

## Layout

A vertical stack. The top bar holds the brand and the connection state on the left, and the header actions on the far right. Below 1000px the metric cards take their own full-width row. Below it sit the filter tabs and the search field. The main area leads with the next arrival band, then the queue panel: a toolbar with the batch actions, a strip of the three queue totals, then the table.

The spacing scale is a 4px base: 4, 8, 12, 16, 24, and 32px. The next arrival band uses 24px padding. Queue lines use 16px padding and a 1px row rule.

Breakpoints:
- 1000px: metric cards sit in a row beside the title.
- 900px: queue lines become stacked cards, and controls reflow.
- 640px: metric cards stack the label over the figure. The next arrival figure drops to 4.5rem. Batch and add buttons reach 44px.
- 380px: smallest phones keep one column.
- Touch (`pointer: coarse`): buttons and tabs reach 44px.

Under 900px, each queue line becomes a stacked card in the same order: name, progress, speed and ETA, state, actions.

## Elevation & Depth

Flat. The system uses no box shadows. Depth comes from changes of ground (board to panel to track), hairline rules, and the scrim behind dialogs: rgba(17, 17, 17, 0.55) in light and rgba(0, 0, 0, 0.6) in dark.

### Named Rules
**The Flat Board Rule.** Surfaces are flat at rest. Depth is a change of ground or a rule, never a shadow.

## Shapes

Square corners everywhere: buttons, tabs, panels, inputs, progress rules, and dialogs use 0px radius. The one exception is the connection state pill in the top bar, which is fully rounded and marks a live status, not a control.

The brand mark is an SVG converging-halves symbol in signal yellow and ink.

The app icon is the same symbol on a tile: a white tile with ink (the yellow and white icon) or a black tile with off-white (the yellow and black icon), both with signal yellow. The dashboard tab icon follows the theme choice. The extension and desktop icon files are the white tile, since they cannot follow a choice.

## Components

### Buttons
- **Primary:** signal yellow fill, ink text, 0px radius, 36px tall, 0 16px padding. Hover #FFD433, pressed #D9AE00. Used once per view, for Add download.
- **Secondary:** track grey fill, hairline border, ink text. Used for Pause All, Resume All, and Clear Finished.
- **Danger:** returned red fill, white text. Used only to confirm a removal.
- **Focus:** a 2px ink outline, offset 2px, in the light theme; a 2px signal-yellow outline in the dark theme.

### Filter Tabs
- **Shape:** text tabs, 44px tall, with a 2px underline on the selected tab.
- **Selected:** ink text and an ink underline. The count sits on signal yellow.
- **Unselected:** secondary ink text. Count in a white box with a hairline border.

### Next Arrival Band
- **Active:** full-width signal-yellow band, ink text. The filename, a meta line with speed and connections, and the time left as a clock in the display figure: `2:14` under an hour, `1:02:14` from an hour. Minutes and seconds are NumberFlow figures, and the seconds field is always two digits.
- **Idle:** a white panel with a hairline border and the message in ink. No yellow.
- **Waiting:** an active download with no speed yet keeps the yellow band, names the file, and shows "--" with "Waiting for data".

### Status
- **Shape:** a lamp and a word. The lamp is an 8px square; the word is uppercase, bold, and tracked.
- **Colour:** Active uses an ink lamp and ink text, so yellow stays on the next arrival and the primary action. Paused, Failed, and Completed use their status colour for both lamp and word.

### Queue Lines
- **Shape:** table rows separated by 1px row rules, not boxes. Headers sit on a 2px ink rule.
- **Content:** name on top, folder beneath in small text; the connection count and the byte counts as meta under progress. Speed, ETA and byte counts are NumberFlow figures with their units as text. An idle line shows a dash.
- **Progress:** a 4px rule under the progress figure. Ink while moving, held amber when paused, returned red on failure.
- **Motion:** a newly changed row flashes its background for 1s. It is removed under reduced motion.

### Metric Cards
- **Shape:** three cells in one strip on the queue panel, between the toolbar and the table. Hairline rules divide the cells, and the strip takes the panel's ground. Each cell has a label and a figure in the mono stack.
- **Active speed:** the speed figure is ink, not yellow, to keep yellow for the next arrival. Its number is a NumberFlow figure, and its unit stays text.

### Dialogs and Toasts
- **Dialogs:** white panels, hairline borders, 0px radius, the scrim behind. Focus is trapped inside.
- **Toasts:** white panels with a status word. Errors stay for 7 seconds, and a toast with actions holds while it is hovered or focused.

### Extension Surfaces
The browser extension's popup and its offline page follow this system. The extension ships without the dashboard stylesheet, so each keeps its own copy of the tokens and the IBM Plex file under `extension/fonts`. Status is a lamp and a word, the one primary action is signal yellow with ink text, and held and active downloads are departure rows, not boxes. The offline page shows the connection state, the held queue, and the retry action. It carries no setup steps. Each surface has the same theme button, which cycles system, light and dark. The daemon's `theme` setting is the source; the extension keeps a copy so its pages paint correctly while the daemon is off, and a change made while it is off is sent once the daemon answers. The in-page download toast has no button. It reads the same choice from extension storage, which the popup and offline pages keep in step, and it uses the white and black tiles from the board palette. The desktop splash follows the system setting only, since it cannot read the dashboard's saved choice.

### Motion
Motion follows Apple's fluid-interface rules. The dashboard's springs run in `web/motion.js`, and NumberFlow is the only vendored library (see Vendored libraries). Only transform and opacity move, except height, which moves only where a box takes or gives space (the next arrival band, the connection banner's frame and the empty queue's frame). Nothing loops except the offline radar, which pulses twice and rests.
- **Press:** a button scales to 0.97 on pointer-down over 100ms, on an ease-out curve, and releases on the same curve. Hover changes colour only. The close button and the extension popup's small buttons press the same way.
- **Moves (queue reorder, tab underline):** a critically damped spring with a response of 0.4s. It reaches about 99% in 0.4s and never overshoots. A retarget carries the velocity already in flight, so an interrupted move continues.
- **Next arrival:** the filename enters on a critically damped spring with a response of 0.3s, rising 10px. The clock's digits roll on their own with NumberFlow. When the band changes state, its height eases to the new size on the 0.4s move spring, so a resumed download reshapes the band instead of jumping.
- **Queue lines:** a new line fades in over 260ms. A line removed by a confirmed delete or clear fades out over 160ms while the request runs, and fades back in if the request is refused. A filter or search that hides lines removes them at once.
- **Dialogs:** enter over 260ms on cubic-bezier(0.16, 1, 0.3, 1) from 8px below. Exit over 160ms on the inverse curve, cubic-bezier(0.7, 0, 0.84, 0), back down the same 8px.
- **Drawer:** slides in from the right edge over 320ms and out over 220ms on the inverse curve.
- **Toasts:** rise 12px over 260ms and drop back the same 12px over 160ms.
- **Connection banner:** rises 8px and fades in over 260ms on the enter curve, and leaves up the same 8px over 160ms on the exit curve. Its frame takes the height on the move spring, so the queue below glides instead of jumping.
- **Setup:** the folder step leaves upward over 160ms, then the extension step rises in on the enter spring. The continue button and the browser steps rise 12px over 260ms.
- **Empty queue:** the first-time message, with no downloads at all, rises 12px on the enter spring once the page has booted. Search and filter empties stay still, since they change on every keystroke. The loading, waiting and first-run text fades out and back in (100ms, then 180ms), and the box eases to its new height on the move spring.
- **Theme:** the icon cross-fades over 160ms. The page cross-fades over the same time, so a change of colour scheme is not a one-frame flip. The extension popup and the offline page do the same.
- **Progress rules:** move on a 300ms ease-out. They show data rather than a gesture, so they take no spring.
- **Reduced motion:** all movement is removed. Fades remain. The underline and the reorder land at once.
- **Bootup:** when the dashboard opens, its elements arrange into place once: the header, the metric cards, the filters and search, the next arrival band, then the queue lines. Each rises 12px and fades in on a critically damped spring with a 0.3s response, staggered about 40ms per group and 30ms per line. The last queue line starts within 0.5s and settles about 0.6s later. A click during it takes effect at once. Under reduced motion it is a fade with no stagger.
- **Figures:** NumberFlow rolls a changed digit to its new value over 300ms on the ease-out curve cubic-bezier(0.16, 1, 0.3, 1). This applies to the header, the strip, the band and every queue line. A queue line rolls only while it is on screen; a line off screen keeps its values and brings them up to date when it comes into view. It respects reduced motion and animates only while the page is visible.

### Vendored libraries
A library may be added when it is vendored under `daemon/internal/server/web/vendor/<name>/`, with its licence and pinned version beside it. The daemon serves it from the binary. The page loads it from `/vendor/`, never from a CDN or any external host, and the dashboard must work with no network.
- **NumberFlow 0.6.2** (MIT): the speed figure and the time-left figure. Bundled from `dist/index.mjs` into `vendor/number-flow/number-flow.js`, with the licence beside it. The full build takes numbers through `update()` with an Intl format, which the seconds field needs. The lite build takes only pre-formatted parts. The bundling script is a dev-only tool in `tools/vendor-numberflow/` and is not shipped.

## Do's and Don'ts

### Do:
- **Do** show every status as a lamp and a word together.
- **Do** keep queue lines in queue order, and move a line only when the queue changes.
- **Do** use tabular figures for every speed, time, and count.
- **Do** keep the next arrival band the only yellow on a working screen, apart from the primary action.
- **Do** keep touch targets at 44px on phones and touch screens.

### Don't:
- **Don't** use signal yellow for anything except the next arrival and the primary action.
- **Don't** add box shadows, gradients, or rounded corners to panels, buttons, or lines.
- **Don't** put a kicker or eyebrow label above a heading.
- **Don't** rely on colour alone to show a state.
- **Don't** animate a row's position. Rows reorder only when the queue reorders.
