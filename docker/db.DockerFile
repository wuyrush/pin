FROM postgres:alpine
# NOTE: NEVER persist the db username and password as env vars during image build time, since an attacker can easily dump the
# content of intermediate image layers and find out such info
# https://docs.docker.com/develop/develop-images/dockerfile_best-practices/#env
# Instead, pass them as env var when starting up the db container

# setup application's db layer during image build time
# https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh#L255-L276
# https://stackoverflow.com/a/38714511
ADD ./config/db/* /docker-entrypoint-initdb.d/
