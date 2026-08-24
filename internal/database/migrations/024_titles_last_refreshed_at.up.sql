-- Trace de la dernière synchronisation méta réussie (TMDB / TVDB / AniList)
-- pour qu'un utilisateur puisse jauger sur la fiche s'il a une chance que
-- de nouveaux épisodes soient déjà visibles. NULL = jamais rafraîchi.
ALTER TABLE titles ADD COLUMN last_refreshed_at DATETIME NULL;
