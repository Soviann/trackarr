package repository

// displayNameExpr is a SQL expression that resolves the display name for a title
// aliased as `t` in the enclosing query. Priority: fr → en → (x-romaji → ja when
// anime) → any. Callers interpolate it as a scalar column, e.g.
// `SELECT `+displayNameExpr+` AS name`.
//
// Implementation: SQLite scalar correlated subqueries don't allow referencing
// outer columns from ORDER BY CASE, so the outer `t.is_anime` is first pulled
// in via the inner SELECT clause (where correlation is allowed), then reused
// in the outer ORDER BY.
const displayNameExpr = `(
    SELECT name FROM (
        SELECT tn.id AS tid, tn.name, tn.language, t.is_anime AS an
        FROM title_names tn WHERE tn.title_id = t.id
    )
    ORDER BY
        CASE
            WHEN language = 'fr' THEN 1
            WHEN language = 'en' THEN 2
            WHEN an = 1 AND language = 'x-romaji' THEN 3
            WHEN an = 1 AND language = 'ja' THEN 4
            ELSE 5
        END,
        tid
    LIMIT 1
)`
