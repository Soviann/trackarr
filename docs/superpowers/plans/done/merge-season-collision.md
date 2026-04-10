# Merge — gestion des saisons déjà existantes sur la cible

## Context

Lors d'un merge de titres, si la cible possède déjà une saison avec le même numéro que la saison source (après application de `season_offset`), le `UPDATE seasons` échoue avec une violation de contrainte `UNIQUE(title_id, season_number)`. La transaction est rollbackée et le merge échoue entièrement.

Exemple rapide : source title 7570 → target title 5994 avec `season_offset: 2`. Si source a une saison 1 → elle devient saison 3 sur la cible. Si la cible a déjà une saison 3 : crash.

Le code actuel contient même un commentaire qui admet le problème (`"If it crashes on unique constraint, it means we have overlapping seasons."`).

## Comportement attendu

Quand une saison source entre en collision avec une saison déjà existante sur la cible :
- Les épisodes de la saison source sont déplacés dans la saison cible existante (ceux dont le numéro d'épisode n'existe pas encore)
- Les épisodes en double (même numéro) sont supprimés — leurs `watch_events` conservent le `title_id` cible mais perdent le lien `episode_id` (acceptable)
- La saison source (maintenant vide) est supprimée
- Le merge continue normalement pour les autres saisons

## Acceptance Criteria

- Un merge avec `season_offset` qui crée une collision de saison réussit sans erreur
- Les épisodes non-conflictuels sont bien rattachés à la saison cible existante
- Aucun épisode n'est perdu côté cible
- Le merge sans collision fonctionne exactement comme avant

## Fichier à modifier

`/Users/nicolasvasse/Siqual/plextracker/internal/repository/title.go` — fonction `mergeInTx`, bloc de la boucle sur `moves` (lignes ~793–801)

## Changement

Dans la boucle `for _, m := range moves`, remplacer le UPDATE direct par :

1. Chercher si une saison avec `season_number = m.newNum` existe déjà sur `destID` :
   ```sql
   SELECT id FROM seasons WHERE title_id = ? AND season_number = ?
   ```

2. **Pas de collision** → comportement actuel :
   ```sql
   UPDATE seasons SET title_id = ?, season_number = ? WHERE id = ?
   ```

3. **Collision** (on a trouvé un `targetSeasonID`) :
   a. Déplacer les épisodes non-conflictuels :
      ```sql
      UPDATE OR IGNORE episodes SET season_id = ? WHERE season_id = ?
      -- (targetSeasonID, m.id)
      ```
   b. Supprimer les épisodes restants (ceux qui n'ont pas pu être déplacés, numéros en double) :
      ```sql
      DELETE FROM episodes WHERE season_id = ?
      -- (m.id) — les watch_events référençant ces épisodes passeront à episode_id = NULL via ON DELETE SET NULL
      ```
   c. Supprimer la saison source devenue vide :
      ```sql
      DELETE FROM seasons WHERE id = ?
      -- (m.id)
      ```

## Aucune migration SQL requise

Le schéma n'est pas modifié. Uniquement logique Go + SQL dans `mergeInTx`.

## Vérification

1. `make test` — les tests existants passent
2. Rejouer le merge qui échouait : POST `/api/titles/7570/merge` avec `{"target_id": 5994, "season_offset": 2}` → doit retourner `{"status": "ok"}`
3. Vérifier visuellement que le titre cible (5994) possède les bonnes saisons et épisodes dans l'app
