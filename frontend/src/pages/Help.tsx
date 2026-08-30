import { useState, useEffect } from 'preact/hooks'
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
    label: 'Getting Started & PWA',
    items: [
      {
        q: 'What is Trackarr?',
        a: (
          <>
            Trackarr is a fast, self-hosted personal media tracker for movies, TV shows, and anime.
            It automatically logs what you watch via Jellyfin and Plex webhooks, synchronizes anime progress
            and ratings with AniList, and interfaces with your Radarr and Sonarr instances.
          </>
        ),
      },
      {
        q: 'How do I install Trackarr as an app (PWA)?',
        a: (
          <>
            Trackarr is an offline-capable Progressive Web App (PWA):
            <ul className={s.bulletList}>
              <li>
                <strong>Android (Chrome)</strong>: Tap the « Install Trackarr » prompt at the bottom of the screen,
                or open Chrome menu (⋮) → <strong>Install app</strong> / <strong>Add to Home screen</strong>.
              </li>
              <li>
                <strong>iOS (Safari)</strong>: Tap the Share button at the bottom of Safari → <strong>Add to Home Screen</strong>.
              </li>
              <li>
                <strong>Desktop (Chrome / Edge)</strong>: Click the install icon in the right side of the address bar.
              </li>
            </ul>
          </>
        ),
      },
      {
        q: 'What are the Android App Shortcuts?',
        a: (
          <>
            Long-pressing the Trackarr icon on your home screen or app drawer displays 3 instant shortcuts:
            <ul className={s.bulletList}>
              <li><strong>Add title</strong> — Jump straight to the search and URL paste screen (`/add`).</li>
              <li><strong>Library</strong> — Return to your active watching library (`/`).</li>
              <li><strong>Search</strong> — Search your existing media library (`/search`).</li>
            </ul>
          </>
        ),
      },
      {
        q: 'Can I import my existing watch history from Simkl?',
        a: (
          <>
            Yes! If you have a Simkl JSON or ZIP export file, place it on your server and run the CLI command inside the container:
            <br />
            <code>docker exec -t trackarr trackarr import /path/to/backup.zip</code>
            <br />
            You can also test first with <code>import-dry</code> to preview without writing to the database.
          </>
        ),
      },
    ],
  },
  {
    label: 'API Keys & System Setup',
    items: [
      {
        q: 'How do I get a free TMDB API Key?',
        a: (
          <>
            1. Create a free account on <a href="https://www.themoviedb.org/signup" target="_blank" rel="noopener noreferrer">themoviedb.org</a>.
            <br />
            2. Go to <strong>Settings → API → Create / Request API Key (Developer)</strong>.
            <br />
            3. Fill in application details (Application Name: <code>Trackarr</code>, URL: <code>http://localhost:8080</code>).
            <br />
            4. Copy your <strong>API Key (v3 auth)</strong> and paste it into <strong>Admin → System Settings → TMDB API Key</strong>.
          </>
        ),
      },
      {
        q: 'How do I get a Google Gemini AI Key for smart matching?',
        a: (
          <>
            1. Go to <a href="https://aistudio.google.com/app/apikey" target="_blank" rel="noopener noreferrer">Google AI Studio</a>.
            <br />
            2. Sign in with your Google account and click <strong>Create API Key</strong>.
            <br />
            3. Copy the key and paste it into <strong>Admin → System Settings → Gemini API Key</strong>. Free tier keys are sufficient for personal use.
          </>
        ),
      },
      {
        q: 'How do I set up AniList OAuth connection?',
        a: (
          <>
            1. In AniList, open <strong>Settings → Developer → Create New Client</strong>.
            <br />
            2. Set <strong>Name</strong> to <code>Trackarr</code>.
            <br />
            3. Set <strong>Redirect URL</strong> to:
            <br />
            <code>https://your-domain.com/api/auth/anilist/callback</code> (or <code>http://localhost:8080/api/auth/anilist/callback</code>).
            <br />
            4. Save and copy the numeric <strong>Client ID</strong> and <strong>Client Secret</strong> into Trackarr System Settings.
            <br />
            5. Go to <strong>Admin → AniList → Connect Account</strong> to authorize.
          </>
        ),
      },
      {
        q: 'How do I connect Radarr, Sonarr, and Prowlarr?',
        a: (
          <>
            1. In each app, navigate to <strong>Settings → General → Security</strong> and copy the <strong>API Key</strong>.
            <br />
            2. In Trackarr <strong>Admin → Arr Stack</strong>, provide the URL (e.g. <code>http://radarr:7878</code>) and API Key.
            <br />
            3. Select your preferred default Root Folder and Quality Profiles from the dropdowns.
          </>
        ),
      },
    ],
  },
  {
    label: 'Library Navigation & Gestures',
    items: [
      {
        q: 'What do the different statuses mean?',
        a: (
          <ul className={s.bulletList}>
            <li>
              <strong>Watching</strong> — At least one aired episode is still unwatched.
            </li>
            <li>
              <strong>Caught up</strong> — You have watched every currently aired episode, but the series is ongoing and more episodes are scheduled. It displays a green <strong>CAUGHT UP</strong> badge and automatically flips back to Watching when a new episode airs.
            </li>
            <li>
              <strong>Completed</strong> — The movie is finished, or the series has ended / was cancelled and all episodes are watched.
            </li>
            <li>
              <strong>Plan to Watch</strong> — Added to your backlog but not started yet.
            </li>
            <li>
              <strong>Dropped</strong> — Abandoned series or movies.
            </li>
          </ul>
        ),
      },
      {
        q: 'How does the one-tap quick progress pill work?',
        a: (
          <>
            On any watching card, the round badge shows your next unwatched episode number (e.g. <code>E05</code>).
            Tap that circular pill once to immediately mark that episode as watched and advance your progress without opening the title page.
          </>
        ),
      },
      {
        q: 'How do I select multiple titles at once?',
        a: (
          <>
            Long-press (~½ second) on any poster or card. Your device will vibrate briefly and enter multi-selection mode.
            Tap additional cards to select them, then use the bottom action bar for bulk status changes or deletion.
          </>
        ),
      },
      {
        q: 'What do the colored borders and icons on posters mean?',
        a: (
          <ul className={s.bulletList}>
            <li>
              <strong>Yellow top border</strong> on the type badge — Movie is monitored in <strong>Radarr</strong>.
            </li>
            <li>
              <strong>Cyan top border</strong> on the type badge — Series is monitored in <strong>Sonarr</strong>.
            </li>
            <li>
              <strong>Arr icon badge</strong> — An 18px indicator pill showing a yellow target (Radarr) or cyan waveform (Sonarr) appears next to the status badge on list and search cards.
            </li>
          </ul>
        ),
      },
      {
        q: 'How do I merge two duplicate titles?',
        a: (
          <>
            Open the title you want to merge away (the source) → pull up the <strong>Actions</strong> bar → <strong>More → Merge</strong>.
            Search for the target title to keep, pick the destination season number, and confirm. All watch events, episodes, and AniList links migrate over automatically.
          </>
        ),
      },
    ],
  },
  {
    label: 'Media Matching & Season Audit',
    items: [
      {
        q: 'What is the difference between "Pending review" and "Unconfirmed"?',
        a: (
          <>
            Trackarr auto-confirms high-confidence matches via TMDB, AniList, or AI verification.
            <br />
            <strong>Unconfirmed</strong> — Medium or low confidence match, or AI verification failed; verify in Match Review.
            <br />
            <strong>Pending review</strong> — Automatic fallback when the AI verifier (Gemini) is not configured or rate-limited.
            <br />
            Both appear in the <strong>Match Review</strong> dashboard (`/admin/match-review`).
          </>
        ),
      },
      {
        q: 'How do swipe actions work in Match Review?',
        a: (
          <>
            On the Match Review screen, swipe any card left to reveal quick action buttons:
            <ul className={s.bulletList}>
              <li><strong>Confirm (Green)</strong> — Accepts the suggested match.</li>
              <li><strong>Fix Match (Amber)</strong> — Opens the search & URL panel to select a different title.</li>
            </ul>
            Swiping all the way to the far left edge automatically triggers <strong>Confirm</strong>.
          </>
        ),
      },
      {
        q: 'What is "Season Audit" in the Admin menu?',
        a: (
          <>
            <strong>Admin → Season Audit</strong> detects split anime seasons or stray entries that were imported separately (e.g. *Attack on Titan Final Season* or *Frieren Season 2*).
            It presents side-by-side comparison cards with posters, years, and season counts, and lets you attach the stray entry under the correct parent season number with one click.
          </>
        ),
      },
    ],
  },
  {
    label: 'AniList Sync & Multi-Part Seasons',
    items: [
      {
        q: 'When does AniList get updated?',
        a: (
          <>
            Automatically in the background every time you toggle an episode, update a title status, or rate an anime.
          </>
        ),
      },
      {
        q: 'Why was my rating not sent to AniList?',
        a: (
          <>
            AniList's API strictly rejects rating scores on series still marked as <em>Current/Watching</em>.
            Trackarr holds your score and pushes it to AniList the moment the season status changes to <strong>Completed</strong> or <strong>Dropped</strong>.
          </>
        ),
      },
      {
        q: 'How do I link multi-part anime seasons (Part 1 / Part 2)?',
        a: (
          <>
            On any multi-season anime title, look at the blue AniList score strip on the active season.
            Tap the pencil icon <strong>✎</strong> to link one or multiple AniList entries to that single season. Trackarr automatically balances watched episode counts across parts.
          </>
        ),
      },
    ],
  },
  {
    label: 'Troubleshooting & Maintenance',
    items: [
      {
        q: 'How do I reset my password if I get locked out?',
        a: (
          <>
            You have two ways to reset your password:
            <br />
            <strong>1. From the browser:</strong> Click &laquo; Forgot password? &raquo; on the login page and enter your <code>TRCK-XXXX-XXXX-XXXX</code> emergency recovery key saved during setup.
            <br />
            <strong>2. From the terminal (CLI):</strong> Run the command inside your container:
            <br />
            <code>docker exec -t trackarr trackarr reset-password --password="MyNewPassword"</code>
          </>
        ),
      },
      {
        q: 'Why does the app show older cached screens after an update?',
        a: (
          <>
            The PWA Service Worker caches JavaScript and CSS bundles for fast mobile launch.
            Perform a hard refresh in your browser (or append <code>?t=123</code> to the address bar) to let the Service Worker install the new version.
          </>
        ),
      },
      {
        q: 'How do background metadata refreshes work?',
        a: (
          <>
            Trackarr runs an automatic daily background job that refreshes ended/cancelled statuses from TMDB, checks for new upcoming episodes, and updates community ratings.
            You can also trigger an immediate library-wide refresh anytime via <strong>Admin → Refresh All Metadata</strong>.
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
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [])

  return (
    <div className={s.page}>
      <h1 className={s.title}>Help & Documentation</h1>
      <p className={s.subtitle}>
        Comprehensive guides, gestures, API setup, and troubleshooting. Tap any topic to expand.
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

