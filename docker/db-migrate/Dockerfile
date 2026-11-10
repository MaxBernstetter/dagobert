# syntax=docker/dockerfile:1
# One-off migration runner: Atlas CLI plus db/ (atlas.hcl, schema, migrations). Build context is the repository root:
#   docker build -f docker/db-migrate/Dockerfile .
# postgres:16-alpine supplies psql for the role guard in entrypoint.sh (same major version as Aurora 16).
FROM postgres:16-alpine
ARG TARGETARCH=amd64
ARG ATLAS_VERSION=v1.3.0
RUN apk add --no-cache ca-certificates \
 && wget -qO /usr/local/bin/atlas "https://release.ariga.io/atlas/atlas-linux-${TARGETARCH}-${ATLAS_VERSION}" \
 && chmod 0755 /usr/local/bin/atlas

WORKDIR /db
COPY db/atlas.hcl ./atlas.hcl
COPY db/schema ./schema
COPY db/migrations ./migrations
COPY db/roles.sql ./roles.sql

COPY docker/db-migrate/entrypoint.sh /usr/local/bin/db-migrate
RUN chmod 0755 /usr/local/bin/db-migrate

ENTRYPOINT ["/usr/local/bin/db-migrate"]
