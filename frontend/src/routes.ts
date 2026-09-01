/**
 * Single source of truth for SPA routes.
 *
 * - `ROUTE_PATHS`: literal patterns with `:param` placeholders, used by
 *   `<Route path="...">` declarations in `app.tsx`.
 * - `routeTo`: builders that produce concrete URLs for navigation
 *   (`route(routeTo.title(id))`, `<a href={routeTo.title(id)}>`).
 *
 * Why these exist: typo bugs (e.g. `/titles/:id` instead of `/title/:id`)
 * have already shipped to prod. Centralizing kills the bug class.
 */

export const ROUTE_PATHS = {
  home: '/',
  comingUp: '/coming-up',
  continueWatching: '/continue-watching',
  releases: '/releases',
  search: '/search',
  add: '/add',
  stats: '/stats',
  wrapped: '/wrapped',
  wrappedYear: '/wrapped/:year',
  login: '/login',
  setup: '/setup',
  matchReview: '/match-review',
  title: '/title/:id',
  person: '/person/:name',
  admin: '/admin',
  adminSettings: '/admin/settings',
  adminAuth: '/admin/auth',
  adminValidate: '/admin/validate',
  adminTasks: '/admin/tasks',
  adminSeasonAudit: '/admin/season-audit',
  adminNotifications: '/admin/notifications',
  adminJellyfin: '/admin/jellyfin',
  adminAniList: '/admin/anilist',
  adminArr: '/admin/arr',
  adminHelp: '/admin/help',
  anilistCallback: '/anilist/callback',
} as const

export const routeTo = {
  home: () => '/',
  comingUp: () => '/coming-up',
  continueWatching: () => '/continue-watching',
  releases: () => '/releases',
  search: () => '/search',
  add: () => '/add',
  stats: () => '/stats',
  wrapped: (year?: number) => year ? `/wrapped/${year}` : '/wrapped',
  login: () => '/login',
  setup: () => '/setup',
  matchReview: () => '/match-review',
  title: (id: number | string) => `/title/${id}`,
  person: (name: string) => `/person/${encodeURIComponent(name)}`,
  admin: () => '/admin',
  adminSettings: () => '/admin/settings',
  adminAuth: () => '/admin/auth',
  adminValidate: () => '/admin/validate',
  adminTasks: () => '/admin/tasks',
  adminSeasonAudit: () => '/admin/season-audit',
  adminNotifications: () => '/admin/notifications',
  adminJellyfin: () => '/admin/jellyfin',
  adminAniList: () => '/admin/anilist',
  adminArr: () => '/admin/arr',
  adminHelp: () => '/admin/help',
  anilistCallback: () => '/anilist/callback',
} as const
