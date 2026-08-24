-- Store the FR "flatrate" (subscription-included) streaming providers per title,
-- as a JSON array of {id, name}. NULL = never fetched; '[]' = fetched, none in FR.
ALTER TABLE titles ADD COLUMN watch_providers TEXT;
