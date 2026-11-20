import { useState, useEffect } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import { useTranslation, type TranslationKey } from '../i18n'
import s from './Help.module.css'

interface FaqItem {
  q: string
  a: ComponentChildren
}

interface FaqSection {
  label: string
  items: FaqItem[]
}

type TranslateFn = (key: TranslationKey, params?: Record<string, string | number>) => string

function buildSections(t: TranslateFn): FaqSection[] {
  return [
    {
      label: t('help.secGettingStarted'),
      items: [
        {
          q: t('help.qWhatIsTrackarr'),
          a: <>{t('help.aWhatIsTrackarr')}</>,
        },
        {
          q: t('help.qInstallPwa'),
          a: (
            <>
              {t('help.aInstallPwaIntro')}
              <ul className={s.bulletList}>
                <li>{t('help.aInstallPwaAndroid')}</li>
                <li>{t('help.aInstallPwaIos')}</li>
                <li>{t('help.aInstallPwaDesktop')}</li>
              </ul>
            </>
          ),
        },
        {
          q: t('help.qAndroidShortcuts'),
          a: (
            <>
              {t('help.aAndroidShortcutsIntro')}
              <ul className={s.bulletList}>
                <li>{t('help.aAndroidShortcutsAdd')}</li>
                <li>{t('help.aAndroidShortcutsLib')}</li>
                <li>{t('help.aAndroidShortcutsSearch')}</li>
              </ul>
            </>
          ),
        },
        {
          q: t('help.qImportSimkl'),
          a: (
            <>
              {t('help.aImportSimklWeb')}
              <br />
              {t('help.aImportSimklCli')}
              <br />
              <code>docker exec -t trackarr trackarr import /path/to/backup.zip</code>
              <br />
              <code>docker exec -t trackarr trackarr import --dry-run /path/to/backup.zip</code>
            </>
          ),
        },
      ],
    },
    {
      label: t('help.secApiKeys'),
      items: [
        {
          q: t('help.qTmdbKey'),
          a: (
            <>
              {t('help.aTmdbKey1')}{' '}
              <a href="https://www.themoviedb.org/signup" target="_blank" rel="noopener noreferrer">
                themoviedb.org
              </a>
              .
              <br />
              {t('help.aTmdbKey2')}
              <br />
              {t('help.aTmdbKey3')}
              <br />
              {t('help.aTmdbKey4')}
            </>
          ),
        },
        {
          q: t('help.qGeminiKey'),
          a: (
            <>
              {t('help.aGeminiKey1')}{' '}
              <a href="https://aistudio.google.com/app/apikey" target="_blank" rel="noopener noreferrer">
                {t('help.aGeminiKey1Link')}
              </a>
              .
              <br />
              {t('help.aGeminiKey2')}
              <br />
              {t('help.aGeminiKey3')}
            </>
          ),
        },
        {
          q: t('help.qAnilistOauth'),
          a: (
            <>
              {t('help.aAnilistOauth1')}
              <br />
              {t('help.aAnilistOauth2')}
              <br />
              {t('help.aAnilistOauth3')}
              <br />
              <code>https://your-domain.com/anilist/callback</code> ({t('help.aAnilistOauthOr')}{' '}
              <code>http://localhost:8080/anilist/callback</code>).
              <br />
              {t('help.aAnilistOauth4')}
              <br />
              {t('help.aAnilistOauth5')}
            </>
          ),
        },
        {
          q: t('help.qConnectArr'),
          a: (
            <>
              {t('help.aConnectArr1')}
              <br />
              {t('help.aConnectArr2')}
              <br />
              {t('help.aConnectArr3')}
            </>
          ),
        },
      ],
    },
    {
      label: t('help.secNavGestures'),
      items: [
        {
          q: t('help.qStatuses'),
          a: (
            <ul className={s.bulletList}>
              <li>{t('help.aStatusesWatching')}</li>
              <li>{t('help.aStatusesCaughtUp')}</li>
              <li>{t('help.aStatusesCompleted')}</li>
              <li>{t('help.aStatusesPlan')}</li>
              <li>{t('help.aStatusesDropped')}</li>
            </ul>
          ),
        },
        {
          q: t('help.qQuickPill'),
          a: <>{t('help.aQuickPill')}</>,
        },
        {
          q: t('help.qMultiSelect'),
          a: <>{t('help.aMultiSelect')}</>,
        },
        {
          q: t('help.qPosterBorders'),
          a: (
            <ul className={s.bulletList}>
              <li>{t('help.aPosterBordersRadarr')}</li>
              <li>{t('help.aPosterBordersSonarr')}</li>
              <li>{t('help.aPosterBordersBadge')}</li>
            </ul>
          ),
        },
        {
          q: t('help.qMergeTitles'),
          a: <>{t('help.aMergeTitles')}</>,
        },
      ],
    },
    {
      label: t('help.secMatchingAudit'),
      items: [
        {
          q: t('help.qMatchDiff'),
          a: (
            <>
              {t('help.aMatchDiffIntro')}
              <br />
              {t('help.aMatchDiffUnconfirmed')}
              <br />
              {t('help.aMatchDiffPending')}
              <br />
              {t('help.aMatchDiffFooter')}
            </>
          ),
        },
        {
          q: t('help.qMatchSwipe'),
          a: (
            <>
              {t('help.aMatchSwipeIntro')}
              <ul className={s.bulletList}>
                <li>{t('help.aMatchSwipeConfirm')}</li>
                <li>{t('help.aMatchSwipeFix')}</li>
              </ul>
              {t('help.aMatchSwipeFooter')}
            </>
          ),
        },
        {
          q: t('help.qSeasonAudit'),
          a: <>{t('help.aSeasonAudit')}</>,
        },
      ],
    },
    {
      label: t('help.secAnilistSync'),
      items: [
        {
          q: t('help.qAnilistWhen'),
          a: <>{t('help.aAnilistWhen')}</>,
        },
        {
          q: t('help.qAnilistRating'),
          a: <>{t('help.aAnilistRating')}</>,
        },
        {
          q: t('help.qAnilistMultiPart'),
          a: <>{t('help.aAnilistMultiPart')}</>,
        },
      ],
    },
    {
      label: t('help.secTroubleshooting'),
      items: [
        {
          q: t('help.qResetPassword'),
          a: (
            <>
              {t('help.aResetPasswordIntro')}
              <br />
              {t('help.aResetPassword1')}
              <br />
              {t('help.aResetPassword2')}
              <br />
              <code>docker exec -t trackarr trackarr reset-password --password="MyNewPassword"</code>
            </>
          ),
        },
        {
          q: t('help.qCachedScreens'),
          a: <>{t('help.aCachedScreens')}</>,
        },
        {
          q: t('help.qBgRefresh'),
          a: <>{t('help.aBgRefresh')}</>,
        },
      ],
    },
  ]
}

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
  void path
  const { t } = useTranslation()
  const sections = buildSections(t)

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [])

  return (
    <div className={s.page}>
      <h1 className={s.title}>{t('help.title')}</h1>
      <p className={s.subtitle}>{t('help.subtitle')}</p>
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
