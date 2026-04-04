# Audit UX/UI complet — PlexTracker

## Contexte

Audit visuel de toutes les pages de l'application via Chrome DevTools (emulation mobile Pixel 7, 393x852). L'application contenait 7 459 titres au moment de l'audit. Ce plan couvre les problemes non deja traites par `2026-04-04-post-audit-fixes.md`.

**Deja couvert dans le plan post-audit (ne pas dupliquer) :** crash JS Library/Search/Detail (tache 1), onglet Watching par defaut (tache 2), pagination serveur (tache 3), metadonnees episodes (tache 4), bouton Confirm grise (tache 5).

---

## Tache 1 — P1 : Afficher les erreurs au lieu de "Loading..." infini

**Probleme :** Quand une requete API echoue, l'utilisateur reste bloque sur "Loading..." sans aucun feedback. Les erreurs sont capturees en interne mais jamais affichees.

**Comportement attendu :**
- Si une erreur survient, un message s'affiche : "Impossible de charger les donnees. Reessayer." avec un bouton "Reessayer"
- L'ecran "Loading..." ne doit jamais rester plus de 10 secondes sans feedback
- Si l'application plante completement (erreur JS non geree), un ecran de secours s'affiche au lieu d'une page blanche

**Criteres d'acceptation :**
- Library, Search, Detail, Match Review affichent un message d'erreur clair en cas d'echec API
- Le bouton "Reessayer" relance le chargement
- Un crash JS affiche un ecran de secours ("Quelque chose s'est mal passe") au lieu d'une page blanche

---

## Tache 2 — P1 : Corriger la recherche (resultats non pertinents)

**Probleme :** Chercher "Naruto" retourne 4 845 resultats dont aucun n'est Naruto. Les resultats sont totalement hors sujet (ex: "A Frozen Flower", "The Bodyguard from Beijing").

**Comportement attendu :**
- Chercher "Naruto" retourne d'abord les titres contenant exactement "Naruto", puis ceux proches
- Le nombre de resultats est proportionnel a la pertinence (pas des milliers de faux positifs)
- La recherche avec un mot precis (ex: "Breaking Bad") retourne le bon titre en premier

**Criteres d'acceptation :**
- "Naruto" retourne les titres Naruto en premiers resultats
- "Breaking Bad" retourne Breaking Bad en premier resultat
- Pas plus de ~50 resultats pour un terme precis
- Les resultats sont tries par pertinence (match exact > prefixe > fuzzy)

---

## Tache 3 — P2 : Placeholder pour les titres sans couverture

**Probleme :** Les titres sans image de couverture (ex: Breaking Bad id 7458) affichent un bloc noir sans aucune indication visuelle — ni dans les cartes de la Library, ni sur la page Detail.

**Comportement attendu :**
- Carte Library : une icone ou un fond colore remplace l'image manquante (ex: icone film/serie/anime selon le type)
- Page Detail : le header montre un fond degrade avec l'icone du type au lieu d'un bloc noir
- Le titre reste lisible dans tous les cas

**Criteres d'acceptation :**
- Un titre sans `cover_url` affiche un placeholder visible sur Library et Detail
- Le placeholder est different selon le type (film, serie, anime)

---

## Tache 4 — P2 : Page de revue accessible directement

**Probleme :** Naviguer vers `/review` ou `/match-review` directement affiche une page vide (seule la navbar est visible). L'acces fonctionne uniquement via la banniere rouge sur Library.

**Comportement attendu :**
- `/match-review` s'affiche correctement meme en navigation directe (bookmark, refresh)
- Si aucun titre n'est a revoir, un message "Aucun titre a verifier" s'affiche

**Criteres d'acceptation :**
- Ouvrir `/match-review` dans un nouvel onglet affiche la page de revue
- Page vide avec message explicite si rien a revoir

---

## Tache 5 — P3 : Coherence de langue sur la page Login

**Probleme :** La page Login melange francais et anglais :
- Bouton Google : "Se connecter avec Google" (FR)
- Tagline : "Track your media library" (EN)
- Aide : "Sign in with your Google account to get started" (EN)
- Section : "Dev Login" (EN)

**Comportement attendu :**
- Toute l'interface est en anglais (langue cible de l'app)
- OU toute l'interface est en francais — pas de melange

**Criteres d'acceptation :**
- Tous les textes de la page Login sont dans la meme langue
- Le bouton Google peut rester dans la langue du navigateur (controle par Google)

---

## Tache 6 — P3 : Accessibilite des boutons retour/edit (Detail)

**Probleme :** Les boutons retour (fleche) et edit (crayon) en haut de la page Detail sont des `<div>` sans role bouton ni label accessible. Ils sont invisibles pour les lecteurs d'ecran et n'apparaissent pas dans l'arbre d'accessibilite.

**Comportement attendu :**
- Les boutons sont des `<button>` avec un `aria-label` descriptif
- Ils sont navigables au clavier (focus visible)

**Criteres d'acceptation :**
- Les boutons retour et edit apparaissent dans l'arbre d'accessibilite
- Ils ont des labels (`aria-label="Retour"`, `aria-label="Modifier"`)

---

## Tache 7 — P4 : Indicateur visuel pour Stats "Coming soon"

**Probleme :** Le bouton Stats dans la navbar est actif et cliquable, mais la page affiche uniquement "Coming soon." — l'utilisateur n'a aucun indice avant de cliquer.

**Comportement attendu :**
- Le bouton Stats dans la navbar est legerement attenue (opacite reduite) pour indiquer que la fonctionnalite n'est pas encore disponible
- OU un petit badge "bientot" apparait sur l'icone

**Criteres d'acceptation :**
- L'utilisateur comprend visuellement que Stats n'est pas encore disponible avant de cliquer

---

## Observations notees (pas d'action immediate)

- **Onglets de filtre etroits sur mobile** : "Up to date" s'affiche sur deux lignes, "Plan" est tronque. A surveiller apres la tache 2 du plan post-audit (reordonnancement des onglets).
- **Troncature du titre "Terra e..."** sur la page Detail : le titre complet tiendrait. Mineur, pas d'action requise.
- **Barre d'actions vide pour les films sans liens** : seul "Rate" apparait. Acceptable en l'etat.

---

## Verification globale

Apres chaque tache :
1. `make test` + `make lint`
2. Verification visuelle en emulation mobile (Chrome DevTools MCP, port 5173) :
   - Les pages d'erreur s'affichent correctement (simuler un echec API)
   - La recherche retourne des resultats pertinents
   - Les titres sans couverture ont un placeholder
   - Match Review s'affiche en navigation directe
   - Login est coherent en langue
   - Boutons Detail sont accessibles
