#!/bin/sh
# Runs once on first cluster init. Integration tests use their own database
# (and create throwaway databases from it) so they never touch development data.
set -e
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
	CREATE DATABASE app_test OWNER $POSTGRES_USER;
SQL
