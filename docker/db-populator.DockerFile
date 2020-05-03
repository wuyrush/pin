FROM alpine:latest

RUN apk add curl

COPY ./config/couchdb/setup-db.sh /

ENTRYPOINT ["/setup-db.sh"]
