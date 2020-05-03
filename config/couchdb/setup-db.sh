#!/bin/sh

set -e

# setup system database
CURL_ADMIN="curl --silent --show-error -u ${COUCHDB_USER}:${COUCHDB_PASSWORD}"
# TODO: given this is run in a one-off container, change this when setting up in production environment to avoid transmitting creds over plain-text http
DB_ADDR="http://pin-db:5984"

echo "Waiting for CouchDB to stand up"
sleep 3

# ensure following before proceeding:
# 1. db is up 
# 2. application setup params are present
echo "Checking if CouchDB is up"
$CURL_ADMIN $DB_ADDR/_up | grep -q 'ok' 
if [ $? -ne 0 ]; then
  echo "CouchDB is yet up" >&2
  exit 1
fi

echo "Checking if application setup param is present"
if [ -z "$COUCHDB_USER_APP" -o -z "$COUCHDB_PASSWORD_APP" -o -z "$DB_NAME_PIN" -o -z "$DB_NAME_PIN_META" -o -z "$DB_NAME_USER" ]; then
  echo "Application setup parameters cannot be blank" >&2
  exit 1
fi

echo "Creating system databases"
for dbname in "_users" "_replicator"
do
  $CURL_ADMIN -X PUT $DB_ADDR/$dbname
done

# setup application user
echo "Creating application user"

DATA_PUT_USER="{\"name\": \"$COUCHDB_USER_APP\", \"password\": \"$COUCHDB_PASSWORD_APP\", \"roles\": [], \"type\": \"user\"}"
$CURL_ADMIN -X PUT $DB_ADDR/_users/org.couchdb.user:${COUCHDB_USER_APP} \
     -H "Accept: application/json" \
     -H "Content-Type: application/json" \
     -d "$DATA_PUT_USER"

# setup application databases and grant application user permissions
echo "Creating application databases and populating application user's permissions"

SEC_DOC="{\"admins\": { \"names\": [], \"roles\": [] }, \"members\": { \"names\": [\"$COUCHDB_USER_APP\"], \"roles\": [] } }"
for dbname in $DB_NAME_PIN $DB_NAME_PIN_META $DB_NAME_PIN_USER
do
  $CURL_ADMIN -X PUT $DB_ADDR/$dbname
  $CURL_ADMIN -X PUT $DB_ADDR/$dbname/_security -H "Content-Type: application/json" -d "$SEC_DOC"
done
