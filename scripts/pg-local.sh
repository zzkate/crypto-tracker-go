#!/usr/bin/env bash
#
# Локальный PostgreSQL без sudo и Docker.
# Бинарники распакованы из .deb в ~/.local/pg, данные в ~/.local/pgdata.
#
#   scripts/pg-local.sh start     # поднять (initdb + роль/БД при первом запуске)
#   scripts/pg-local.sh stop      # остановить
#   scripts/pg-local.sh status    # состояние
#   scripts/pg-local.sh psql      # консоль к БД crypto
set -euo pipefail

PGBIN="${PGBIN:-$HOME/.local/pg/usr/lib/postgresql/18/bin}"
DATADIR="${PGDATA_LOCAL:-$HOME/.local/pgdata}"
PORT="${PGPORT:-5432}"
DB="${PGDB:-crypto}"
USER="${PGUSER:-crypto}"
PASSWORD="${PGPASSWORD:-crypto}"

cmd="${1:-status}"

case "$cmd" in
  start)
    if "$PGBIN/pg_ctl" -D "$DATADIR" status >/dev/null 2>&1; then
      echo "PostgreSQL уже запущен (порт $PORT)"
      exit 0
    fi
    if [ ! -f "$DATADIR/PG_VERSION" ]; then
      mkdir -p "$DATADIR"
      "$PGBIN/initdb" -D "$DATADIR" -U postgres \
        --auth-local=trust --auth-host=scram-sha-256 --encoding=UTF8 --locale=C >/dev/null
      echo "initdb готов"
    fi
    "$PGBIN/pg_ctl" -D "$DATADIR" -l "$DATADIR/logfile" \
      -o "-p $PORT -k /tmp -c listen_addresses=127.0.0.1" start
    if ! "$PGBIN/psql" -h /tmp -p "$PORT" -U postgres -d postgres -tAc \
        "SELECT 1 FROM pg_roles WHERE rolname='$USER'" | grep -q 1; then
      "$PGBIN/psql" -h /tmp -p "$PORT" -U postgres -d postgres \
        -c "CREATE ROLE $USER LOGIN PASSWORD '$PASSWORD';" >/dev/null
      "$PGBIN/psql" -h /tmp -p "$PORT" -U postgres -d postgres \
        -c "CREATE DATABASE $DB OWNER $USER;" >/dev/null
      echo "роль/БД $DB созданы"
    fi
    echo "PostgreSQL на 127.0.0.1:$PORT (БД $DB, роль $USER)"
    echo "export TEST_DATABASE_URL=postgres://$USER:$PASSWORD@127.0.0.1:$PORT/$DB?sslmode=disable"
    ;;
  stop)
    "$PGBIN/pg_ctl" -D "$DATADIR" stop -m fast
    ;;
  status)
    "$PGBIN/pg_ctl" -D "$DATADIR" status
    ;;
  psql)
    shift
    exec "$PGBIN/psql" "postgres://$USER:$PASSWORD@127.0.0.1:$PORT/$DB?sslmode=disable" "$@"
    ;;
  *)
    echo "usage: $0 {start|stop|status|psql}" >&2
    exit 2
    ;;
esac
