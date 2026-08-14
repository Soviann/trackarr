# Aperçu & Accès

[← Retour à l'index](INDEX.md)

---

## Qu'est-ce que PlexTracker ?

PlexTracker est une application personnelle de suivi de visionnage. Elle remplace Simkl comme tracker central pour les films, séries et animes regardés sur Jellyfin et Plex.

### Ce que fait PlexTracker

- **Suivi automatique** : Chaque film ou épisode terminé sur Jellyfin ou Plex est automatiquement enregistré via webhook.
- **Bibliothèque centralisée** : Vue d'ensemble de tout ce qui est en cours, terminé, abandonné ou à regarder.
- **Progression en un coup d'œil** : Suivi exact des épisodes vus et des prochains épisodes à regarder.
- **Notes & Synchronisation** : Notation des titres (1-10), liens IMDb, et synchronisation automatique vers AniList.
- **Import Simkl** : Migration initiale de l'historique existant depuis un export Simkl.

### Ce que PlexTracker ne fait PAS

- **Pas d'agrégateur de critiques** : Pas de rédigés de reviews ou de scores critiques agrégés (IMDb/AniList s'en chargent en un clic).
- **Pas de version desktop** : Interface uniquement pensée et optimisée pour mobile (PWA).
- **Pas de mode hors-ligne** : Le service worker reste *online-first* et ne met pas l'application en cache hors-ligne.
- **Pas de multi-utilisateur** : Conçu pour un seul utilisateur principal.

---

## Accès & Installation PWA

PlexTracker est une **Progressive Web App (PWA)** accessible depuis le navigateur (Chrome recommandé sur Android/iOS).

### Premier accès & Connexion

1. Ouvrir l'URL de votre instance (ex: `http://<nas-ip>:8080`) dans Chrome.
2. Se connecter avec le compte Google autorisé.
3. **Installer l'app** : Chrome propose une bannière « Installer PlexTracker » lors de la première visite. Sinon, ouvrir le menu Chrome (⋮) → *Installer l'application* / *Ajouter à l'écran d'accueil*.

### Raccourcis Android

Un appui long sur l'icône PlexTracker dans le launcher Android affiche 3 raccourcis directs :
- **Ajouter un titre**
- **Bibliothèque**
- **Recherche**

### Compteur & Badge d'icône

L'icône de l'application affiche un badge indiquant le nombre de titres en attente de révision (*pending review* + *unconfirmed*). Le compteur se met à jour automatiquement à l'ouverture de l'application et après chaque action dans l'écran **Match Review**.
