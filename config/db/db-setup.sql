-- postgres docker db creation is done via environment variable:
-- https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh#L169-L178

CREATE TABLE IF NOT EXISTS info (
    id varchar(64),
    expiry bigint,  -- unix epoch timestamp for periodical db cleanup
    body text
);

-- try building the index with no locks to avoid performance issue
CREATE INDEX CONCURRENTLY expiration ON info USING btree (
    expiry ASC
);
