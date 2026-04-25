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

**Raccourcis** : un appui long sur l'icône PlexTracker dans le launcher Android affiche 3 raccourcis : Ajouter un titre, Bibliothèque, Recherche.

**Badge** : l'icône affiche le nombre de titres en attente de révision (pending review + unconfirmed). Le compteur se met à jour à l'ouverture de l'app et après chaque action dans Match Review.

---

## Écrans

### Bibliothèque (accueil)

L'écran principal affiche tous les titres organisés par statut.

**Onglet "All"** (par défaut) : sections empilées — Terminés, À regarder, Abandonnés (grilles de posters), puis En cours / À jour (cartes horizontales avec barre de progression).

**Onglets filtrés** : Watching, Up to date, Completed, Dropped, Plan to watch — affiche uniquement les titres du statut sélectionné.

**Tri** : le tiroir de filtres propose 5 options de tri (dernière mise à jour, titre, année, note, date d'ajout). Taper un chip l'active ; taper à nouveau inverse la direction (↑/↓). Le tri est masqué pendant la recherche et persiste via localStorage.

**Action rapide** : le badge rond sur chaque carte "En cours" affiche le numéro du prochain épisode. Un tap marque cet épisode comme vu et passe au suivant.

**Sélection multiple** : un appui long (~500 ms) sur une vignette active le mode sélection (vibration courte). Toucher d'autres vignettes les coche/décoche. Actions disponibles : changement de statut ou suppression en lot.

**Pull-to-refresh** : tirer vers le bas pour rafraîchir la page. Un indicateur circulaire suit le doigt ; il passe en teal avec une vibration au franchissement du seuil, puis tourne pendant le chargement. Disponible sur Library, Search, Validate, Match Review et Admin Notifications.

**Bannière de revue** : si des titres ont un match à vérifier, une bannière rouge apparaît sous le titre "Library" avec le nombre de titres concernés.

### Détail d'un titre

- **Couverture** en haut avec dégradé, bouton retour et bouton édition
- **Notes externes** : badges TMDB (bleu-vert), TVDB (bleu) et AniList (%) affichés côte à côte quand disponibles
- **Barre de progression** : "S2 · 7 of 10 episodes watched"
- **Onglets de saison** : chaque saison est un pill. Vert = terminée (avec note), Ambre = en cours (avec progression), Gris = pas commencée
- **Liste d'épisodes** : tap sur un épisode pour le marquer vu/non vu
- **Barre d'actions** (au-dessus de la navbar) :
  - **S02E06** : prochain épisode à regarder, tap pour marquer vu
  - **IMDb** : ouvre la page IMDb
  - **TVDB** : ouvre la page TheTVDB (quand un `tvdb_id` est connu)
  - **AniList** : synchronise la note (anime uniquement)
  - **Rate** : ouvre le prompt de notation

### Recherche

Recherche globale dans toute la bibliothèque, quel que soit le filtre actif. Le champ de recherche est en bas (zone du pouce). Les résultats s'affichent au-dessus avec le statut actuel de chaque titre.

**Persistance** : Les résultats de recherche et votre position de défilement sont conservés si vous quittez la page puis y revenez (bouton retour depuis une fiche titre). Les résultats sont vidés si vous changez d'onglet dans la barre de navigation.

### Ajouter un titre

Trois méthodes :

1. **Coller un lien** : URL IMDb, TMDB (movie/tv), AniList ou **TheTVDB** (`thetvdb.com/series/<slug>` ou `thetvdb.com/movies/<slug>`) → PlexTracker résout automatiquement les IDs et affiche le titre pour confirmation
2. **Rechercher par nom** : recherche dans TMDB/AniList
3. **Partage Android/iOS** : PlexTracker s'enregistre comme cible de partage. Depuis l'app IMDb, TVDB ou un navigateur, utiliser "Partager" → "PlexTracker" pour ajouter directement un titre.

Après confirmation, choisir le statut : En cours, Déjà vu, À regarder.
- "Déjà vu" marque tous les épisodes comme vus et propose de noter le titre.

### Fusion de titres

Il arrive que Plex ou l'import Simkl crée des doublons (ex: une série d'anime éclatée en plusieurs titres).

**Action "Merge into..."** : Depuis le détail d'un titre, dans le tiroir d'actions (ActionDrawer) → "Manage" → "Merge into...".
1. Rechercher le titre de destination (celui à conserver)
2. Confirmer la fusion

**Identification intelligente (Anime)** : Pour les anime, PlexTracker utilise Gemini AI pour identifier si le titre source est une saison spécifique (ex: Saison 2). Si c'est le cas, les épisodes fusionnés sont automatiquement décalés vers la bonne saison dans le titre de destination.

### Revue des matchs

Quand PlexTracker reçoit un nouveau titre via Plex, il tente de l'identifier automatiquement (TMDB, AniList, Gemini AI). Si l'identification n'est pas certaine :

- **Pending review** (confiance haute) : probablement correct, à confirmer
- **Unconfirmed** (confiance basse) : nécessite une vérification manuelle

Actions : Confirmer le match ou Corriger (re-recherche ou saisie manuelle d'IDs).

**Swipe actions** : glisser une carte vers la gauche révèle deux boutons — Confirm (vert) et Fix match (orange). Glisser loin exécute automatiquement l'action principale (Confirm).

Le bouton "Batch confirm" permet de confirmer tous les matchs "pending" d'un coup.

### Panneaux (BottomSheet)

Les panneaux (notation, édition, AniList, fix match) s'ouvrent en glissant depuis le bas. On peut les fermer en glissant vers le bas depuis n'importe quel endroit du panneau, en tapant le fond, ou avec le bouton retour Android. Le scroll de la page en arrière-plan est bloqué pendant l'ouverture.

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

### Stats

L'onglet Stats (icône chart, barre lavande) affiche un tableau de bord de l'ensemble de la bibliothèque.

**En un coup d'œil** : 4 cartes avec les chiffres-clés — titres suivis, épisodes vus, taux de complétion, note moyenne. Avec la répartition films/séries/anime en dessous.

**Notes** : barres horizontales montrant la distribution des notes de 10 à 1. Un texte d'insight résume la tendance (généreux, exigeant, etc.).

**Bibliothèque** : deux donuts montrant la répartition par statut (en cours, terminé, abandonné, à voir) et par type (film, série, anime).

**Le savais-tu ?** : cartes insight surprenantes — plus gros binge, série la plus fidèle, sprint complétion, oiseau de nuit/lève-tôt, Plex vs Manuel, écart de notes entre types, préférence de décennies, cimetière des titres abandonnés, pression du backlog, mois record. Chaque carte n'apparaît que si la donnée est pertinente.

**Année en cours** : 3 mini-cartes — titres ajoutés, épisodes vus, et complétions cette année.

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

PlexTracker synchronise automatiquement votre activité avec AniList : note, statut, et progression par épisode. Aucune action manuelle requise après la connexion initiale.

### Connexion

1. Paramètres → « Connecter AniList ».
2. Autoriser l'accès sur la page AniList.
3. Le token est conservé (~1 an de validité).

### Synchronisation automatique

Dès qu'un événement se produit dans PlexTracker, une mise à jour est envoyée à AniList :

- **Épisode marqué vu/non-vu** → progression + statut (CURRENT, COMPLETED) de la saison concernée.
- **Titre abandonné** → statut DROPPED pour chaque saison non terminée.
- **Note du titre modifiée** → note envoyée aux saisons déjà terminées ou abandonnées (les saisons en cours ne reçoivent pas de note tant qu'elles ne sont pas clôturées).

La synchronisation est **par saison**. AniList traite chaque saison comme une œuvre séparée : *Solo Leveling* S1 et *Solo Leveling S2 — Arise from the Shadow* sont deux entrées distinctes. PlexTracker envoie chaque saison à l'entrée AniList correspondante.

### Associer une saison à une entrée AniList

Pour la plupart des animes, le pipeline d'identification (Gemini AI) attribue automatiquement la bonne entrée AniList à chaque saison lors de l'import. Si une saison n'est pas mappée, un bandeau ambre « Not mapped for this season · Link entry » apparaît sur la saison active. Tapper « Link entry » ouvre un panneau de recherche AniList — sélectionner la bonne entrée et valider.

Pour corriger une association, taper le crayon ✎ dans le bandeau bleu de la saison active.

### Bouton AniList dans la barre d'actions

- **Films** : bouton visible, ouvre la fiche AniList du film.
- **Séries mono-saison** : bouton visible, ouvre la fiche de la saison unique.
- **Séries multi-saisons** : bouton masqué — chaque saison a son propre lien dans le bandeau par saison.

### Token expiré

Si le token AniList expire, une bannière rouge apparaît dans les paramètres : « AniList connection expired. Rating & status sync is paused ». Cliquer sur « Reconnect » relance le flux OAuth et réactive la synchronisation.

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
