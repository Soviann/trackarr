ALTER TABLE seasons ADD COLUMN my_rating INTEGER
    CHECK(my_rating IS NULL OR (my_rating >= 1 AND my_rating <= 10));
