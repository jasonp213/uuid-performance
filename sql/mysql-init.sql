-- MySQL 8 init script.
-- UUIDs stored as BINARY(16) (production-recommended encoding).

DROP TABLE IF EXISTS mysql_autoinc;
DROP TABLE IF EXISTS mysql_autoinc_uuidv4_uk;
DROP TABLE IF EXISTS mysql_uuidv4_app;
DROP TABLE IF EXISTS mysql_uuidv7_app;

CREATE TABLE mysql_autoinc (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT        NOT NULL,
    amount     DECIMAL(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    KEY mysql_autoinc_created_at_idx (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE mysql_autoinc_uuidv4_uk (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    uuid       BINARY(16)    NOT NULL,
    user_id    BIGINT        NOT NULL,
    amount     DECIMAL(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    KEY mysql_autoinc_uuidv4_uk_created_at_idx (created_at),
    UNIQUE KEY mysql_autoinc_uuidv4_uk_uuid_idx (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE mysql_uuidv4_app (
    id         BINARY(16)    NOT NULL PRIMARY KEY,
    user_id    BIGINT        NOT NULL,
    amount     DECIMAL(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE mysql_uuidv7_app (
    id         BINARY(16)    NOT NULL PRIMARY KEY,
    user_id    BIGINT        NOT NULL,
    amount     DECIMAL(12,2) NOT NULL,
    status     VARCHAR(20)   NOT NULL,
    created_at TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
