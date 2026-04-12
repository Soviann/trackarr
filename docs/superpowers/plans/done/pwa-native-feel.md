# PWA — Sensation d'application native

**Objectif** : Transformer PlexTracker en PWA qui donne les mêmes sensations qu'une vraie app Android, sans wrapping APK. L'app reste une PWA installable depuis Chrome, mais toutes les interactions tactiles standard des apps modernes sont présentes.

**Contexte** : Cible unique Android (Pixel 9 Pro, Android 16). Aucune compatibilité iOS à maintenir. Le `share_target` (partage depuis une autre app vers PlexTracker) est déjà en place — il n'est pas repris dans ce plan.

**Découpage en phases** : Chaque phase est indépendante et peut tourner dans sa propre session. La Phase 0 est un prérequis partagé par plusieurs phases suivantes (indiquée en tête de chacune). Les phases 1 à 6 n'ont pas de dépendance mutuelle et peuvent être menées en parallèle par plusieurs agents une fois la Phase 0 terminée.

---

## Phase 0 — Fondations partagées

**Pourquoi** : Avant de toucher aux écrans, poser les briques que plusieurs phases vont réutiliser : un petit vibreur pour les retours haptiques, un utilitaire d'appui long, et le blocage du "tirer pour rafraîchir" natif du navigateur (sinon il entre en conflit avec nos propres gestes).

### Ce qui change pour l'utilisateur
- Aucun changement visible tant que les phases suivantes ne sont pas livrées.
- Indirectement : dès qu'un écran est scrollé vers le haut et qu'on continue à tirer vers le bas, Chrome n'affiche plus son animation de rafraîchissement par-dessus l'app.

### Critères d'acceptation
- [ ] Sur la page Bibliothèque, tirer vers le bas quand on est en haut de la liste ne déclenche plus le rafraîchissement du navigateur (l'animation circulaire bleue de Chrome n'apparaît plus).
- [ ] Vérification manuelle : sur chaque page scrollable (Bibliothèque, Recherche, Validation, Match Review, Notifications admin), le comportement est identique — pas de rafraîchissement navigateur intempestif.
- [ ] Le scroll normal vers le haut et vers le bas continue de fonctionner sans ralentissement.

<details>
<summary>Note technique (scope)</summary>

Trois utilitaires à créer dans le front :
1. Un hook/fonction `useLongPress` (pointer events + timer ~500ms, annulation sur move/up).
2. Un helper `haptic(pattern)` wrappant `navigator.vibrate` avec garde de feature detection.
3. Application globale de `overscroll-behavior-y: contain` sur les conteneurs scrollables racines + `user-select: none` + `-webkit-touch-callout: none` sur les éléments interactifs concernés.

Aucun impact backend. Aucune nouvelle dépendance si implémenté à la main.
</details>

---

## Phase 1 — Sélection par appui long dans la Bibliothèque

**Prérequis** : Phase 0 terminée.

**Pourquoi** : Aujourd'hui, pour sélectionner plusieurs titres dans la Bibliothèque, il faut d'abord appuyer sur un bouton "Select" qui fait basculer toute la grille en mode sélection. C'est le pattern des sites web, pas des apps. Sur une vraie app (Google Photos, Files, Apple Photos), on appuie long sur un item et le mode sélection s'active automatiquement avec cet item déjà coché.

### Ce qui change pour l'utilisateur
- Le bouton "Select" disparaît de la barre d'actions de la Bibliothèque.
- Un appui maintenu ~500 ms sur n'importe quelle vignette de la grille déclenche :
  - Une petite vibration de confirmation (~10 ms).
  - L'entrée en mode sélection.
  - La vignette sous le doigt est automatiquement cochée.
- Une fois en mode sélection, les appuis courts sur les autres vignettes les ajoutent/retirent de la sélection (comportement actuel du mode sélection).
- La sortie du mode sélection se fait toujours via le bouton "Annuler" / "Terminer" de la barre d'actions (comportement actuel conservé).
- L'appui court hors mode sélection continue d'ouvrir la fiche du titre (comportement inchangé).

### Critères d'acceptation
- [ ] Hors mode sélection, un appui court sur une vignette ouvre la fiche du titre (comportement actuel).
- [ ] Hors mode sélection, un appui long sur une vignette : fait vibrer le téléphone, active le mode sélection, coche cette vignette.
- [ ] Le bouton "Select" n'est plus présent dans la barre d'actions de la Bibliothèque.
- [ ] En mode sélection, les appuis courts suivants cochent/décochent normalement les autres vignettes.
- [ ] Un appui long en mode sélection ne casse rien (ne ré-ouvre pas la fiche, ne re-déclenche pas une deuxième fois la sélection).
- [ ] Pas de sélection de texte parasite pendant l'appui long.
- [ ] Pas de menu contextuel du navigateur ("Enregistrer l'image", "Ouvrir dans un nouvel onglet") pendant l'appui long.
- [ ] Si le doigt bouge de plus de ~10 px pendant l'appui long, l'appui long est annulé (c'était un début de scroll, pas un long-press).
- [ ] Vérification visuelle in-browser via Chrome DevTools : login → Bibliothèque → tester les 3 parcours (tap court, long-press, scroll depuis une vignette).

<details>
<summary>Note technique (scope)</summary>

- Modifier le composant de grille de `Library.tsx` pour câbler `useLongPress` (Phase 0) sur chaque vignette.
- Retirer le bouton Select du header de la page.
- Le state `selectionMode` / `selectedIds` existe déjà (cf. commit `b90b0d4`), il suffit de le déclencher depuis le long-press.
- Pas de backend.
</details>

---

## Phase 2 — Pull-to-refresh personnalisé

**Prérequis** : Phase 0 terminée (le rafraîchissement navigateur doit déjà être bloqué).

**Pourquoi** : Sur Android, tirer vers le bas pour rafraîchir est un réflexe universel. Aujourd'hui, soit le geste déclenche le rafraîchissement du navigateur (qui recharge toute la PWA), soit il ne fait rien. On veut le geste "natif app" : un indicateur circulaire qui apparaît sous la barre du haut, et au relâchement, seules les données de l'écran sont rechargées (pas toute la PWA).

### Ce qui change pour l'utilisateur
- Sur les pages listées ci-dessous, quand on est en haut du contenu et qu'on tire vers le bas :
  - Une zone apparaît progressivement sous la barre du haut avec un indicateur circulaire qui suit le doigt.
  - Passé un seuil (~70 px), l'indicateur change d'état pour signaler "relâchez pour rafraîchir" (vibration courte).
  - Au relâchement au-dessus du seuil : l'indicateur se cale à sa position de chargement, l'app recharge les données, puis l'indicateur disparaît avec une petite animation.
  - Au relâchement en-dessous du seuil : l'indicateur se rétracte sans rien faire.
- Pages concernées : **Bibliothèque**, **Recherche** (résultats), **Validation**, **Match Review**, **Notifications admin**.
- Pendant le rafraîchissement, l'utilisateur peut continuer à scroller/interagir normalement.
- Aucune autre page n'est affectée (pages détail, Stats, Admin : pas de PTR, ce sont des vues statiques ou non-listes).

### Critères d'acceptation
- [ ] Sur la Bibliothèque, le geste fonctionne de bout en bout : tirer → indicateur → relâcher → données rafraîchies → indicateur disparaît.
- [ ] Idem sur chacune des 5 pages listées.
- [ ] Si on tire puis on remonte (annulation), aucune requête réseau n'est déclenchée.
- [ ] Si on est à mi-hauteur d'une liste et qu'on tire vers le bas, rien ne se passe (le geste est ignoré, le scroll normal prend la main).
- [ ] Pendant le rafraîchissement, déclencher un second geste pull-to-refresh ne lance pas un second appel réseau (idempotence).
- [ ] Vibration courte au franchissement du seuil.
- [ ] Le rafraîchissement du navigateur Chrome n'apparaît jamais pendant ces gestes (vérifié sur Pixel 9 Pro en conditions réelles).
- [ ] Vérification visuelle in-browser via Chrome DevTools pour chaque page.

<details>
<summary>Note technique (scope)</summary>

Deux options : lib (`pulltorefreshjs` — simple, ~3 kB) ou implémentation maison (~50 lignes). Recommandation : implémentation maison, petit composant `<PullToRefresh onRefresh={...}>` qui wrappe la zone scrollable. Réutilisable.

Chaque page listée doit câbler son propre callback `onRefresh` (refetch de la query existante). Zéro changement backend.
</details>

---

## Phase 3 — Swipe actions sur les listes en colonne unique

**Prérequis** : Phase 0 terminée.

**Pourquoi** : Swiper un item vers la gauche pour révéler des actions rapides est un pattern app très productif. La Bibliothèque étant une grille multi-colonnes, elle n'est pas concernée — mais les pages **Validation**, **Match Review** et **Notifications admin** affichent des listes en colonne unique qui s'y prêtent naturellement.

### Ce qui change pour l'utilisateur

**Sur la page Validation** (items à valider/rejeter) :
- Swipe vers la gauche sur une ligne → révèle à droite 2 boutons : ✓ Valider (vert) et ✗ Rejeter (rouge).
- Tap sur un bouton = exécute l'action + anime la sortie de la ligne vers la droite (Valider) ou vers la gauche (Rejeter).
- Vibration courte au franchissement du seuil d'ouverture des actions (~60 px).
- Swipe complet (dépassement ~40% de la largeur de la ligne) → exécute directement l'action principale (Valider) sans avoir à toucher le bouton.
- Taper ailleurs sur l'écran referme les actions révélées.

**Sur la page Match Review** :
- Swipe gauche → 2 actions : ✓ Confirmer et ↻ Voir d'autres propositions.
- Même logique que Validation (vibration, swipe complet = confirmer).

**Sur la page Notifications admin** :
- Swipe gauche → 1 action : 🗑 Supprimer.
- Swipe complet = supprimer directement, avec toast "Annuler" pendant 4 secondes.

### Critères d'acceptation
- [ ] Sur chacune des 3 pages : swipe lent révèle les actions, swipe rapide/complet exécute l'action principale.
- [ ] Vibration courte au franchissement du seuil de révélation.
- [ ] Une seule ligne peut avoir ses actions révélées à la fois (ouvrir les actions sur une autre ligne referme automatiquement la précédente).
- [ ] Scroll vertical normal fonctionne toujours : si le geste commence plus vertical qu'horizontal, il est interprété comme scroll, pas comme swipe.
- [ ] Tap ailleurs sur l'écran referme les actions révélées.
- [ ] L'animation de sortie d'item (après action) est fluide et ne laisse pas de "trou" dans la liste.
- [ ] Sur Notifications admin : le toast "Annuler" fonctionne (restaure la notif supprimée si l'utilisateur clique dessus dans les 4 s).
- [ ] Vérification visuelle in-browser via Chrome DevTools sur les 3 pages.

<details>
<summary>Note technique (scope)</summary>

- Composant réutilisable `<SwipeActions right={[{icon, color, onAction, primary?: bool}]}>` wrappant chaque item.
- Recommandation lib : `framer-motion` (déjà probablement présent ou très léger) pour l'animation de sortie + gestion du drag. Alternative pure touch events si aucune lib anim n'est déjà dans le projet.
- Soft delete avec toast annulable pour Notifications admin → nécessite soit un endpoint DELETE qui supporte un délai, soit un état local "pending delete" avant l'appel réel (option 2 plus simple, recommandée).
</details>

---

## Phase 4 — Bottom sheets (panneaux glissants)

**Prérequis** : Phase 0 terminée.

**Pourquoi** : Les modales centrées avec fond assombri sont typiques du web. Les apps Android modernes utilisent à la place des **bottom sheets** : des panneaux qui montent depuis le bas de l'écran, occupent la demi-hauteur ou la hauteur complète, et se ferment en les tirant vers le bas. C'est plus ergonomique au pouce, plus naturel.

### Ce qui change pour l'utilisateur

Les modales existantes suivantes deviennent des bottom sheets :

1. **Actions d'un titre** (menu `…` sur une vignette ou sur la fiche titre) : liste d'actions verticale, monte depuis le bas sur ~40% de la hauteur.
2. **Filtres de la Bibliothèque** : panneau plein hauteur qui monte depuis le bas, avec barre de fermeture en haut.
3. **Sélection de liste/tags** lors de l'ajout d'un titre : ~60% de la hauteur.
4. **Confirmations destructives** (ex : "Supprimer ce titre ?") : petit sheet ~25% de la hauteur avec titre, message, 2 boutons.

Comportement commun de tous les bottom sheets :
- Montent avec une animation de slide-up + fond qui s'assombrit progressivement.
- Une poignée visuelle (petite barre horizontale) en haut du sheet indique qu'il est tirable.
- Geste : tirer le sheet vers le bas suit le doigt ; relâcher au-delà de 30% de sa hauteur le ferme, en-dessous il revient en place.
- Taper sur le fond assombri ferme le sheet.
- Appui sur le bouton retour Android (back gesture) ferme le sheet en priorité, avant de quitter la page.

### Critères d'acceptation
- [ ] Les 4 modales listées sont remplacées par des bottom sheets.
- [ ] Aucune autre modale n'est transformée dans le cadre de cette phase (scope limité — d'autres suivront en suivant ce pattern).
- [ ] Tous les sheets ont la poignée en haut, l'animation d'entrée et de sortie, le fond assombri tapable.
- [ ] Le geste drag-to-dismiss fonctionne de manière fluide (pas de saccade, suit le doigt en temps réel).
- [ ] Le bouton retour Android ferme le sheet au lieu de quitter la page (vérifié en PWA installée).
- [ ] Pendant qu'un sheet est ouvert, le scroll de la page en arrière-plan est bloqué.
- [ ] Tirer un sheet vers le haut (au-delà de sa hauteur) : il reste calé, ne déborde pas.
- [ ] Vérification visuelle in-browser via Chrome DevTools pour chacun des 4 cas.

<details>
<summary>Note technique (scope)</summary>

Recommandation : lib `vaul` (spécialisée bottom sheets, ~5 kB, React/Preact compatible) ou équivalent. Alternative custom avec framer-motion + gesture handler.

À noter : le blocage du back Android demande d'utiliser l'History API pour pousser une entrée fictive quand le sheet s'ouvre, et écouter `popstate` pour le fermer. Pattern standard mais à ne pas oublier.

Scope d'écrans affectés : uniquement les 4 cas listés. Les autres modales pourront migrer ultérieurement en suivant le composant créé ici.
</details>

---

## Phase 5 — Raccourcis depuis l'icône de l'app (App Shortcuts)

**Prérequis** : Aucun.

**Pourquoi** : Sur Android, un appui long sur l'icône d'une app affiche un menu de raccourcis rapides (ex : Gmail → "Composer un mail"). Pour une PWA installée, ces raccourcis sont déclarés dans le `manifest.json`. Ajout quasi-gratuit, gros gain de praticité.

### Ce qui change pour l'utilisateur
- Appui long sur l'icône PlexTracker dans le launcher Android → menu avec 3 raccourcis :
  1. **Ajouter un titre** → ouvre directement l'écran Add.
  2. **Bibliothèque** → ouvre directement la Bibliothèque.
  3. **Recherche** → ouvre directement l'écran Search.
- Chaque raccourci a une icône dédiée dans le menu du launcher.
- Tap sur un raccourci ouvre la PWA déjà sur l'écran demandé (pas de passage par l'accueil).

### Critères d'acceptation
- [ ] Le manifest déclare 3 `shortcuts` avec leurs URL cibles (`/add`, `/library`, `/search`) et leurs icônes.
- [ ] Chaque raccourci a une icône 96×96 minimum (fournir `shortcut-add.png`, `shortcut-library.png`, `shortcut-search.png` dans `frontend/public/`).
- [ ] Après réinstallation de la PWA sur le Pixel 9 Pro, l'appui long sur l'icône affiche les 3 raccourcis.
- [ ] Chaque raccourci ouvre la PWA directement sur l'écran cible.
- [ ] Vérification visuelle sur le téléphone (pas testable en DevTools desktop).

<details>
<summary>Note technique (scope)</summary>

Uniquement éditions dans `frontend/public/manifest.json` + 3 fichiers PNG à créer (peuvent être générés depuis l'icône principale par variation de teinte, ou des pictos simples).

Attention : le manifest doit être re-téléchargé par le service worker → bump de version du SW ou réinstallation de la PWA nécessaire pour que les raccourcis apparaissent.
</details>

---

## Phase 6 — Badge sur l'icône (propositions à traiter)

**Prérequis** : Aucun. (Indépendant des autres phases.)

**Pourquoi** : Quand des propositions de matching attendent une décision, rien ne le signale depuis l'icône. L'API Badging permet d'afficher un petit chiffre sur l'icône de la PWA installée (comme Gmail, WhatsApp). L'utilisateur voit d'un coup d'œil qu'il a du travail en attente sans ouvrir l'app.

### Ce qui change pour l'utilisateur
- Un chiffre apparaît sur l'icône PlexTracker dans le launcher Android = nombre de **séries** ayant au moins une proposition à traiter (pas le nombre de propositions — le nombre de séries distinctes concernées).
- Le badge s'actualise automatiquement :
  - À l'ouverture de l'app.
  - Après validation/rejet d'une proposition (décrément immédiat).
  - Périodiquement en arrière-plan via le service worker (toutes les ~15 min, quand le réseau est disponible).
  - Via push : si une nouvelle proposition arrive, la push de notif déclenche aussi la mise à jour du badge.
- Si aucune proposition en attente : le badge disparaît complètement (pas de "0" affiché).
- L'utilisateur peut désactiver le badge depuis les Paramètres (nouveau toggle "Afficher le badge sur l'icône").

### Critères d'acceptation
- [ ] Endpoint backend exposant le compte de séries avec ≥1 proposition en attente (typé, testé).
- [ ] À l'ouverture de l'app, le front récupère ce compte et appelle `navigator.setAppBadge(n)` (ou `clearAppBadge()` si 0).
- [ ] Après validation ou rejet d'une proposition dans l'app, le badge se met à jour dans la foulée.
- [ ] Le service worker a une tâche périodique (Periodic Background Sync ou fallback : simple fetch à chaque réveil du SW) qui rafraîchit le badge.
- [ ] Le badge apparaît visuellement sur l'icône de la PWA installée sur le Pixel 9 Pro.
- [ ] Le badge disparaît quand le compteur tombe à 0.
- [ ] Un toggle "Afficher le badge" est présent dans les Paramètres (ou Admin si pas de page paramètres), persisté en localStorage.
- [ ] Quand le toggle est off, aucun appel à `setAppBadge` n'est fait.
- [ ] Tests backend : comptage correct (vérifier qu'on compte bien les séries distinctes, pas les propositions).
- [ ] Vérification visuelle sur le téléphone après déploiement.

<details>
<summary>Note technique (scope)</summary>

- Backend : endpoint `GET /api/propositions/pending-count` (ou équivalent dans la structure routes actuelle). Query : `SELECT COUNT(DISTINCT series_id) FROM propositions WHERE status = 'pending'` (adapter aux vrais noms de tables).
- Front : wrapper autour de `navigator.setAppBadge` / `clearAppBadge` avec feature detection (ne pas planter si non supporté).
- Service worker : Periodic Background Sync n'est pas universellement dispo, prévoir un fallback via `BroadcastChannel` ou simple refresh à l'ouverture + après actions.
- Scope backend minimal : 1 endpoint + tests repository + tests handler.
</details>

---

## Plan de livraison conseillé

### Mode "sessions parallèles" (rapide)
1. Session A → **Phase 0** (bloquant, ~30 min de travail).
2. Une fois Phase 0 mergée sur `main` : lancer en parallèle 3 sessions → **Phases 1, 2, 3** (dépendent toutes de Phase 0).
3. En parallèle et dès le début : **Phase 5** (aucune dépendance, ~15 min) et **Phase 6** (aucune dépendance, plus gros car backend + front).
4. Après livraison des Phases 1/2/3 : **Phase 4** (bottom sheets) — évitée au début car elle peut toucher à des composants modifiés par la Phase 1.

### Mode "subagents depuis une session unique"
1. Session orchestratrice lance la Phase 0 en inline execution.
2. Une fois mergée, dispatch en parallèle 5 subagents Sonnet : Phases 1, 2, 3, 5, 6.
3. Au retour de 1/2/3 : dispatch Phase 4.

### Vérifications finales (toutes phases livrées)
- [ ] Test bout-en-bout sur Pixel 9 Pro : installer la PWA fraîchement, faire un parcours complet utilisant les nouvelles interactions (long-press, PTR, swipe, bottom sheet, shortcut, badge).
- [ ] `make lint` et `make test` verts.
- [ ] `CHANGELOG.md` mis à jour section `## [Unreleased]` avec un résumé utilisateur des nouveautés.
- [ ] `docs/patterns.md` mis à jour si de nouveaux composants partagés ont été ajoutés (PullToRefresh, SwipeActions, BottomSheet, useLongPress, haptic).
- [ ] `docs/user-guide.md` mis à jour avec les nouveaux gestes disponibles.
