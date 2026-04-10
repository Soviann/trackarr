# Scroll automatique vers la saison active sur la page de détail d'une série

## Context

Quand une série a beaucoup de saisons (ex: Les Simpson, One Piece), la saison active (celle dont les épisodes sont affichés) peut se retrouver hors de l'écran dans la barre horizontale de badges. L'utilisateur ne voit pas quelle saison est sélectionnée et doit scroller manuellement.

## Plan

### Étape 1 — `scrollIntoView` sur le badge actif

**Fichier:** `frontend/src/components/SeasonTab.tsx`

- Ajouter une `ref` callback sur le bouton actif
- Appeler `scrollIntoView({ inline: 'center', block: 'nearest' })` quand le badge est actif (au mount et quand la saison change)

Approche concrète :
- Ajouter un `useEffect` + `useRef` dans `SeasonTab` : quand `active` est `true`, scroll le bouton dans la vue
- `behavior: 'instant'` au premier rendu pour éviter une animation visible au chargement, `'smooth'` lors d'un changement de saison par clic (mais le clic positionne déjà naturellement, donc `instant` suffit globalement)

### Étape 2 — Cacher la scrollbar du conteneur

**Fichier:** `frontend/src/pages/TitleDetail.module.css`

- Ajouter `scrollbar-width: none` + `::-webkit-scrollbar { display: none }` sur `.seasonTabs` pour un rendu plus propre (la scrollbar horizontale n'est pas utile visuellement puisque le scroll est automatique)

## Fichiers modifiés

- `frontend/src/components/SeasonTab.tsx` — ajout ref + scrollIntoView
- `frontend/src/pages/TitleDetail.module.css` — masquer scrollbar

## Vérification

1. `make build` pour vérifier la compilation
2. Ouvrir une série avec beaucoup de saisons (ex: Les Simpson)
3. Vérifier que le badge de la saison active est visible/centré
4. Cliquer sur une autre saison → les épisodes changent, le badge reste visible
