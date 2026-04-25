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
            then tap <strong>Fix match</strong>. Paste a TMDB / IMDb / TheTVDB / AniList URL,
            or search by name. Saving rematches the title immediately.
          </>
        ),
      },
      {
        q: 'What does "Pending review" / "Unconfirmed" mean? Should I do something?',
        a: (
          <>
            PlexTracker tried to identify the title automatically but isn't fully sure.
            <br />
            <strong>Pending review</strong> — high confidence, just confirm.
            <br />
            <strong>Unconfirmed</strong> — weak match, probably needs a manual fix.
            <br />
            Both show up in the <strong>Match Review</strong> screen (a red banner appears on the
            Library home if any are waiting).
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
              <strong>Watching</strong> — at least one unwatched episode exists right now.
            </li>
            <li>
              <strong>Up to date</strong> — caught up on all aired episodes, but more are coming.
              It’s a sub-filter of Watching, not a separate status.
            </li>
            <li>
              <strong>Completed</strong> — series ended <em>and</em> fully watched.
            </li>
            <li>
              <strong>Plan to watch</strong> — added but not started.
            </li>
            <li>
              <strong>Dropped</strong> — abandoned.
            </li>
          </ul>
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
            Actions bar → <strong>Merge</strong>. Search for the title to keep, confirm. All
            episodes, watch events and AniList mapping move over. For anime, Gemini detects
            which season the source maps to, so episodes land on the right season number in the
            destination.
          </>
        ),
      },
    ],
  },
  {
    label: 'Adding & maintenance',
    items: [
      {
        q: 'How do I add a title that isn’t on Plex yet?',
        a: (
          <>
            Bottom nav → <strong>Add</strong>. Three options:
            <ul className={s.bulletList}>
              <li>Paste an IMDb / TMDB / TheTVDB / AniList URL</li>
              <li>Search by name (TMDB + AniList)</li>
              <li>Use the Android share sheet — PlexTracker registers as a share target</li>
            </ul>
            <span className={s.afterList}>
              After confirmation, choose Watching, Already seen, or Plan to watch.
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
