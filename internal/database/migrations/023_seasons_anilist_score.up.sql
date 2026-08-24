-- Score communautaire AniList persisté par saison, rafraîchi quotidiennement
-- pour alimenter le bandeau d'information du concept B sur le détail de titre.
ALTER TABLE seasons ADD COLUMN anilist_average_score INTEGER NULL;
