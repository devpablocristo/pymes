#!/usr/bin/env python3
"""Execute one libpq command without placing its connection URI in argv."""

from __future__ import annotations

import os
import sys
from urllib.parse import parse_qsl, unquote, urlsplit


QUERY_ENV = {
    "application_name": "PGAPPNAME",
    "channel_binding": "PGCHANNELBINDING",
    "client_encoding": "PGCLIENTENCODING",
    "connect_timeout": "PGCONNECT_TIMEOUT",
    "gssencmode": "PGGSSENCMODE",
    "host": "PGHOST",
    "hostaddr": "PGHOSTADDR",
    "keepalives": "PGKEEPALIVES",
    "keepalives_count": "PGKEEPALIVESCOUNT",
    "keepalives_idle": "PGKEEPALIVESIDLE",
    "keepalives_interval": "PGKEEPALIVESINTERVAL",
    "load_balance_hosts": "PGLOADBALANCEHOSTS",
    "options": "PGOPTIONS",
    "passfile": "PGPASSFILE",
    "port": "PGPORT",
    "require_auth": "PGREQUIREAUTH",
    "requirepeer": "PGREQUIREPEER",
    "service": "PGSERVICE",
    "sslcert": "PGSSLCERT",
    "sslcrl": "PGSSLCRL",
    "sslcrldir": "PGSSLCRLDIR",
    "sslkey": "PGSSLKEY",
    "ssl_max_protocol_version": "PGSSLMAXPROTOCOLVERSION",
    "ssl_min_protocol_version": "PGSSLMINPROTOCOLVERSION",
    "sslmode": "PGSSLMODE",
    "sslpassword": "PGSSLPASSWORD",
    "sslrootcert": "PGSSLROOTCERT",
    "target_session_attrs": "PGTARGETSESSIONATTRS",
    "tcp_user_timeout": "PGTCPUSER_TIMEOUT",
}

CONNECTION_ENV = {
    "PGAPPNAME",
    "PGCHANNELBINDING",
    "PGCLIENTENCODING",
    "PGCONNECT_TIMEOUT",
    "PGDATABASE",
    "PGGSSENCMODE",
    "PGHOST",
    "PGHOSTADDR",
    "PGKEEPALIVES",
    "PGKEEPALIVESCOUNT",
    "PGKEEPALIVESIDLE",
    "PGKEEPALIVESINTERVAL",
    "PGLOADBALANCEHOSTS",
    "PGOPTIONS",
    "PGPASSFILE",
    "PGPASSWORD",
    "PGPORT",
    "PGREQUIREAUTH",
    "PGREQUIREPEER",
    "PGSERVICE",
    "PGSERVICEFILE",
    "PGSSLCERT",
    "PGSSLCRL",
    "PGSSLCRLDIR",
    "PGSSLKEY",
    "PGSSLMAXPROTOCOLVERSION",
    "PGSSLMINPROTOCOLVERSION",
    "PGSSLMODE",
    "PGSSLPASSWORD",
    "PGSSLROOTCERT",
    "PGTARGETSESSIONATTRS",
    "PGTCPUSER_TIMEOUT",
    "PGUSER",
}


def fail(message: str) -> "NoReturn":
    print(f"libpq connection configuration error: {message}", file=sys.stderr)
    raise SystemExit(2)


def main() -> None:
    if len(sys.argv) < 3:
        fail("expected a connection environment variable and a command")
    source_name = sys.argv[1]
    if (
        not source_name
        or not source_name.replace("_", "").isalnum()
        or not source_name.isupper()
    ):
        fail("invalid connection environment variable name")
    connection_uri = os.environ.get(source_name, "")
    if not connection_uri:
        fail(f"{source_name} is required")

    try:
        parsed = urlsplit(connection_uri)
        port = parsed.port
        hostname = parsed.hostname
    except ValueError:
        fail("invalid PostgreSQL connection URI")
    if parsed.scheme not in {"postgres", "postgresql"}:
        fail("connection URI must use postgres or postgresql")
    if parsed.fragment:
        fail("connection URI must not contain a fragment")
    if parsed.username is None:
        fail("connection URI must contain an explicit user")
    if not parsed.path.startswith("/") or len(parsed.path) < 2:
        fail("connection URI must contain an explicit database")

    child_environment = os.environ.copy()
    child_environment.pop(source_name, None)
    for key in CONNECTION_ENV:
        child_environment.pop(key, None)

    child_environment["PGUSER"] = unquote(parsed.username)
    if parsed.password is not None:
        child_environment["PGPASSWORD"] = unquote(parsed.password)
    child_environment["PGDATABASE"] = unquote(parsed.path[1:])
    if hostname is not None:
        child_environment["PGHOST"] = hostname
    if port is not None:
        child_environment["PGPORT"] = str(port)

    seen_query: set[str] = set()
    try:
        query_items = parse_qsl(
            parsed.query,
            keep_blank_values=True,
            strict_parsing=True,
        )
    except ValueError:
        fail("invalid PostgreSQL connection URI query")
    for key, value in query_items:
        if key in seen_query:
            fail(f"duplicate connection parameter: {key}")
        seen_query.add(key)
        environment_name = QUERY_ENV.get(key)
        if environment_name is None:
            fail(f"unsupported connection parameter: {key}")
        child_environment[environment_name] = value

    command = sys.argv[2:]
    try:
        os.execvpe(command[0], command, child_environment)
    except OSError:
        fail("could not execute the libpq command")


if __name__ == "__main__":
    main()
