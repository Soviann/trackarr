# Roadmap de Refonte UI/UX & Craft Design Trackarr

Date de cadrage : 2026-10-04  
Statut : **Validé (Prêt pour exécution séquentielle)**  
Cibles prioritaires : **Mobile First (Android PWA / WebAPK)** avec extension adaptative **Desktop / Grand écran**.

---

## 🧭 Principes Directeurs
1. **Contrôle à une main (One-Handed First / Android)** : Toutes les actions d'exploration, de recherche, de filtrage et de navigation sont amarrées en bas d'écran ou dans des tiroirs coulissants (bottom sheets). Aucune action critique ne doit forcer le pouce à aller chercher le sommet de l'écran.
2. **Élimination du "AI Slop" & Fin des Dérives Dev/Terminal** : Fin des trigrammes cryptiques (`LIB`, `SCH`, `ADD`, `STA`, `ADM`), des en-têtes en commentaires de code (`// IN PROGRESS`), des rebonds cartoon élastiques (`springPop`), des bordures arbitraires de 3px et des découpes de dégradés sur le texte.
3. **Ergonomie Tactile & Accessibilité WCAG AA** : Cibles tactiles conformes (minimum 44×44px sur les boutons de jaquette), contraste des textes secondaires corrigé (≥ 4.5:1 sur `--ink-mute`), suppression des faux attributs ARIA.
4. **Performance & Rendu Sans À-Coups (Zero-Reflow)** : Élimination de l'animation des propriétés géométriques (`max-height`, `width`, `margin-bottom`) pour garantir un 60/120 FPS fluide sur Android.
5. **Mode Adaptatif Grand Écran Non-Destructif** : Rayonnage fluide en grille auto-fill (5 à 7 colonnes) avec barre latérale et recherche rapide clavier (`Cmd+K`) sur écran large, sans altérer l'expérience mobile.

---

## 🗂️ Découpage des Chantiers en Sessions Dédiées

Chaque section ci-dessous fera l'objet d'une **session de travail indépendante** avec son propre plan de mise en œuvre (`docs/plans/YYYY-MM-DD-<feature>.md`), ses tests de non-régression et sa revue par sous-agent.

```
┌────────────────────────────────────────────────────────────────────────┐
│ SECTION 1 : Ergonomie Tactile, Polish Mobile & Nettoyage "AI Slop"    │
│ (Bouton +1 44px, fin des rebonds cartoon, correction contrastes WCAG)  │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│ SECTION 2 : Fluidité & Zéro-Reflow (Animations & Gestuelle Mobile)    │
│ (Suppression max-height/width animés, SwipeActions, tiroirs GPU)       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│ SECTION 3 : Clarté UX & Navigation Unifiée Bas de Page (Android)      │
│ (Refonte Navbar, suppression trigrammes, recherche unifiée au pouce)   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│ SECTION 4 : Expérience Grand Écran & Rayonnage Adaptatif (Desktop)     │
│ (Grille fluide auto-fill 145px, Sidebar repliable, raccourcis Cmd+K)  │
└────────────────────────────────────────────────────────────────────────┘
```

---

### 📦 SECTION 1 : Ergonomie Tactile, Polish Mobile & Nettoyage "AI Slop"
*Objectif : Résoudre immédiatement les irritants tactiles quotidiens et assainir le design visuel.*

- [x] **Bouton d'action rapide sur jaquette ([`PosterCard.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/PosterCard.tsx) & [`PosterTile.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/PosterTile.tsx))** :
  - Agrandir la zone de frappe tactile à **44×44px minimum** (via `::after` hit area ou padding tactile).
  - Isoler strictement les événements tactiles (`stopPropagation`) pour éliminer les clics accidentels vers la fiche titre.
  - Remplacer le rebond cartoon `@keyframes springPop` (1.36× d'échelle) par une micro-transition de confirmation sobre et nette de 150ms.
- [x] **Contraste WCAG AA des Tokens ([`tokens.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/tokens.css))** :
  - Rehausser le contraste du jeton `--ink-mute` sur fond sombre de 2.83:1 à ≥ 4.5:1 (autour de `#968877`).
  - Corriger le contraste du bouton accent dans le thème Sunset.
- [x] **Suppression des tics de style IA (Side-tabs & Gradients)** :
  - Supprimer la bordure gauche arbitraire de 3px sur [`MatchReviewCard.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/MatchReviewCard.module.css) et le pseudo-élément 3px de [`NextEpisodeHero.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/NextEpisodeHero.module.css).
  - Remplacer les dégradés superposés sur le texte dans [`Wrapped.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/pages/Wrapped.module.css) par des contrastes solides nets.
- [x] **Accessibilité technique (Audit Lighthouse & ARIA)** :
  - Corriger les attributs `aria-label` orphelins sur `<div>` dans [`TypeBadge.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/TypeBadge.tsx) en ajoutant `role="img"`.
  - Ajouter le point de repère structurel `<main>` dans [`App.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/App.tsx).

---

### 📦 SECTION 2 : Fluidité & Zéro-Reflow (Animations & Gestuelle Mobile)
*Objectif : Garantir un défilement et des gestes à 60–120 FPS sur smartphone Android sans blocage de layout.*

- [x] **Optimisation de l'animation de sortie dans [`SwipeActions.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/SwipeActions.tsx)** :
  - Éliminer le reflow forcé synchrone (`containerEl.offsetHeight`) et la transition combinée `max-height` + `margin-bottom`.
  - Migrer vers un effondrement par CSS Grid (`grid-template-rows: 1fr -> 0fr`) avec accélération GPU (`transform: translateX` / `opacity`).
- [x] **Ouverture/Fermeture sans à-coups des tiroirs bas d'écran ([`ActionDrawer.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/ActionDrawer.module.css) & [`FilterDrawer.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/FilterDrawer.module.css))** :
  - Remplacer `transition: max-height` par une translation matérielle fluide `transform: translateY(...)`.
- [x] **Fluidification de la barre de progression ([`Admin.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/pages/Admin.module.css))** :
  - Remplacer l'animation de `width` par `transform: scaleX(...)` avec `transform-origin: left`.
- [x] **Résolution des conflits de gestes tactiles** :
  - Empêcher le conflit entre le glissement vers le bas du tiroir de filtres et le `PullToRefresh` natif de la page.

---

### 📦 SECTION 3 : Clarté UX & Navigation Unifiée Bas de Page (Android)
*Objectif : Réduire la charge cognitive et rendre l'application chaleureuse et intuitive à une main.*

- [x] **Refonte de la barre de navigation ([`Navbar.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/Navbar.tsx))** :
  - Supprimer les trigrammes abrégés (`LIB`, `SCH`, `ADD`, `STA`, `ADM`).
  - Adopter 4 ou 5 onglets explicites avec icônes universelles et libellés traduits (`Collection`, `Explorer`, `Calendrier`, `Stats`, `Admin`).
- [x] **Unification de la recherche & découverte au pouce ([`SearchBar.tsx`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/components/SearchBar.tsx))** :
  - Fusionner l'intention de recherche locale (`/search`) et d'ajout (`/add`) dans un champ unique docked en bas d'écran.
  - Résultat immédiat : recherche d'abord dans la collection personnelle, avec suggestion directe de découverte TMDB/AniList en un clic si le titre est absent.
- [x] **Humanisation du wording & dé-geekification** :
  - Remplacer les en-têtes préfixés par des commentaires de code (`// IN PROGRESS`, `// COMING UP`, `// RELEASES`) par une titraille éditoriale élégante.
  - Clarifier les badges d'état (remplacer `PLAN` par "À voir" / "Watchlist").
- [x] **Onboarding & états vides accueillants** :
  - Remplacer la phrase brute d'état vide par une carte d'accueil guidée expliquant les webhooks Jellyfin/Plex et la recherche.

---

### 📦 SECTION 4 : Expérience Grand Écran & Rayonnage Adaptatif (Desktop)
*Objectif : Offrir une interface digne d'un média center sur bureau sans impacter le mobile.*

- [x] **Grille responsive fluide ([`Library.module.css`](file:///Users/nicolasvasse/Soviann/plextracker/frontend/src/pages/Library.module.css))** :
  - Remplacer les 3 colonnes figées (`grid-template-columns: 1fr 1fr 1fr;`) par une grille adaptative intelligente `grid-template-columns: repeat(auto-fill, minmax(145px, 1fr));`.
  - Adapter la taille des jaquettes entre mobile (2 à 3 colonnes) et écrans larges (5 à 8 colonnes).
- [x] **Navigation latérale desktop (Sidebar repliable)** :
  - Sur écran large (≥ 1024px), basculer la barre d'onglets du bas vers une barre latérale élégante.
- [x] **Accélérateurs clavier pour Power Users** :
  - Ajout du raccourci universel `Cmd+K` ou `/` pour ouvrir instantanément la recherche.
  - Fermeture des tiroirs et modales avec la touche `Escape`.
  - Prise en charge de la sélection multiple par plage (Shift+Clic).

---

## 📅 Bilan du Revamp
Toutes les sections (Section 1 à 4) de la roadmap UI/UX ont été complétées avec succès.
