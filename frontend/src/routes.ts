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
  search: '/search',
  add: '/add',
  stats: '/stats',
  login: '/login',
  matchReview: '/match-review',
  title: '/title/:id',
  person: '/person/:name',
  admin: '/admin',
  adminValidate: '/admin/validate',
  adminTasks: '/admin/tasks',
  adminSeasonAudit: '/admin/season-audit',
  adminNotifications: '/admin/notifications',
  adminAniList: '/admin/anilist',
  adminArr: '/admin/arr',
  adminArrQueue: '/admin/arr/queue',
  adminHelp: '/admin/help',
  anilistCallback: '/anilist/callback',
} as const

export const routeTo = {
  home: () => '/',
  comingUp: () => '/coming-up',
  continueWatching: () => '/continue-watching',
  search: () => '/search',
  add: () => '/add',
  stats: () => '/stats',
  login: () => '/login',
  matchReview: () => '/match-review',
  title: (id: number | string) => `/title/${id}`,
  person: (name: string) => `/person/${encodeURIComponent(name)}`,
  admin: () => '/admin',
  adminValidate: () => '/admin/validate',
  adminTasks: () => '/admin/tasks',
  adminSeasonAudit: () => '/admin/season-audit',
  adminNotifications: () => '/admin/notifications',
  adminAniList: () => '/admin/anilist',
  adminArr: () => '/admin/arr',
  adminArrQueue: () => '/admin/arr/queue',
  adminHelp: () => '/admin/help',
  anilistCallback: () => '/anilist/callback',
} as const
