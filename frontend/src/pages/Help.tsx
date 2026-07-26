import { useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import s from './Help.module.css'

interface FaqItem {
  q: string
  a: ComponentChildren
}

interface FaqSection {
  label: string
  items: FaqItem[]
}

const sections: FaqSection[] = [
  {
    label: 'Fixing & updating matches',
    items: [
      {
        q: 'PlexTracker matched a title to the wrong thing — how do I fix it?',
        a: (
          <>
            Open the title, pull up the <strong>Actions</strong> bar at the bottom of the screen,
            tap <strong>More</strong>, then <strong>Rematch</strong>. Paste a TMDB / IMDb /
            TheTVDB / AniList URL, or search by name. Saving rematches the title immediately.
          </>
        ),
      },
      {
        q: 'What does "Pending review" / "Unconfirmed" mean? Should I do something?',
        a: (
          <>
            PlexTracker tried to identify the title automatically but isn't sure enough to
            confirm it on its own.
            <br />
            <strong>Unconfirmed</strong> — medium or low confidence, or AI verification failed;
            take a look and confirm or fix the match.
            <br />
            <strong>Pending review</strong> — falls back to manual confirmation when the AI
            verifier (Gemini) is unavailable. Same action: confirm or fix.
            <br />
            High-confidence AI-verified matches are confirmed automatically and never reach this
            queue. Both statuses appear in the <strong>Match Review</strong> screen (a red banner
            shows on Library home when any are waiting).
          </>
        ),
      },
      {
        q: 'On the Match Review screen, what’s the difference between Confirm and Fix match?',
        a: (
          <>
            <strong>Confirm</strong> keeps the proposed match.{' '}
            <strong>Fix match</strong> opens the same panel as the title page so you can paste a
            URL or re-search. Swiping a card left also exposes both as quick buttons —
            swiping all the way left runs Confirm.
          </>
        ),
      },
      {
        q: 'How do I link or change the AniList entry for one specific season?',
        a: (
          <>
            On a multi-season anime, the active season shows a blue strip with its AniList
            community score. Tap the pencil <strong>✎</strong> in that strip to change the entry,
            or tap <strong>Link entry</strong> if the season is unmapped. AniList sync starts
            using the new entry immediately.
          </>
        ),
      },
      {
        q: 'Why is my Match Review list so much shorter after an import?',
        a: (
          <>
            High-confidence matches — those AI-verified against TMDB, AniList, or a fuzzy
            search — are <strong>confirmed automatically</strong> and never enter the queue.
            Only titles the app isn't sure about land in Match Review, so the list stays
            focused on the ones that actually need a human look.
          </>
        ),
      },
      {
        q: 'What is the "Recently auto-matched" section on the Match Review screen?',
        a: (
          <>
            A read-only log (newest first) of titles the app confirmed automatically or anime
            seasons it attached to their main series without prompting you. Each row shows the
            original import name and what it resolved to. Nothing to action — it's there for
            spot-checking. If something looks wrong, tap <strong>Fix match</strong> on that row.
          </>
        ),
      },
      {
        q: 'How does the app handle "Solo Leveling Season 2" style anime imports?',
        a: (
          <>
            When an imported anime is a later season of a series already in your library,
            PlexTracker uses AniList's relations to attach it to the main series entry at the
            correct season number — automatically, with no separate entry and no review item.
            It only does this when external IDs confirm it's the same series. Genuine sequels
            (e.g. Dragon Ball → Dragon Ball Z) are left as separate titles.
          </>
        ),
      },
      {
        q: 'The Match Review card shows chips like "Simkl", "IMDb", "TMDB" — what are those?',
        a: (
          <>
            Clickable links to the title's page on each service. Tap one to open it in your
            browser so you can verify the match before confirming. Only chips for IDs the app
            actually found are shown.
          </>
        ),
      },
    ],
  },
  {
    label: 'AniList sync',
    items: [
      {
        q: 'When does AniList get updated?',
        a: (
          <>
            Automatically — every time you mark an episode watched/unwatched, change a title’s
            status, or change its rating. The push runs in the background; nothing to press.
          </>
        ),
      },
      {
        q: 'Why didn’t my rating reach AniList for a particular season?',
        a: (
          <>
            Ratings are only sent for seasons whose status is <strong>Completed</strong> or{' '}
            <strong>Dropped</strong>. AniList rejects scores on series still in progress, so
            PlexTracker filters them out. Finish (or drop) the season — the next push carries
            the rating.
          </>
        ),
      },
      {
        q: '"AniList connection expired" — how do I fix it?',
        a: (
          <>
            Open <strong>Admin → AniList → Reconnect</strong>. Re-authorize on AniList’s page;
            PlexTracker stores the new token and resumes pushing. Until you reconnect every push
            is silently dropped.
          </>
        ),
      },
    ],
  },
  {
    label: 'Library navigation',
    items: [
      {
        q: 'What do the statuses mean?',
        a: (
          <ul className={s.bulletList}>
            <li>
              <strong>Watching</strong> — at least one <em>aired</em> episode is still unwatched.
              Episodes that haven’t aired yet don’t count.
            </li>
            <li>
              <strong>Caught up</strong> — every aired episode watched, but the series is
              ongoing (more are coming). These cards show a green <strong>CAUGHT UP</strong>{' '}
              badge instead of WATCHING, and flip back to WATCHING the moment a new episode airs.
              It’s a sub-filter of Watching, not a separate status.
            </li>
            <li>
              <strong>Completed</strong> — series ended (or cancelled) <em>and</em> fully watched.
            </li>
            <li>
              <strong>Plan</strong> — added but not started.
            </li>
            <li>
              <strong>Dropped</strong> — abandoned.
            </li>
          </ul>
        ),
      },
      {
        q: 'Does PlexTracker ever change a series to Completed by itself?',
        a: (
          <>
            Yes — but only when a series <em>both</em> ends (or gets cancelled) <em>and</em> has
            every episode watched. It happens instantly when you mark the last episode watched,
            and the daily refresh also catches it when TMDB reports the series ended or cancelled.
            <br />
            A cancelled series you haven’t finished stays <strong>Watching</strong> — cancellation
            alone never marks it Completed.
          </>
        ),
      },
      {
        q: 'How do I select multiple titles at once?',
        a: (
          <>
            Long-press (~½ second) on any poster or card. The phone vibrates briefly and the app
            enters selection mode. Tap other titles to add them; the action bar offers bulk
            status change or delete. Tap an empty area or hit back to exit.
          </>
        ),
      },
      {
        q: 'What’s the round badge with a number on a "Watching" card?',
        a: (
          <>
            It’s the next unwatched episode number. Tap it once to mark that episode as
            watched and advance — useful for one-tap progress without opening the title.
          </>
        ),
      },
      {
        q: 'I have two duplicate titles — how do I merge them?',
        a: (
          <>
            Open the title you want to <strong>discard</strong> (the source) → pull up the
            Actions bar → <strong>More</strong> → <strong>Merge</strong>. Search for the title
            to keep, confirm. All episodes, watch events and AniList mapping move over. For
            anime, Gemini detects which season the source maps to, so episodes land on the
            right season number in the destination.
          </>
        ),
      },
    ],
  },
  {
    label: 'Adding & maintenance',
    items: [
      {
        q: 'How do I add a title manually?',
        a: (
          <>
            Bottom nav → <strong>Add</strong>. Three options:
            <ul className={s.bulletList}>
              <li>Paste an IMDb / TMDB / TheTVDB / AniList URL</li>
              <li>Search by name (TMDB + AniList)</li>
              <li>Use the Android share sheet — PlexTracker registers as a share target</li>
            </ul>
            <span className={s.afterList}>
              After confirmation, choose Watching, Plan to watch, or Completed (marks every
              episode as watched and prompts you to rate the title).
            </span>
          </>
        ),
      },
      {
        q: 'What does "Refresh all metadata" do?',
        a: (
          <>
            Force-runs the daily background refresh now: re-checks every non-completed title
            against TMDB / AniList for new episodes, status changes, missing covers, and
            updates AniList community scores. Takes a few minutes. Normally not needed — the
            same job runs automatically once a day.
          </>
        ),
      },
      {
        q: 'When does the rating prompt appear?',
        a: (
          <>
            Automatically when:
            <ul className={s.bulletList}>
              <li>A movie is marked watched</li>
              <li>The last episode of a season is marked watched</li>
              <li>A title is changed to <strong>Completed</strong> or <strong>Dropped</strong></li>
            </ul>
            <span className={s.afterList}>
              To rate manually, open the title, pull up the Actions bar and tap{' '}
              <strong>★ Rate</strong>.
            </span>
          </>
        ),
      },
      {
        q: 'What is "Season audit" in Admin?',
        a: (
          <>
            <strong>Admin → Season audit</strong> scans your library for anime seasons that were
            imported as standalone entries and ended up confirmed before being attached to their
            main series. For each one, it proposes a merge. Tap <strong>Accept</strong> to merge
            it into the right series, or <strong>Dismiss</strong> to leave it alone (dismissed
            proposals don't come back). <strong>Accept all</strong> processes every proposal at
            once. Nothing merges without your click.
          </>
        ),
      },
    ],
  },
]

interface AccordionProps {
  q: string
  a: ComponentChildren
}

function FaqAccordion({ q, a }: AccordionProps) {
  const [open, setOpen] = useState(false)
  return (
    <div className={s.item}>
      <button
        type="button"
        className={s.header}
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <span className={s.question}>{q}</span>
        <span className={clsx(s.chevron, open && s.chevronOpen)} aria-hidden="true">
          ›
        </span>
      </button>
      {open && <div className={s.answer}>{a}</div>}
    </div>
  )
}

export function Help({ path }: { path?: string }) {
  return (
    <div className={s.page}>
      <h1 className={s.title}>Help</h1>
      <p className={s.subtitle}>
        Quick answers to things you might forget. Tap any question to expand.
      </p>
      {sections.map((section) => (
        <section key={section.label} className={s.section}>
          <h2 className={s.sectionLabel}>{section.label}</h2>
          <div className={s.list}>
            {section.items.map((item) => (
              <FaqAccordion key={item.q} q={item.q} a={item.a} />
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
