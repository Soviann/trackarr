# Guide de l'Interface Utilisateur

[← Retour à l'index](INDEX.md)

---

## 1. Bibliothèque (Écran d'Accueil)

L'écran principal affiche l'ensemble des titres organisés par statut.

- **Bandeau de stats** : Résume l'année en cours sous le titre *Library* (ex: `2026 · 47 watched · ★ 7.8 avg · 3h this week`).
- **Affichage dynamique** : Grille de posters par défaut. Quand le filtre est *Watching* (En cours) ou *Caught up* (À jour), l'écran bascule automatiquement en cartes horizontales avec barre de progression et badge du prochain épisode.
- **Filtres de statut** : Le tiroir de filtres permet de basculer entre *All*, *Plan*, *Watching*, *Caught up*, *Completed*, *Dropped*.
- **Filtres avancés** :
  - **Statut de série** (quand Type = Series) : *Returning* (en cours), *Ended* (terminée), *Cancelled* (annulée), *Not started* (pas encore diffusée).
  - **Pays d'origine** : Filtre combinatoire par pays (ex: Corée du Sud, Japon).
  - **Notes minimales** : Filtre simultané sur *My rating* (note perso) et *TMDB* (note communautaire).
- **Tri** : 6 options de tri (dernière mise à jour, titre, date de sortie, note, date d'ajout, dernier visionnage). Taper un chip l'active ; taper à nouveau inverse la direction (↑/↓).
- **Action rapide** : Un tap sur le badge du prochain épisode marque l'épisode comme vu et passe au suivant.
- **Sélection multiple** : Appui long (~500 ms) sur une vignette pour activer la sélection multiple et appliquer des changements de statut ou de suppression en lot.
- **Pull-to-refresh** : Tirer vers le bas pour rafraîchir les données.
- **Vues Coming Up / Continue Watching** : Lignes d'aperçu menant vers des vues grille 3 colonnes dédiées.

---

## 2. Détail d'un Titre

Chaque fiche titre rassemble l'ensemble des informations et actions :

- **Identité** : Couverture avec couleur d'accent extraite, titre, année, durée/saisons, genres, pastille de statut.
- **Badge Prime** : Badge bleu **prime** si le titre est inclus sur Amazon Prime Video (France).
- **Notes** : Note personnelle (/10) à gauche, notes externes TMDB et AniList (%) à droite.
- **Cast & Crew** : Liste des acteurs et de l'équipe. Taper un nom ouvre la fiche **Person** filtrant la bibliothèque.
- **Détails & Titres alternatifs** : Dates d'ajout et dernier visionnage, temps cumulé, nom original, bouton **File Arr**, et liste des alias connus avec drapeau de langue (🇫🇷/🇬🇧/🇯🇵).
- **Historique de visionnage** : Vue des sessions de visionnage avec regroupement automatique des épisodes consécutifs en plages (ex: `S1 E1–4 · 12 avr.`).
- **Saisons & Épisodes** : Barre de progression, onglets par saison (vert = terminée, ambre = en cours, gris = pas commencée), et liste interactive d'épisodes.
- **Bandeau AniList par saison** (anime) : Liens et scores AniList par saison. Support des saisons découpées en plusieurs *parts* (*Part 1*, *Part 2*).
- **Tiroir « Actions »** :
  - **Rate** : Note personnelle.
  - **Edit** : Modifier type, statut, nom affiché.
  - **More** : File Arr, Rematch, Merge, Refresh, Delete.
  - **Liens externes** : Accès direct IMDb, TVDB, AniList.

---

## 3. Recherche & Ajout de Titres

### Recherche globale
Recherche instantanée dans toute la bibliothèque. La position de défilement et les résultats sont conservés lors des retours depuis une fiche titre.

### Ajouter un titre
- **Coller un lien** : URLs IMDb, TMDB, AniList ou TheTVDB (`thetvdb.com/series/<slug>` ou `/movies/<slug>`).
- **Recherche par nom** : Recherche directe dans TMDB/AniList.
- **Partage natif Android/iOS** : Sélectionner "Partager" → "PlexTracker" depuis l'application IMDb, TVDB ou un navigateur.

---

## 4. Fusion de Titres (Merge)

Pour regrouper les doublons (ex: animes éclatés par saison) :
1. Ouvrir la fiche du titre à fusionner et supprimer.
2. Tiroir Actions → **More** → **Merge**.
3. Rechercher le titre cible et valider.
4. **Intelligence Anime** : Si Gemini détecte une saison spécifique (ex: Saison 2), les épisodes vus sont automatiquement réattribués à la bonne saison sur le titre cible.

---

## 5. Match Review (Revue des Correspondances)

File de validation des titres importés via Plex/Jellyfin :

- **Auto-confirmation** : Les matchs à haute confiance Gemini sont confirmés automatiquement.
- **File de revue** : Seuls les cas ambigus apparaissent (*Pending review* ou *Unconfirmed*).
- **Actions** : Confirm, Keep as-is (pour les titres absents des bases externes), Fix match.
- **Gestes** : Swipe vers la gauche pour valider ou corriger rapidement.
- **Recently auto-matched** : Historique des confirmations automatiques récentes.

---

## 6. Notation & Panneaux (BottomSheet)

- **Prompt automatique** : Proposé automatiquement à la fin d'un film, d'une saison, ou lors du passage en *Completed* / *Dropped*.
- **Options** : *Save rating* (synchro AniList auto pour les animes), *IMDb Save & rate* (ouvre IMDb), ou *Skip*.
- **Panneaux glissants** : Tous les tiroirs (notation, édition, fix match) se manipulent par glissement vertical.

---

## 7. Statistiques & Insights

Onglet dédié présentant les métriques globales de la bibliothèque :
- **Chiffres clés** : Titres suivis, épisodes vus, complétion, note moyenne, répartition par type.
- **Distribution des notes** : Graphique en barres (10 à 1) avec insight de tendance.
- **Insight Cards ("Le savais-tu ?")** : Plus gros binge, fidélité, sprint de complétion, créneau horaire de visionnage, écart de notes, décennies préférées.
- **Activité récente** : Flux paginé avec regroupement intelligent des épisodes consécutifs.
