---
name: no-ddev-poweroff
enabled: true
event: bash
pattern: ddev\s+poweroff
action: block
---

**ddev poweroff interdit**

`ddev poweroff` arrête TOUS les projets DDEV, pas seulement celui-ci.
Ne jamais exécuter cette commande sans permission explicite de l'utilisateur.
