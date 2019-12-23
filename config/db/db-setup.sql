-- postgres docker db creation is done via environment variable:
-- https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh#L169-L178

CREATE TABLE IF NOT EXISTS info (
    id varchar(64),
    body text
)
