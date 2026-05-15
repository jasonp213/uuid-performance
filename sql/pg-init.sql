-- PostgreSQL 17 init script.
-- PG 17 has no native uuidv7() generator, so DB-side UUIDv7 is not benchmarked.
-- UUIDv7 values are generated in the Go app (google/uuid v1.6+).

DROP TABLE IF EXISTS pg_serial;
DROP TABLE IF EXISTS pg_uuidv4_app;
DROP TABLE IF EXISTS pg_uuidv7_app;

CREATE TABLE pg_serial (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    amount     NUMERIC(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX pg_serial_created_at_idx ON pg_serial(created_at);

CREATE TABLE pg_uuidv4_app (
    id         UUID PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    amount     NUMERIC(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE pg_uuidv7_app (
    id         UUID PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    amount     NUMERIC(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT clock_timestamp()
);
