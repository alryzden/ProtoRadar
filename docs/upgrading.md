# Upgrading

## Community v1.0

ProtoRadar v1.0.0 is the first stable Community release target. It establishes the baseline for the public REST API, CLI behavior, Docker Compose quickstart, migrations, observability, examples, and documentation.

## Migration Policy From v1.0 Onward

From v1.0 onward, database migrations are expected to be forward-compatible. The server applies migrations at startup from the packaged `migrations/` directory.

Recommended upgrade process:

1. Back up PostgreSQL.
2. Back up S3/MinIO artifact storage.
3. Review release notes and config changes.
4. Deploy the new image.
5. Watch startup logs for migration errors.
6. Check `/readyz` and `/metrics`.
7. Run `make smoke-test` or equivalent environment smoke checks where appropriate.

## Pre-v1.0 Schemas

Pre-v1.0 schemas were developed during rapid feature phases. If you ran an earlier development build, a fresh install may be the simplest path. Manual migration may be required for pre-v1.0 databases depending on which unmerged migration state was used.

For local demos, use:

```sh
docker compose down -v
docker compose up --build
```

This removes local PostgreSQL and MinIO volumes.

## Config Compatibility

Keep Go 1.26 as the project baseline for building from source. Do not downgrade Dockerfiles, CI, or local tooling to older Go versions.

New config fields should be added in `internal/config` with defaults, env/YAML binding, validation, and runtime mapping.
