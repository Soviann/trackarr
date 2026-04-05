# Tri par date de sortie

## Résumé

Ajouter un tri par date de sortie sur la page d'accueil. La date de sortie provient de TMDB (`release_date` pour les films, `first_air_date` pour les séries). Ce tri remplace l'ancien tri "Year" dans l'interface et devient le **tri par défaut** (plus récent en premier).

## Comportement utilisateur

### Page d'accueil

- Au premier chargement (ou si aucune préférence sauvegardée), les titres sont triés par **date de sortie décroissante** (les plus récents en haut).
- Les titres sans date de sortie apparaissent en fin de liste (NULLS LAST).

### Tiroir de filtres

Les options de tri affichées sont, dans l'ordre :

1. **Last updated** (desc par défaut)
2. **Title** (asc par défaut)
3. **Release date** (desc par défaut) — remplace l'ancien chip "Year"
4. **Rating** (desc par défaut)
5. **Date added** (desc par défaut)

Un clic sur le chip actif inverse l'ordre (asc ↔ desc). Un clic sur un autre chip applique son ordre par défaut.

### Rétro-remplissage des titres existants

Après déploiement, l'utilisateur clique sur **"Rafraîchir tout"** dans la page Admin. Les tâches d'enrichissement remplissent progressivement la date de sortie de chaque titre. Aucune nouvelle action admin n'est nécessaire.

## Données

- Nouvelle colonne `release_date TEXT` sur la table `titles` (format `YYYY-MM-DD`).
- La colonne `year` (entier) reste en base et dans le code, mais n'est plus exposée comme option de tri dans l'interface.

## Pipeline d'enrichissement

- Les métadonnées TMDB déjà récupérées contiennent `release_date` (films) et `first_air_date` (séries). Ces valeurs sont aujourd'hui ignorées.
- L'enrichissement les persiste désormais dans la colonne `release_date` du titre.
- Le webhook Plex déclenche aussi l'enrichissement pour les nouveaux titres — même chemin.

## Critères d'acceptation

- [ ] Le tri par défaut à l'accueil est "Release date" décroissant.
- [ ] Le chip "Year" est remplacé par "Release date" dans le tiroir de filtres.
- [ ] Les titres sans date de sortie apparaissent en fin de liste quel que soit l'ordre.
- [ ] Après un "Rafraîchir tout" en admin, tous les titres avec un `tmdb_id` ont une `release_date` renseignée.
- [ ] Les nouveaux titres créés via Plex reçoivent automatiquement leur `release_date` après enrichissement.
