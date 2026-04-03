# PlexTracker — Guide utilisateur

## Qu'est-ce que PlexTracker ?

PlexTracker est une application personnelle de suivi de visionnage. Elle remplace Simkl comme tracker central pour les films, séries et anime regardés sur Plex.

### Ce que fait PlexTracker

- **Suivi automatique** : chaque film ou épisode terminé sur Plex est automatiquement enregistré via webhook
- **Bibliothèque** : vue d'ensemble de tout ce qui est en cours, terminé, abandonné ou à regarder
- **Progression** : savoir en un coup d'œil où on en est dans chaque série
- **Notes** : noter les titres et saisons (1-10), avec liens vers IMDb et AniList
- **Import Simkl** : import initial de l'historique existant depuis un export Simkl

### Ce que PlexTracker ne fait PAS

- Ce n'est pas une base de données média (pas de synopsis, casting, critiques — IMDb/AniList s'en chargent)
- Pas de version desktop — interface mobile uniquement
- Pas de mode hors-ligne
- Pas de multi-utilisateur

---

## Accès

PlexTracker est une PWA (Progressive Web App) accessible depuis le navigateur Chrome sur Android.

1. Ouvrir `http://<nas-ip>:8080` dans Chrome
2. Se connecter avec le compte Google autorisé
3. Ajouter à l'écran d'accueil pour un accès rapide (Chrome → menu → "Ajouter à l'écran d'accueil")

---

## Écrans

### Bibliothèque (accueil)

L'écran principal affiche tous les titres organisés par statut.

**Onglet "All"** (par défaut) : sections empilées — Terminés, À regarder, Abandonnés (grilles de posters), puis En cours / À jour (cartes horizontales avec barre de progression).

**Onglets filtrés** : Watching, Up to date, Completed, Dropped, Plan to watch — affiche uniquement les titres du statut sélectionné.

**Action rapide** : le badge rond sur chaque carte "En cours" affiche le numéro du prochain épisode. Un tap marque cet épisode comme vu et passe au suivant.

**Bannière de revue** : si des titres ont un match à vérifier, une bannière rouge apparaît sous le titre "Library" avec le nombre de titres concernés.

### Détail d'un titre

- **Couverture** en haut avec dégradé, bouton retour et bouton édition
- **Barre de progression** : "S2 · 7 of 10 episodes watched"
- **Onglets de saison** : chaque saison est un pill. Vert = terminée (avec note), Ambre = en cours (avec progression), Gris = pas commencée
- **Liste d'épisodes** : tap sur un épisode pour le marquer vu/non vu
- **Barre d'actions** (au-dessus de la navbar) :
  - **S02E06** : prochain épisode à regarder, tap pour marquer vu
  - **IMDb** : ouvre la page IMDb
  - **AniList** : synchronise la note (anime uniquement)
  - **Rate** : ouvre le prompt de notation

### Recherche

Recherche globale dans toute la bibliothèque, quel que soit le filtre actif. Le champ de recherche est en bas (zone du pouce). Les résultats s'affichent au-dessus avec le statut actuel de chaque titre.

### Ajouter un titre

Deux méthodes :

1. **Coller un lien** : URL IMDb, TVDB ou AniList → PlexTracker résout automatiquement les IDs et affiche le titre pour confirmation
2. **Rechercher par nom** : recherche dans TMDB/AniList

Après confirmation, choisir le statut : En cours, Déjà vu, À regarder.
- "Déjà vu" marque tous les épisodes comme vus et propose de noter le titre.

**Partage Android** : PlexTracker s'enregistre comme cible de partage. Depuis l'app IMDb ou un navigateur, utiliser "Partager" → "PlexTracker" pour ajouter directement un titre.

### Revue des matchs

Quand PlexTracker reçoit un nouveau titre via Plex, il tente de l'identifier automatiquement (TMDB, AniList, Gemini AI). Si l'identification n'est pas certaine :

- **Pending review** (confiance haute) : probablement correct, à confirmer
- **Unconfirmed** (confiance basse) : nécessite une vérification manuelle

Actions : Confirmer le match ou Corriger (re-recherche ou saisie manuelle d'IDs).

Le bouton "Batch confirm" permet de confirmer tous les matchs "pending" d'un coup.

### Notation

Le prompt de notation apparaît automatiquement quand :
- Un film est marqué comme vu
- Le dernier épisode d'une saison est marqué comme vu
- Le statut est changé en "Abandonné" ou "Terminé"

Options :
- **Save rating** : enregistre la note localement
- **Save & rate on IMDb** : enregistre et ouvre la page IMDb pour noter là-bas aussi
- **Save & sync AniList** : enregistre et synchronise via l'API AniList (anime uniquement)
- **Skip for now** : ferme sans noter

### Édition d'un titre

Depuis le bouton crayon sur le détail d'un titre :
- Changer le **type** (Film, Série, Anime)
- Changer le **statut** (En cours, Terminé, Abandonné, À regarder)
- Changer le **nom affiché** parmi les noms multilingues disponibles

Changer le statut en "Terminé" marque tous les épisodes comme vus. Changer en "Terminé" ou "Abandonné" déclenche le prompt de notation.

---

## Suivi automatique Plex

PlexTracker écoute les webhooks Plex. Quand un film ou épisode atteint ~90% de visionnage, Plex envoie un événement `media.scrobble` et PlexTracker :

1. Identifie le titre (par IDs Plex, TMDB, IMDb, ou via le pipeline de matching)
2. Marque l'épisode/film comme vu
3. Met à jour le statut si nécessaire (tout vu + série terminée → "Completed")
4. Envoie une notification push si c'est une fin de saison ou un film (pour proposer de noter)

Les re-visionnages sont enregistrés dans l'historique mais ne changent pas l'état "vu" de l'épisode.

---

## Notifications push

PlexTracker envoie des notifications push (navigateur Chrome) dans deux cas :

1. **Fin de visionnage** : un film ou le dernier épisode d'une saison est marqué vu → notification pour noter
2. **Changement de statut** : la tâche de fond détecte qu'une série est terminée/annulée ou qu'une nouvelle saison est annoncée

Tap sur la notification → ouvre le prompt de notation ou le détail du titre.

---

## AniList

Pour les anime, PlexTracker peut se connecter à AniList pour synchroniser les notes.

1. Aller dans les paramètres → "Connect AniList"
2. Autoriser l'accès sur la page AniList
3. Le token est stocké (~1 an de validité)

Ensuite, le bouton "Save & sync AniList" dans le prompt de notation ou le bouton AniList dans la barre d'actions envoie la note vers AniList.

---

## Tâches automatiques

PlexTracker exécute des tâches en arrière-plan sans intervention de l'utilisateur.

### Webhook Plex (temps réel)

Dès qu'un film ou épisode atteint ~90% de visionnage sur Plex, un événement `media.scrobble` est envoyé automatiquement à PlexTracker. Le traitement est immédiat :

1. Identification du titre (IDs Plex, puis pipeline de matching si nouveau titre)
2. Marquage de l'épisode/film comme vu
3. Mise à jour du statut si nécessaire (tout vu + série terminée → "Completed")
4. Notification push si c'est une fin de saison ou un film

Aucune configuration côté PlexTracker — il suffit de déclarer l'URL du webhook dans les paramètres Plex : `http://<nas-ip>:8080/api/webhook/plex`.

### Rafraîchissement quotidien des titres (tâche planifiée)

Une fois par jour, PlexTracker parcourt automatiquement la bibliothèque pour maintenir les données à jour.

**Titres concernés** : tous les titres non terminés (en cours, à regarder) + les titres avec des données manquantes (pas de couverture, pas de liste d'épisodes, pas de statut de série).

**Ce que la tâche fait** :

| Action | Détail |
|---|---|
| **Statut de la série** | Vérifie sur TMDB/AniList si le statut a changé (ex: en cours → terminée, nouvelle saison annoncée) |
| **Nouveaux épisodes** | Récupère les épisodes récemment ajoutés sur TMDB/AniList et les crée dans la base |
| **Couvertures** | Télécharge les images de couverture manquantes depuis TMDB |
| **Noms multilingues** | Récupère les titres en français, anglais, romaji (AniList) s'ils manquent |
| **Complétion automatique** | Si une série est terminée/annulée et que tous les épisodes sont vus → passe en "Completed" |
| **Base de cross-référence** | Met à jour le fichier anime-offline-database (mapping entre IDs IMDB/TMDB/AniList/MAL) |

**Notifications** : si le statut d'une série change (terminée, annulée, nouvelle saison), une notification push est envoyée.

**Rate limiting** : les appels aux APIs externes (TMDB, AniList, Gemini) sont séquentiels avec des délais entre chaque requête pour éviter les blocages. Pour Gemini, les clés API sont utilisées en rotation.

**Enrichissement post-import** : après un import Simkl (~7 500 titres), les données manquantes sont comblées progressivement par cette tâche quotidienne sur plusieurs jours. Il n'y a pas de tâche d'enrichissement dédiée — le rafraîchissement quotidien gère tout naturellement.

### Pipeline de matching média (à la demande)

Déclenché automatiquement quand un nouveau titre est détecté (via Plex ou ajout manuel). Le pipeline tente d'identifier le titre et de résoudre tous les IDs externes :

1. **IDs Plex** : utilise les IDs IMDB/TMDB/TVDB déjà présents dans les métadonnées Plex → match `confirmed`
2. **Cross-référence** : consulte la base anime-offline-database pour compléter les IDs manquants → match `confirmed`
3. **Recherche TMDB** : recherche par titre + année si aucun ID trouvé
4. **Recherche AniList** : recherche par titre (anime uniquement)
5. **Vérification Gemini AI** : si les étapes 3-4 ont trouvé un candidat, Gemini vérifie la correspondance
   - Confiance haute → `pending_review` (à confirmer par l'utilisateur)
   - Confiance basse → `unconfirmed` (à vérifier manuellement)

Si aucune étape ne trouve de match, le titre est créé quand même avec les métadonnées Plex disponibles, en statut `unconfirmed`. L'utilisateur peut corriger depuis l'écran de revue des matchs.

---

## Import Simkl

L'import initial depuis Simkl se fait en ligne de commande (pas depuis l'interface web).

```bash
# Dry-run (prévisualisation sans écriture)
make import-dry BACKUP_FILE=/chemin/vers/Simkl_backup.zip

# Import réel
make import BACKUP_FILE=/chemin/vers/Simkl_backup.zip
```

L'import crée les titres avec les IDs externes, les épisodes vus, et les notes. Les données manquantes (couvertures, liste complète d'épisodes, noms multilingues) sont enrichies automatiquement par la tâche de fond quotidienne sur plusieurs jours.

---

## Déploiement

PlexTracker tourne dans un seul conteneur Docker sur le Synology DS920+.

```bash
# Mise à jour
docker compose pull && docker compose up -d
```

Variables d'environnement requises : voir `docker-compose.yml`.
