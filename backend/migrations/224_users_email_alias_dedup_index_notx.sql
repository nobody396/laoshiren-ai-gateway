-- Provider-aware mailbox identity constraint. Production preflight must prove
-- there are no historical duplicates before this migration is released.
-- Runtime transaction advisory locks provide a deterministic application error;
-- this unique index remains the final cross-instance/write-path invariant.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_alias_dedup
    ON users (((
        CASE
            WHEN rtrim(split_part(lower(btrim(email)), '@', 2), '.') IN ('gmail.com', 'googlemail.com') THEN
                coalesce(
                    nullif(
                        replace(
                            CASE
                                WHEN strpos(split_part(lower(btrim(email)), '@', 1), '+') > 1 THEN
                                    left(
                                        split_part(lower(btrim(email)), '@', 1),
                                        strpos(split_part(lower(btrim(email)), '@', 1), '+') - 1
                                    )
                                ELSE split_part(lower(btrim(email)), '@', 1)
                            END,
                            '.',
                            ''
                        ),
                        ''
                    ),
                    CASE
                        WHEN strpos(split_part(lower(btrim(email)), '@', 1), '+') > 1 THEN
                            left(
                                split_part(lower(btrim(email)), '@', 1),
                                strpos(split_part(lower(btrim(email)), '@', 1), '+') - 1
                            )
                        ELSE split_part(lower(btrim(email)), '@', 1)
                    END
                ) || '@gmail.com'
            ELSE
                split_part(lower(btrim(email)), '@', 1) || '@' ||
                rtrim(split_part(lower(btrim(email)), '@', 2), '.')
        END
    )))
    WHERE deleted_at IS NULL;
