FROM postgres:alpine

# persist environment variables for DB setup
ARG DB_NAME 
ENV POSTGRES_DB DB_NAME
ARG DB_USER 
ENV POSTGRES_USER DB_USER
ARG DB_PASSWD
ENV POSTGRES_PASSWORD DB_PASSWD

# setup db infra during image build time
# https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh#L255-L276
# https://stackoverflow.com/a/38714511
ADD ./config/db/* /docker-entrypoint-initdb.d/
