# PlexTracker — Guide utilisateur

## Qu'est-ce que PlexTracker ?

PlexTracker est une application personnelle de suivi de visionnage. Elle remplace Simkl comme tracker central pour les films, séries et anime regardés sur Plex.

### Ce que fait PlexTracker

- **Suivi automatique** : chaque film ou épisode terminé sur Plex est automatiquement enregistré via webhook
- **Bibliothèque** : vue d'ensemble de tout ce qui est en cours, terminé, abandonné ou à regarder
- **Progression** : savoir en un coup d'œil où on en est dans chaque série
- **Notes** : noter les titres (1-10), avec liens vers IMDb et synchronisation automatique vers AniList
- **Import Simkl** : import initial de l'historique existant depuis un export Simkl

### Ce que PlexTracker ne fait PAS

- Ce n'est pas un agrégateur de critiques — pas de reviews, pas de scores critiques agrégés (IMDb/AniList s'en chargent en un clic)
- Pas de version desktop — interface mobile uniquement
- Pas de mode hors-ligne (le service worker reste online-first et ne met rien en cache)
- Pas de multi-utilisateur

---

## Accès

PlexTracker est une PWA (Progressive Web App) accessible depuis le navigateur Chrome sur Android.

1. Ouvrir `http://<nas-ip>:8080` dans Chrome
2. Se connecter avec le compte Google autorisé
3. Installer l'app : Chrome propose nativement une bannière « Installer PlexTracker » la première fois qu'on ouvre le site. Sinon, ouvrir le menu Chrome (⋮) → « Installer l'application » / « Ajouter à l'écran d'accueil »

**Raccourcis** : un appui long sur l'icône PlexTracker dans le launcher Android affiche 3 raccourcis : Ajouter un titre, Bibliothèque, Recherche.

**Badge** : l'icône affiche le nombre de titres en attente de révision (pending review + unconfirmed). Le compteur se met à jour à l'ouverture de l'app et après chaque action dans Match Review.

---

## Écrans

### Bibliothèque (accueil)

L'écran principal affiche tous les titres organisés par statut.

**Bandeau de stats** : juste sous le titre « Library », une ligne discrète résume l'année en cours — `2026 · 47 watched · ★ 7.8 avg · 3h this week`.

**Affichage par défaut** : grille de posters de toute la bibliothèque. Quand le filtre est `Watching` (En cours) ou `Caught up` (À jour), l'écran bascule en cartes horizontales avec barre de progression et badge « prochain épisode ».

**Filtres de statut** : ouvrir le tiroir de filtres pour basculer entre All, Plan, Watching, Caught up, Completed, Dropped. Les filtres actifs sont rappelés sur la barre du tiroir replié.

**Tri** : le tiroir de filtres propose 6 options de tri (dernière mise à jour, titre, date de sortie, note, date d'ajout, dernier visionnage). Taper un chip l'active ; taper à nouveau inverse la direction (↑/↓). Le tri est masqué pendant la recherche et persiste via localStorage.

**Badge film/série** : un petit pictogramme se superpose à chaque vignette pour distinguer d'un coup d'œil les films des séries (en grille comme en cartes horizontales).

**Badge de statut « à jour »** : pour une série en cours dont tous les épisodes déjà diffusés ont été vus, le badge de statut affiche « CAUGHT UP » (vert) au lieu de « WATCHING ». On repère ainsi directement dans la liste les séries à jour, sans passer par le filtre `Caught up`. Les épisodes pas encore diffusés ne comptent pas : le badge repasse à « WATCHING » dès qu'un nouvel épisode sort.

**Action rapide** : le badge rond sur chaque carte "En cours" affiche le numéro du prochain épisode. Un tap marque cet épisode comme vu et passe au suivant.

**Sélection multiple** : un appui long (~500 ms) sur une vignette active le mode sélection (vibration courte). Toucher d'autres vignettes les coche/décoche. Actions disponibles : changement de statut ou suppression en lot.

**Pull-to-refresh** : tirer vers le bas pour rafraîchir la page. Un indicateur circulaire suit le doigt ; il passe en teal avec une vibration au franchissement du seuil, puis tourne pendant le chargement. Disponible sur Library, Search, Validate, Match Review et Admin Notifications.

**Bannière de revue** : si des titres ont un match à vérifier, une bannière rouge apparaît sous le titre "Library" avec le nombre de titres concernés.

**Coming Up / Continue Watching** : sous le bandeau de stats, deux lignes — `// COMING UP` et `// CONTINUE WATCHING` — affichent un aperçu de 3 posters et le nombre de titres concernés. Taper sur une ligne ouvre une vue dédiée plein écran (grille 3 colonnes) : Coming Up affiche les titres qui sortent bientôt avec un badge de date (`Today`, `Mon`, `in 12d`…), Continue Watching affiche les séries en cours avec leur barre de progression et le prochain épisode. Flèche retour en haut à gauche pour revenir à la bibliothèque.

### Détail d'un titre

- **Couverture** en haut, avec teinte dominante extraite (accent), bouton retour.
- **Identité** : titre, année, durée/saisons, statut série, pills de genres, pastille de statut.
- **Carte « My rating »** : note personnelle (sur 10) à gauche ; à droite, les notes externes TMDB et AniList (en %) quand disponibles. Pas de note TVDB.
- **Carte « Synopsis »** : résumé du titre avec bouton « Show more » / « Show less » pour développer.
- **Carte « Cast & Crew »** : liste des acteurs principaux et de l'équipe (rôles). Taper un nom ouvre la page **Person** qui filtre la bibliothèque sur tous les titres où cette personne apparaît.
- **Carte « Details »** : Added, Last watched, Watch time (cumulé), Last refreshed, Original title, et une section **« Autres titres »** listant les titres dans les autres langues et les alias connus (chacun avec son drapeau 🇫🇷/🇬🇧/🇯🇵), sans doublon ni répétition du titre déjà affiché. Ces titres alternatifs sont complétés au fil des rafraîchissements (traductions TMDB/TVDB).
- **Bouton « Historique »** : ouvre une vue plein écran des sessions de visionnage du titre, avec regroupement automatique des épisodes consécutifs en plages (ex: `S1 E1–4 · 12 avr.`).
- **Barre de progression** (séries/anime) : `S2 · 7 of 10 episodes watched`.
- **Onglets de saison** : chaque saison est un pill. Vert = terminée (avec note), Ambre = en cours (avec progression), Gris = pas commencée.
- **Bandeau AniList par saison** (anime uniquement) : entre la barre de progression et la liste d'épisodes — score communautaire, lien AniList, crayon ✎ pour corriger l'association. Une saison peut être liée à **plusieurs entrées AniList** (« Part 1 », « Part 2 »…) pour les saisons découpées en deux cours (ex. *Attack on Titan* — saison finale). Chaque part affiche son propre lien et son score.
- **Liste d'épisodes** : tap sur un épisode pour le marquer vu/non vu.
- **Tiroir « Actions »** (poignée glissable au-dessus de la navbar) :
  - **★ Rate** : ouvre le prompt de notation
  - **Edit** : ouvre l'édition de type / statut / nom affiché
  - **More** : déploie Rematch, Merge, Refresh
  - **Liens externes** (sur leur propre ligne) :
    - **IMDb** quand un `imdb_id` est connu
    - **TVDB** quand un `tvdb_id` est connu
    - **AniList** pour les films et séries mono-saison ; masqué pour les séries multi-saisons (chaque saison a son lien dans le bandeau bleu)

### Recherche

Recherche globale dans toute la bibliothèque, quel que soit le filtre actif. Le champ de recherche est en bas (zone du pouce). Les résultats s'affichent au-dessus avec le statut actuel de chaque titre.

**Persistance** : Les résultats de recherche et votre position de défilement sont conservés si vous quittez la page puis y revenez (bouton retour depuis une fiche titre). Les résultats sont vidés si vous changez d'onglet dans la barre de navigation.

### Ajouter un titre

Trois méthodes :

1. **Coller un lien** : URL IMDb, TMDB (movie/tv), AniList ou **TheTVDB** (`thetvdb.com/series/<slug>` ou `thetvdb.com/movies/<slug>`) → PlexTracker résout automatiquement les IDs et affiche le titre pour confirmation
2. **Rechercher par nom** : recherche dans TMDB/AniList
3. **Partage Android/iOS** : PlexTracker s'enregistre comme cible de partage. Depuis l'app IMDb, TVDB ou un navigateur, utiliser "Partager" → "PlexTracker" pour ajouter directement un titre.

Après confirmation, choisir le statut : **Watching** (En cours), **Plan to watch** (À regarder), ou **Completed** (Terminé).
- **Completed** marque tous les épisodes comme vus et déclenche le prompt de notation.

### Fusion de titres

Il arrive que Plex ou l'import Simkl crée des doublons (ex: une série d'anime éclatée en plusieurs titres).

**Action "Merge"** : Depuis le détail du titre à **fusionner (et supprimer)**, ouvrir le tiroir d'actions (ActionDrawer) → **More** → **Merge**.
1. Rechercher le titre de destination (celui à conserver)
2. Confirmer la fusion

**Identification intelligente (Anime)** : Pour les anime, PlexTracker utilise Gemini AI pour identifier si le titre source est une saison spécifique (ex: Saison 2). Si c'est le cas, les épisodes fusionnés sont automatiquement décalés vers la bonne saison dans le titre de destination.

### Revue des matchs

Quand PlexTracker reçoit un nouveau titre via Plex, il tente de l'identifier automatiquement (TMDB, AniList, Gemini AI).

**Confirmation automatique** : quand Gemini est très confiant dans son identification, le match est confirmé directement — sans passer par la file de revue. La revue des matchs ne contient donc que les cas ambigus qui méritent vraiment une vérification.

Si l'identification n'est pas certaine :

- **Pending review** (confiance haute) : probablement correct, à confirmer
- **Unconfirmed** (confiance basse) : nécessite une vérification manuelle

Actions : Confirmer le match ou Corriger (re-recherche ou saisie manuelle d'IDs).

**Liens de vérification** : chaque carte de revue affiche les liens vers les bases externes disponibles (Simkl, IMDb, TMDB, AniList) pour identifier rapidement le titre avant de confirmer.

**Titre introuvable** : certains titres n'existent dans aucune base (TMDB, IMDb, AniList). Dans ce cas le bouton principal devient **Keep as-is** : il accepte le titre tel quel (nom Plex conservé, sans métadonnées externes) et le sort de la file de revue. Le titre reste suivi dans la bibliothèque.

**Swipe actions** : glisser une carte vers la gauche révèle deux boutons — Confirm / Keep as-is (vert) et Fix match (orange). Glisser loin exécute automatiquement l'action principale.

Le bouton "Batch confirm" permet de confirmer tous les matchs "pending" d'un coup.

**Récemment confirmés automatiquement** : en bas de l'écran de revue, une section "Recently auto-matched" liste les titres récemment confirmés automatiquement. Permet de repérer et corriger rapidement un faux positif sans attendre de le trouver dans la bibliothèque.

### Panneaux (BottomSheet)

Les panneaux (notation, édition, AniList, fix match) s'ouvrent en glissant depuis le bas. On peut les fermer en glissant vers le bas depuis n'importe quel endroit du panneau, en tapant le fond, ou avec le bouton retour Android. Le scroll de la page en arrière-plan est bloqué pendant l'ouverture.

### Notation

Le prompt de notation apparaît automatiquement quand :
- Un film est marqué comme vu
- Le dernier épisode d'une saison est marqué comme vu
- Le statut est changé en "Abandonné" ou "Terminé"

Options :
- **Save rating** : enregistre la note. Pour un anime, la synchronisation vers AniList est automatique (note envoyée à toutes les saisons terminées ou abandonnées).
- **IMDb · Save & rate** : enregistre et ouvre la page IMDb pour noter là-bas aussi (visible quand le titre a un `imdb_id`).
- **Skip for now** : ferme sans noter.

### Stats

L'onglet Stats (icône chart, barre lavande) affiche un tableau de bord de l'ensemble de la bibliothèque.

**En un coup d'œil** : 4 cartes avec les chiffres-clés — titres suivis, épisodes vus, taux de complétion, note moyenne. Avec la répartition films/séries/anime en dessous.

**Notes** : barres horizontales montrant la distribution des notes de 10 à 1. Un texte d'insight résume la tendance (généreux, exigeant, etc.).

**Bibliothèque** : deux donuts montrant la répartition par statut (en cours, terminé, abandonné, à voir) et par type (film, série, anime).

**Le savais-tu ?** : cartes insight surprenantes — plus gros binge, série la plus fidèle, sprint complétion, oiseau de nuit/lève-tôt, Plex vs Manuel, écart de notes entre types, préférence de décennies, cimetière des titres abandonnés, pression du backlog, mois record. Chaque carte n'apparaît que si la donnée est pertinente.

**Année en cours** : 3 mini-cartes — titres ajoutés, épisodes vus, et complétions cette année.

**Recent activity** : flux paginé des derniers événements de visionnage. Les épisodes consécutifs d'une même saison sont automatiquement regroupés en plages — `S1 E1–4` plutôt que quatre lignes séparées. Les rewatches et les déclenchements webhook en double sont absorbés dans la plage courante.

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

## Suivi automatique Jellyfin

PlexTracker accepte aussi les webhooks de Jellyfin, en parallèle de Plex (les deux peuvent rester actifs). Le traitement est identique : un film ou épisode terminé est identifié, marqué vu, et déclenche une notification de notation. Les événements sont enregistrés avec la source `jellyfin`.

**Seul un visionnage *terminé* compte** : PlexTracker n'agit que sur l'événement `PlaybackStop` dont l'indicateur « lu jusqu'à la fin » est vrai (équivalent du seuil ~90% de Plex). Les simples « lecture démarrée » et les arrêts en cours de visionnage sont ignorés.

### Configuration côté Jellyfin

1. Installer le plugin **Webhook** (Tableau de bord → Plugins → Catalogue → Webhook), puis redémarrer le serveur.
2. Tableau de bord → Plugins → **Webhook** → **Add Generic Destination**.
3. Renseigner :
   - **Webhook URL** : `https://<adresse-plextracker>/api/webhook/jellyfin/<secret>` (le `<secret>` doit correspondre à la variable d'environnement `JELLYFIN_WEBHOOK_SECRET`).
   - **Notification Type** : cocher **Playback Stop** (et **Playback Start** est inutile, laissé décoché).
   - **Item Type** : cocher **Movies** et **Episodes**.
   - **Send All Properties** : laissé décoché — on utilise le template ci-dessous.
   - **Template** (corps JSON) : coller exactement
     ```handlebars
     {
       "notification_type": "{{NotificationType}}",
       "item_type": "{{ItemType}}",
       "name": "{{{Name}}}",
       "year": "{{Year}}",
       "played_to_completion": "{{PlayedToCompletion}}",
       "provider_imdb": "{{Provider_imdb}}",
       "provider_tmdb": "{{Provider_tmdb}}",
       "provider_tvdb": "{{Provider_tvdb}}",
       "item_id": "{{ItemId}}",
       "series_name": "{{{SeriesName}}}",
       "series_id": "{{SeriesId}}",
       "season": "{{SeasonNumber}}",
       "episode": "{{EpisodeNumber}}"
     }
     ```
   - **Request Header** (optionnel) : ajouter `Content-Type` = `application/json`.

Toutes les valeurs sont volontairement entre guillemets : un champ absent (ex. `SeasonNumber` pour un film) se rend alors en chaîne vide sans casser le JSON.

> **Note migration Plex → Jellyfin** : les films sont dédupliqués automatiquement avec les titres déjà créés par Plex (via les IDs TMDB/IMDb/TVDB du webhook). Les séries sont rapprochées par nom + année. Tant qu'un même titre n'est pas regardé activement sur les deux serveurs en même temps, aucun doublon n'est créé.

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

**Saisons en deux parties.** Certaines saisons existent sous forme de deux entrées AniList (une « Part 1 » et une « Part 2 »). Via le crayon ✎ du bandeau, on peut lier plusieurs entrées à une même saison, les réordonner (▲ ▼) et en retirer. PlexTracker répartit alors les épisodes vus entre les parts (les premiers épisodes vont à la Part 1, les suivants à la Part 2…) et pousse la progression à chaque entrée. Lors d'une fusion, les entrées des deux parts sont conservées au lieu d'être écrasées.

### Rattachement automatique des saisons anime

Quand PlexTracker identifie un anime avec un ID AniList, il remonte automatiquement la chaîne de prequels pour déterminer si c'est une saison d'une série plus grande. Si c'est le cas, le titre est fusionné dans la série parente (créée au besoin) à la bonne position de saison — sans intervention de l'utilisateur.

**Protection franchise** : si la saison a sa propre identité externe (IMDb, TMDB, TVDB propres), elle n'est pas fusionnée automatiquement — elle reste un titre indépendant. Seules les entrées sans identité propre sont rattachées par les relations AniList seules.

Si le rattachement automatique s'est trompé, utiliser **Merge** depuis le tiroir d'actions pour corriger manuellement.

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
   - Confiance haute + vérifié → `confirmed` automatiquement (plus rien à faire)
   - Toute autre issue (confiance moyenne, basse, ou échec de vérification) → `unconfirmed` (à vérifier manuellement)
   - Si Gemini est indisponible → `pending_review` (à confirmer par l'utilisateur)

Si aucune étape ne trouve de match, le titre est créé quand même avec les métadonnées Plex disponibles, en statut `unconfirmed`. L'utilisateur peut corriger depuis l'écran de revue des matchs.

---

## Audit des saisons (Admin)

L'outil **Season Audit** (Admin → Season Audit) détecte les séries confirmées qui partagent un identifiant externe commun — signe qu'elles sont probablement des saisons d'une même série qui n'ont pas encore été fusionnées automatiquement.

Pour chaque groupe détecté, PlexTracker propose une fusion nommée (en s'appuyant sur les relations AniList quand disponibles). Chaque proposition indique le titre source, le titre de destination, et le numéro de saison suggéré.

**Actions disponibles par proposition :**
- **Accept** : fusionne le titre source dans la destination à la saison indiquée. L'opération est immédiate et irréversible (le titre source est supprimé).
- **Dismiss** : écarte définitivement cette proposition. La paire ne sera plus jamais suggérée.

Aucune fusion n'est automatique — toutes nécessitent une validation explicite.

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

**Réinitialisation complète + réimport** : pour repartir d'une base vide et tout réimporter (utile après une migration ou en cas de corruption) :

```bash
# Local
make reset-import BACKUP_FILE=/chemin/vers/Simkl_backup.zip

# NAS (le fichier doit être dans /volume1/downloads/)
make ssh-reset-import BACKUP_FILE=Simkl_backup.zip
```

Ces commandes suppriment les fichiers de base (db + WAL + SHM), redémarrent le conteneur (qui rejoue les migrations sur une base vide), puis réimportent le backup. Elles refusent de s'exécuter sans `BACKUP_FILE`.

---

## Déploiement

PlexTracker tourne dans un seul conteneur Docker sur le Synology DS920+.

```bash
# Mise à jour
docker compose pull && docker compose up -d
```

Variables d'environnement requises : voir `docker-compose.yml`.
