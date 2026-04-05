# PlexTracker — UI/UX Design Spec

Companion to the main design spec (`2026-04-01-plextracker-design.md`). Defines the visual language, navigation patterns, screen layouts, and interaction design for the mobile-only Preact SPA.

## Design Principles

- **Mobile-only Android PWA** — target device: Pixel 9 Pro, Chrome
- **Thumb-first**: all primary actions within bottom-half thumb zone. No top navbars.
- **Dark mode first** — light theme deferred to a later release
- **Font**: DM Sans (Google Fonts), single font for all text
- **Content-forward**: covers and progress are the visual anchors, UI chrome stays minimal

## Color System

### Dark Theme

| Token | Hex | Usage |
|-------|-----|-------|
| `bg-primary` | `#0D0D0D` | Main background |
| `bg-card` | `#161616` | Card backgrounds |
| `bg-surface` | `#1E1E1E` | Inactive pills, input backgrounds, buttons |
| `border-subtle` | `#1A1A1A` | Navbar borders, section dividers |
| `border-card` | `#222222` | Card borders (watched items) |
| `text-primary` | `#F0F0F0` | Titles, primary text |
| `text-secondary` | `#666666` | Metadata, timestamps |
| `text-muted` | `#555555` | Inactive nav labels, placeholders |
| `text-dimmed` | `#444444` | Scale markers, disabled states |

### Accent Colors

| Token | Hex | Usage |
|-------|-----|-------|
| `accent-amber` | `#E8A925` | Library nav, progress bars, active filters, watched checkmarks, primary CTAs |
| `accent-teal` | `#38BDB0` | Search nav active |
| `accent-green` | `#4CAF50` | Add nav active, confirmed/completed states |
| `accent-lavender` | `#9575CD` | Stats nav active |
| `accent-coral` | `#EB5757` | Title detail action bar active, unconfirmed states, review badge |
| `accent-blue` | `#5B9CF6` | Library filter bar active |
| `accent-imdb` | `#F5C518` | IMDb branding |
| `accent-anilist` | `#02A9FF` | AniList branding |

Each accent color has a 12% opacity variant for background wash (e.g. `rgba(232,169,37,0.12)` for amber).

## Navigation

### Main Bottom Navbar

Full-width, fixed at bottom. 4 tabs: **Library**, **Search**, **Add**, **Stats**.

- Each tab has its own identity color (amber, teal, green, lavender)
- Active state: full-height column highlight (2px top border + background wash at 12% opacity + colored icon + colored label)
- Inactive state: no background, `#555` icon and label
- Icons: Lucide-style SVG line icons (stroke, not fill)
  - Library: monitor icon
  - Search: magnifying glass
  - Add: circle-plus
  - Stats: bar chart
- Height: standard (~56px including safe area)

### Secondary Bars (above main navbar)

Two contextual bars share the same compact style and sit above the main navbar:

**Library Filter Bar** (visible on Library screen):
- Same compact height as the action bar (~28px content)
- Full-width segmented items: All, Watching, Up to date, Completed, Dropped, Plan (to watch)
- Active item: `accent-blue` (`#5B9CF6`) with full-height highlight (same pattern as navbar: 2px top border + wash + colored text)
- Inactive items: `#555` text, no background
- Sticky position, always visible when on Library tab

**Title Detail Action Bar** (visible on Title Detail screen):
- Compact height, single row, icon+label inline (no stacking)
- Items: **S02E06** (next unwatched episode, primary action), **IMDb** (open in browser), **AniList** (sync), **Rate** (open rating bottom sheet)
- S__E__ button: shows next unwatched episode in `SXXEXX` format. Refreshes to next episode after tapping. Shows tick icon when all episodes are watched.
- Active item (S__E__): `accent-coral` (`#EB5757`) with full-height highlight
- IMDb and AniList show in their brand colors but are not "active" highlighted
- Rate: `#555` icon + label when no rating, colored when rated

## Screens

### Library (Home)

**Header**: "Library" title, left-aligned, `20px` font-weight-700.

**Match Review Banner**: Inline alert card below header, visible only when `pending_review` or `unconfirmed` titles exist.
- Background: `rgba(235,87,87,0.1)` with `rgba(235,87,87,0.25)` border
- Left: red count badge (circle), right: chevron arrow
- Shows: "X titles need review" + breakdown "N pending · M unconfirmed"
- Tapping opens the Match Review screen
- Disappears when all matches are resolved

**Content — "All" tab** (default): Single scroll with section headers. Sections ordered top-to-bottom: **Completed**, **Plan to watch**, **Dropped**, then **Watching / Up to date** — poster grid sections first (browsing), watching cards last (closer to thumb zone for quick actions).
- **Completed / Dropped / Plan to watch** sections: poster grid (3 columns)
  - Cover fills the cell (aspect-ratio 2:3, 8px radius)
  - Bottom gradient overlay with title name
  - No progress bar or action button (just browsing)
- **Watching / Up to date** sections: horizontal list cards
  - Cover thumbnail (48×68px, 6px radius) + title + year/type + progress bar + next episode badge
  - Progress bar: `accent-amber` on `#2A2A2A` track, 3px height
  - Next episode badge: 34px amber circle with episode number (e.g. "E8"), or play icon for movies

**Content — filtered tabs** (Watching, Up to date, etc.): Shows only titles for that status, using the appropriate card style (horizontal for watching/up-to-date, poster grid for the rest).

**Filter bar**: Sticky above main navbar (see Navigation > Secondary Bars).

### Title Detail

**Hero Cover**: 160px height, gradient fade to `bg-primary` at bottom. Contains:
- Back button: top-left, 32px circle, `rgba(0,0,0,0.5)` background
- Edit button: top-right, 32px circle, same style. Opens Edit bottom sheet.
- AniList badge: next to edit button (top-right), 32px circle, same style. Shows "AL" text in `accent-anilist`. Amber dot = pending match. Red dot = AniList not connected. No dot = confirmed/synced. Tapping always opens AniList bottom sheet (for match review, re-sync, or connect flow). Visible only on anime titles. Note: this badge is for match/connection status — the AniList button in the action bar below is for quick sync access.
- Title name: 20px bold white, over gradient
- Metadata line: type · year · series status (no rating here — rating shown only in the action bar)

**Progress Bar**: Below hero. Amber on dark track, with text "S2 · 7 of 10 episodes watched".

**Season Tabs**: Horizontal row of pills.
- Completed season: green checkmark icon + "S1" + inline rating "★ 8"
- In-progress season (active): amber pill with progress fill circle + "S2" + fraction "7/10"
- Unwatched season: default `bg-surface` pill

**Episode List**: Vertical list of episode rows.
- Each row: episode number + name, right-aligned checkmark (amber filled = watched, empty bordered square = unwatched)
- Tap the row or checkmark to toggle watched state
- Background: `bg-card` with `border-card` for watched, `#1E1E1E` border for unwatched

**Action Bar**: Above main navbar (see Navigation > Secondary Bars).

### Match Review

**Header**: Back arrow + "Match Review" + count badge ("3 remaining").

**Batch Confirm Button**: At top, confirms all `pending_review` items at once. Green text on `bg-surface`.

**Sections** (unconfirmed first, then pending):

**Unconfirmed** (red section):
- Section header: "Unconfirmed · needs attention" in `accent-coral`
- Card border: `rgba(235,87,87,0.25)`
- Cover thumbnail (or "?" placeholder if no cover) + Plex title + type
- Gemini confidence box: shows "low confidence" in red + reasoning text
- External IDs shown as tappable links in their brand colors (IMDb `#F5C518`, AniList `#02A9FF`, TMDB in blue)
- Actions: Confirm (green) | Fix match (blue)

**Pending review** (amber section):
- Section header: "Pending review · likely correct" in `accent-amber`
- Card border: `rgba(232,169,37,0.2)`
- Same layout but shows "high confidence" in green
- Same actions: Confirm | Fix match

### Rating Prompt (Bottom Sheet)

Triggered when:
- Movie or season finale marked as watched (auto or manual)
- Status changed to `dropped` or `completed`
- Tapping "Rate" in the action bar

**Layout**:
- Drag handle at top
- Title text: "Season X completed!" or "Movie watched!" or just title name when rating manually
- Title name as link to detail page
- **10 stars horizontal row**, generously spaced for mobile tap targets. Tap to fill. Amber filled, `#333` empty.
- Large rating number display below stars (e.g. "8/10")
- Action buttons:
  - "Save rating" — saves locally
  - "Save & rate on IMDb" — saves locally, opens IMDb page in browser
  - "Save & sync AniList" — saves locally, syncs via API (only for anime with confirmed AniList match)
  - "Skip for now" — dismisses without rating

### AniList Bottom Sheet

Triggered by tapping the AniList badge on the title hero.

**Layout**:
- Drag handle
- Header: "AniList Match" + status badge (Pending/Unconfirmed)
- Matched entry card: cover thumbnail + romaji title + English title + year + AniList link (tappable, `accent-anilist`)
- Confidence box: match confidence level + Gemini reasoning
- Actions: "Confirm & Sync" (AniList blue) | "Wrong match" (muted)

**States**:
- Not connected: sheet shows "Connect AniList" button, redirects to OAuth flow
- Pending match: shows matched entry with confirm/reject
- Confirmed: sheet shows synced status, option to re-sync or unlink

### Edit Bottom Sheet

Triggered by tapping the pencil button on the title hero.

**Layout**:
- Drag handle
- Header: "Edit Title"
- **Type selector**: 3 buttons in a row (Movie, Series, Anime). Active = amber highlight with border. Anime is essentially a series but flagged separately. Documentaries are either movies or series.
- **Status selector**: 4 buttons (Watching, Completed, Dropped, Plan). Active = amber highlight.
- **Display name**: Radio list of all multilingual names from `title_names`. Shows name + language tag (EN, FR, Romaji, etc.). Active = amber radio dot + highlighted border.
- "Save changes" button (amber, full-width)
- Changing status to completed/dropped → triggers rating prompt after save
- Changing status to completed → marks all episodes as watched

### Add Title

Thin input screen — just a text field. All the actual display/confirmation logic lives in Title Validation. Mockup: `add-screen.html` (Proposal B).

**Layout**:
- Header: "Add Title" + subtitle "Paste an IMDb, TVDB or AniList link — or search by name"
- Input field with search icon + amber submit arrow button, positioned at the **bottom** just above the navbar, within thumb zone
- Empty state above: centered plus icon + hint text + dimmed brand names (IMDb · TVDB · AniList)
- URL detection: IMDB (`tt` ID), TVDB (series/movie slug), AniList (anime ID)
- Text search: searches TMDB/AniList by name
- On submit → navigates to `/validate?q=<url-or-query>`

### Title Validation (`/validate?q=...`)

**Single shared screen** for all entry points:
- **Add screen** → submits URL or search query → navigates here
- **Share Target** → Android share sheet sends URL → opens directly here (skips Add input)
- **Match Review "Fix match"** → navigates here with existing title context

Route: `/validate?q=<url-or-query>[&title_id=<id>]` (`title_id` present only for match fix flow).

**Loading state**: Spinner while the matching pipeline resolves the URL/query into external IDs and fetches metadata from TMDB/AniList.

**Layout** (similar to Title Detail but pre-confirmation):
- Cover image (from TMDB) + title name + year + type
- Resolved external IDs shown as tappable links in brand colors (IMDb, TMDB, AniList)
- If arriving from Match Review: Gemini confidence box + reasoning

**States**:
- **New title**: Status picker (Watching, Already watched, Plan to watch) defaulting to **Plan to watch** + "Add to library" button (amber, full-width). "Already watched" → marks all episodes watched, sets status completed, triggers rating prompt.
- **Already in library**: Shows current status badge + "View in library" link to existing title detail. No duplicate add.
- **Match fix** (from Match Review with `title_id`): "Confirm match" (green) or manual ID entry fields to correct the match.

### Search

Global search across all titles regardless of status filter. Mockup: `search-screen.html` (Proposal A).

**Layout**:
- Search input at **bottom**, just above the navbar, within thumb zone. **Auto-focuses** when navigating to the Search tab via navbar. Results scroll above it.
- Results as compact rows: cover thumbnail (42×60px) + title + inline status badge (colored pill) + metadata line (type · year · progress or rating) + chevron
- Status badges: amber filled for Watching, green outline for Completed, muted for Plan to watch
- Tapping a result navigates to Title Detail
- Results count shown above list

### Stats

TBD — to be designed in a dedicated session. Placeholder tab for now.

## Interaction Patterns

### Bottom Sheets

All bottom sheets share:
- 4px rounded drag handle, centered, `#333`
- Background: `bg-card` (`#161616`)
- Top corners: 16px radius
- Backdrop: dimmed `bg-primary` at 40% opacity
- Slide up animation from bottom

### Quick Mark (Library)

Tapping the circular episode badge on a library card:
1. Marks that episode as watched
2. Badge updates to next unwatched episode number
3. If it was a season finale or movie → rating prompt slides up
4. Progress bar animates to new value

### Transitions

- Library → Title Detail: standard push (slide in from right)
- Any → Bottom Sheet: slide up from bottom with backdrop fade
- Filter tab switch: cross-fade content (no horizontal slide to avoid PWA gesture conflict)

## Mockup Reference

All approved mockups are saved in `docs/mockups/` within the project directory.

### Final versions (approved)

| File | Screen | Decision |
|------|--------|----------|
| `add-screen-v2.html` | Add Title | Proposal A — bottom input, centered empty state with brand hints |
| `title-validation-v2.html` | Title Validation | Proposal A — hero cover, actions anchored at bottom, "Plan" default |
| `library-v2.html` | Library | Proposal A — filter bar at bottom above navbar, completed sections before watching |
| `title-detail-v6.html` | Title Detail | Proposal B — chronological episodes, AniList badge, edit button, no rating in hero (rating in action bar only) |
| `rating-prompt-v3.html` | Rating Prompt | Proposal A — faded cover hero backdrop, single row of 10 stars |
| `match-review-v2.html` | Match Review | Proposal A + left color border from B, clickable ID tag chips |
| `edit-sheet-v2.html` | Edit Sheet | Proposal A — compact rectangular buttons, 3 types (Movie/Series/Anime) |
| `anilist-sheet-v2.html` | AniList Sheet | Proposal B — side-by-side comparison (library vs AniList) |
| `search-screen-v2.html` | Search | Both states: empty (centered hint + auto-focused input) and results (compact rows with status badges) |

### Earlier iterations (reference only)

| File | Notes |
|------|-------|
| `visual-mood-v3.html` | Rich & immersive dark theme with amber accent |
| `navbar-v3.html` | Per-tab identity colors (interactive) |
| `card-layout.html` | Hybrid cards: horizontal for watching, poster grid for others |
| `filter-pills-compact.html` | Compact filter bar style reference |
| `all-tab.html` | "All" tab layout with section headers |
| `match-review-access.html` | Match review banner access point |
| `title-detail-v5.html` | Earlier title detail with S__E__ format in action bar |
| `title-editing.html` | Earlier edit + AniList validation combined |
| `anilist-compact.html` | Earlier AniList badge + sheet |
| `rating-input.html` | Star/slider/grid comparison |
| `swipe-filter-v2.html` | (Rejected: PWA gesture conflict) |
