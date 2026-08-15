# Intégrations & Webhooks

[← Retour à l'index](INDEX.md)

---

## 1. Webhook Jellyfin (Temps Réel)

PlexTracker écoute les événements de visionnage envoyés par Jellyfin. Lorsqu'un film ou un épisode est lu jusqu'à la fin, Jellyfin émet un webhook `PlaybackStop`.

### Traitement côté PlexTracker
1. Identification du titre via les identifiants externes (TMDB, IMDb, TVDB) ou via le pipeline de matching.
2. Marquage automatique de l'épisode ou du film comme vu.
3. Mise à jour du statut (ex: série terminée si dernier épisode vu → *Completed*).
4. Envoi d'une notification push (pour inviter à noter).

> **Règle d'achèvement** : Seul un visionnage terminé déclenche l'enregistrement (`PlayedToCompletion` = `true`). Les arrêts en cours de lecture sont ignorés.

### Configuration du plugin Jellyfin

1. Installer le plugin **Webhook** dans Jellyfin (*Tableau de bord → Plugins → Catalogue → Webhook*), puis redémarrer Jellyfin.
2. Aller dans *Tableau de bord → Plugins → Webhook* → **Add Generic Destination**.
3. Renseigner les champs suivants :
   - **Webhook URL** : `https://<adresse-plextracker>/api/webhook/jellyfin/<secret>` (le `<secret>` doit correspondre à `JELLYFIN_WEBHOOK_SECRET`).
   - **Notification Type** : Cocher uniquement **Playback Stop**.
   - **Item Type** : Cocher **Movies** et **Episodes**.
   - **Send All Properties** : Laisser **décoché**.
   - **Template (corps JSON)** :
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
   - **Request Header** : `Content-Type` = `application/json`.

---

## 2. Synchronisation AniList

PlexTracker synchronise automatiquement votre activité d'anime avec AniList (notes, statuts, progression par épisode).

### Connexion OAuth
1. Aller dans Paramètres → *Connecter AniList*.
2. Autoriser l'accès sur la page d'authentification AniList.
3. Le token OAuth est conservé de manière sécurisée (~1 an de validité). Si le token expire, un bandeau d'avertissement dans les paramètres permet de le renouveler en un clic.

### RÈGLES DE SYNCHRONISATION
- **Par Saison** : AniList traite chaque saison d'un anime comme une fiche distincte (*Solo Leveling* S1 et S2 sont deux entrées séparées sur AniList). PlexTracker synchronise chaque saison avec sa fiche dédiée.
- **Progression** : Marquer un épisode comme vu dans PlexTracker met à jour la progression et le statut (*CURRENT*, *COMPLETED*) sur AniList.
- **Notes** : Modifier la note d'un titre envoie la note aux saisons terminées ou abandonnées sur AniList.

### Saisons en deux parties (Parts)
Certaines saisons sont divisées en plusieurs entrées sur AniList (ex: *Part 1*, *Part 2*).
- Via le crayon ✎ du bandeau AniList d'une saison, vous pouvez rattacher **plusieurs entrées AniList** à une même saison PlexTracker.
- PlexTracker répartit automatiquement les épisodes vus entre les différentes parts.

### Rattachement automatique des prequels
Lors de l'import d'un anime via AniList, PlexTracker remonte automatiquement les liens de prequels pour fusionner la saison dans la série parente au bon numéro de saison, sauf si la saison possède ses propres identifiants externes distincts (TMDB/TVDB).

---

## 3. Intégration Radarr & Sonarr (*arr)

PlexTracker s'intègre avec vos instances **Radarr** (pour les films) et **Sonarr** (pour les séries) pour surveiller l'état des téléchargements et ajouter automatiquement des médias à vos gestionnaires de téléchargement.

### Configuration (`/admin/arr`)

1. Ouvrir l'écran d'administration **Arr** via les paramètres (`/admin/arr`).
2. Configurer pour chaque service :
   - **URL de l'instance** (ex: `http://192.168.1.50:7878` pour Radarr, `http://192.168.1.50:8989` pour Sonarr).
   - **Clé d'API** (trouvée dans *Paramètres → Général → Sécurité* dans Radarr/Sonarr).
   - **Dossier racine par défaut** (*Root Folder*).
   - **Profil de qualité par défaut** (*Quality Profile*).

### Déclenchement de téléchargement (« File Arr »)

Depuis la fiche d'un titre ([TitleDetail](interface.md#2-détail-dun-titre)) :
1. Cliquer sur le bouton **File Arr** (ou via le menu *More → File Arr*).
2. Vérifier ou ajuster le dossier racine, le profil de qualité, l'option de recherche immédiate et de surveillance.
3. Valider pour envoyer le titre en tâche de fond (`TaskTypeRadarrPush` / `TaskTypeSonarrPush`).
4. Une fois envoyé, l'identifiant Radarr/Sonarr est enregistré et les badges de statut s'affichent automatiquement.

### Suivi en temps réel (`/admin/arr/queue`)

La vue de file d'attente centralise tous les téléchargements actifs :
- Barre de progression en temps réel (pourcentage et mégaoctets téléchargés).
- Temps restant estimé.
- Protocole utilisé (Usenet vs BitTorrent).
- Liens directs vers les fiches PlexTracker correspondantes.
