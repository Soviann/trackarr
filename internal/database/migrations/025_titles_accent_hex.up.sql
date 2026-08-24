-- Couleur dominante extraite du cover, utilisée pour teinter la fiche titre.
-- NULL = jamais extraite ou cover absente.
ALTER TABLE titles ADD COLUMN accent_hex TEXT NULL;
