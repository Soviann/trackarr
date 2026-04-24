-- Requiert SQLite >= 3.35 (l'image Docker expose 3.42+).
-- La note par saison n'est plus exposée nulle part : une seule note par titre.
ALTER TABLE seasons DROP COLUMN my_rating;
