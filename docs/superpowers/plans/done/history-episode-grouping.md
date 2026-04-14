# Regroupement des épisodes dans l'historique

## PO

Au lieu d'afficher une ligne par épisode dans l'historique, les épisodes consécutifs d'une même saison sont regroupés en plages (ex: "S1 E1-10"). Concerne l'overlay History d'un titre et le feed Recent Activity de la page Stats.

## Phase 1 — Utilitaire de regroupement

### T1.1 `groupIntoRanges` `[seq]`

- **Fichier** : `frontend/src/utils/episodeRanges.ts` (nouveau)
- **Types** :
  ```ts
  interface RangableEpisode {
    season_number: number | null
    episode_number: number | null
    episode_name: string | null
  }
  interface EpisodeRangeGroup<T> {
    seasonNumber: number | null
    startEp: number | null
    endEp: number | null
    episodeName: string | null  // null quand range > 1
    items: T[]
  }
  ```
- **`groupIntoRanges<T extends RangableEpisode>(items: T[]): EpisodeRangeGroup<T>[]`** : parcours O(n), items triés par `episode_number ASC` en entrée. Accumule tant que `season_number` identique et `episode_number === prev + 1`. Items avec `episode_number == null` → groupe isolé. `episodeName` conservé uniquement si `items.length === 1`.
- **`formatRangeLabel(group: EpisodeRangeGroup<any>): string`** :
  - `season == null && ep == null` → `"Movie"`
  - `startEp === endEp` → `"S{season} E{ep}"`
  - `startEp !== endEp` → `"S{season} E{start}-{end}"`

### T1.2 Tests `[seq]`

- **Fichier** : `frontend/src/utils/episodeRanges.test.ts` (nouveau)
- **Assertions** :
  - `[]` → `[]`
  - `[E1,E2,E3]` → 1 groupe `startEp=1, endEp=3, episodeName=null`
  - `[E1,E2,E5,E6]` → 2 groupes `[1-2], [5-6]`
  - `[S1E1, S2E1]` → 2 groupes (pas de merge cross-saison)
  - `[episode_number=null]` → groupe isolé, label `"Movie"`
  - Single episode → `episodeName` conservé
  - `formatRangeLabel` : 3 cas ci-dessus
- **Critère** : `make test-front` passe

## Phase 2 — TitleHistory

### T2.1 Groupement et rendu `[seq]`

- **Fichier** : `frontend/src/components/TitleHistory.tsx`
- **Logique** (dans le composant, après réception de `data`) :
  1. Grouper `EpisodeHistory[]` par `season_number` → `Map<number|null, EpisodeHistory[]>`
  2. Dans chaque saison : trier par `episode_number ASC`, passer à `groupIntoRanges`
  3. Trier les saisons par `Math.max(...last_watched_at)` DESC
  4. Rendu : pour chaque saison, un séparateur `Season {n}` puis une ligne par range
- **Ligne range** :
  - Label : `formatRangeLabel(group)` + `" — {episodeName}"` seulement si single
  - Date : `max(last_watched_at)` des items du groupe
  - Badge rewatch : uniquement si single et `watch_count > 1`
- **Fichier CSS** : `frontend/src/components/TitleHistory.module.css` — ajouter `.seasonDivider { padding: 8px 16px; font-weight: 600; color: var(--text-secondary); font-size: 0.85rem; }`
- **Critère** : overlay History d'une série avec S1 E1-10 vus → affiche "S1 E1-10" en une ligne. Épisode seul → garde le nom. Film → "Movie".

## Phase 3 — ActivitySection

### T3.1 Groupement dans le feed `[seq]`

- **Fichier** : `frontend/src/pages/Stats.tsx`
- **Logique** (entre `groupByDate` et le rendu) :
  1. Dans chaque bucket de date : sous-grouper par `(title_id, season_number)`
  2. Trier chaque sous-groupe par `episode_number ASC`, passer à `groupIntoRanges`
  3. Pour chaque range : conserver `title_id, title_name, cover_url, title_type` du premier event, `is_completion = group.items.some(e => e.is_completion)`, `watched_at = group.items[0].watched_at`
  4. Re-trier les rows par `watched_at DESC`
- **Type intermédiaire** (local au fichier) :
  ```ts
  interface ActivityRangeRow {
    titleId: number; titleName: string; coverUrl: string | null; titleType: string
    label: string; isCompletion: boolean; watchedAt: string; titleLink: string
  }
  ```
- **Rendu** : remplacer `evts.map(...)` par `activityRows.map(...)`. Sub-label = `formatRangeLabel`. Pas de nom d'épisode pour les ranges. Badge identique (Completed / Movie / Episode).
- **Critère** : 5 épisodes S1E1-5 d'une même série le même jour → une seule ligne "S1 E1-5". Films → inchangés. Séries différentes → pas de merge.

## Fichiers modifiés

| Fichier | Action |
|---------|--------|
| `frontend/src/utils/episodeRanges.ts` | Nouveau — utilitaire |
| `frontend/src/utils/episodeRanges.test.ts` | Nouveau — tests |
| `frontend/src/components/TitleHistory.tsx` | Modifier — groupement + rendu |
| `frontend/src/components/TitleHistory.module.css` | Modifier — `.seasonDivider` |
| `frontend/src/pages/Stats.tsx` | Modifier — groupement dans ActivitySection |

## Vérification

1. `make test-front` — tests unitaires passent
2. Chrome DevTools MCP :
   - Titre série avec historique → ranges dans l'overlay History
   - Page Stats → ranges dans Recent Activity
   - Titre film → "Movie", pas de régression
   - Épisodes non consécutifs → ranges séparés
