-- Primary country of origin per title as ISO-3166-1 alpha-2 (e.g. 'KR', 'JP').
-- NULL = origin never determined (title matched before this field, or source
-- exposed no country). Populated by the metadata refresh; backfilled via
-- POST /admin/refresh-all.
ALTER TABLE titles ADD COLUMN origin_country TEXT;
