#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
temporary_dir=$(mktemp -d)
cleanup() {
  rm -rf "$temporary_dir"
}
trap cleanup EXIT INT TERM

cd "$root_dir/backend"
generator='github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0'
go run "$generator" --package publicapi --generate types,chi-server -o "$temporary_dir/public.gen.go" ../api/openapi.yaml
go run "$generator" --package accountingapi --generate types,client -o "$temporary_dir/accounting.gen.go" ../contracts/accounting.openapi.yaml
go run "$generator" --package fiscalapi --generate types,client -o "$temporary_dir/fiscal.gen.go" ../contracts/fiscal.openapi.yaml
cmp "$temporary_dir/public.gen.go" internal/contracts/publicapi/public.gen.go
cmp "$temporary_dir/accounting.gen.go" internal/contracts/accountingapi/accounting.gen.go
cmp "$temporary_dir/fiscal.gen.go" internal/contracts/fiscalapi/fiscal.gen.go
