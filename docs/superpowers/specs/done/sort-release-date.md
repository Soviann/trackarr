# Tri et filtre par date de sortie

## Résumé

Ajouter un tri et un filtre par date de sortie sur la page d'accueil. La date de sortie provient de TMDB (`release_date` pour les films, `first_air_date` pour les séries). Le tri remplace l'ancien tri "Year" dans l'interface et devient le **tri par défaut** (plus récent en premier). Le filtre offre un accès rapide par décennie et un intervalle de dates précis.

## Comportement utilisateur

### Tri — Page d'accueil

- Au premier chargement (ou si aucune préférence sauvegardée), les titres sont triés par **date de sortie décroissante** (les plus récents en haut).
- Les titres sans date de sortie apparaissent en fin de liste (NULLS LAST).

### Tri — Tiroir de filtres

Les options de tri affichées sont, dans l'ordre :

1. **Last updated** (desc par défaut)
2. **Title** (asc par défaut)
3. **Release date** (desc par défaut) — remplace l'ancien chip "Year"
4. **Rating** (desc par défaut)
5. **Date added** (desc par défaut)

Un clic sur le chip actif inverse l'ordre (asc ↔ desc). Un clic sur un autre chip applique son ordre par défaut.

### Filtre — Section "Release date" dans le tiroir

Nouvelle section dans le tiroir de filtres, après "Series status" :

**Ligne 1 — Dropdown décennie** : menu déroulant avec les options "All", "2000s", "2010s", "2020s".
- Sélectionner une décennie filtre par la colonne `year` (entier) : `year BETWEEN 2020 AND 2029`.
- Revenir sur "All" désactive le filtre.
- Sélectionner une décennie vide les champs date (from/to).

**Ligne 2 — Intervalle de dates** : deux champs date (type `date`), libellés "From" et "To".
- Chaque champ peut rester vide (intervalle ouvert d'un côté ou des deux).
- Filtre sur la colonne `release_date` (texte `YYYY-MM-DD`).
- Remplir un champ date désélectionne tout chip décennie actif.

**Ligne 3 — Toggle** : "Include without release date" (activé par défaut).
- S'applique uniquement quand un filtre date (intervalle ou décennie) est actif.
- Quand désactivé : exclut les titres dont `release_date IS NULL`.
- Quand activé (défaut) : inclut tous les titres, ceux sans date apparaissent en fin si triés par release date.

**Comportement combiné** : décennie et intervalle sont mutuellement exclusifs. Le dropdown décennie filtre sur `year` (entier, déjà renseigné pour quasi tous les titres). L'intervalle de dates filtre sur `release_date` (texte, renseigné après enrichissement TMDB).

**Tag en mode réduit** : quand le tiroir est fermé, le filtre actif s'affiche comme tag — ex. "2020s", "2024-01 → 2025-03", "≥ 2020-01-01".

### Rétro-remplissage des titres existants

Après déploiement, l'utilisateur clique sur **"Rafraîchir tout"** dans la page Admin. Les tâches d'enrichissement remplissent progressivement la date de sortie de chaque titre. Le filtre par décennie fonctionne immédiatement (utilise `year`), le filtre par dates précises nécessite le rétro-remplissage.

## Données

- Nouvelle colonne `release_date TEXT` sur la table `titles` (format `YYYY-MM-DD`).
- La colonne `year` (entier) reste en base et dans le code, utilisée par le filtre décennie.
- Nouveaux query params : `release_from`, `release_to`, `decade`, `include_no_release`.

## Pipeline d'enrichissement

- Les métadonnées TMDB déjà récupérées contiennent `release_date` (films) et `first_air_date` (séries). Ces valeurs sont aujourd'hui ignorées.
- L'enrichissement les persiste désormais dans la colonne `release_date` du titre.
- Le webhook Plex déclenche aussi l'enrichissement pour les nouveaux titres — même chemin.

## Critères d'acceptation

- [ ] Le tri par défaut à l'accueil est "Release date" décroissant.
- [ ] Le chip "Year" est remplacé par "Release date" dans le tiroir de filtres.
- [ ] Les titres sans date de sortie apparaissent en fin de liste quel que soit l'ordre.
- [ ] La section "Release date" avec chips décennie, champs date et toggle est visible dans le tiroir.
- [ ] Le dropdown décennie filtre correctement sur la colonne `year`.
- [ ] L'intervalle de dates filtre correctement sur la colonne `release_date`.
- [ ] Décennie et intervalle sont mutuellement exclusifs.
- [ ] Le toggle "Include without release date" exclut/inclut les titres sans date.
- [ ] Le filtre actif apparaît comme tag quand le tiroir est fermé.
- [ ] Après un "Rafraîchir tout" en admin, tous les titres avec un `tmdb_id` ont une `release_date` renseignée.
- [ ] Les nouveaux titres créés via Plex reçoivent automatiquement leur `release_date` après enrichissement.
