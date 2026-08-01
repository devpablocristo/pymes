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
public_go_operation_ids='getHealthz,getReadyz,createSale,createParty,getParty,createPurchase,getPurchase,createPayment,getPayment,createAccountingReversal,getAccountingFailure,createAccountingAdjustment,getSale,requestFiscalCredentialCSR,getFiscalCredential,uploadFiscalCertificate,configureFiscalPointOfSale,validateFiscalPointOfSale'
go run "$generator" --include-operation-ids "$public_go_operation_ids" --package publicapi --generate types,chi-server -o "$temporary_dir/public.gen.go" ../api/openapi.yaml
# Resolve and generate every public operation as a contract-integrity gate.
# The output is intentionally ephemeral: Go handlers remain context-local,
# while the web client is generated independently from the same canonical API.
go run "$generator" --package publiccontract --generate types,client -o "$temporary_dir/public.contract.go" ../api/openapi.yaml
go run "$generator" --package accountingapi --generate types,client -o "$temporary_dir/accounting.gen.go" ../contracts/accounting.openapi.yaml
go run "$generator" --package accountingapi --generate chi-server -o "$temporary_dir/accounting.server.gen.go" ../contracts/accounting.openapi.yaml
go run "$generator" --package fiscalapi --generate types,client -o "$temporary_dir/fiscal.gen.go" ../contracts/fiscal.openapi.yaml
go run "$generator" --package fiscalapi --generate chi-server -o "$temporary_dir/fiscal.server.gen.go" ../contracts/fiscal.openapi.yaml
cmp "$temporary_dir/public.gen.go" internal/commerce/handler/dto/public.gen.go
cmp "$temporary_dir/accounting.gen.go" internal/commerce/accounting/models/accounting.gen.go
cmp "$temporary_dir/accounting.server.gen.go" internal/commerce/accounting/models/accounting.server.gen.go
cmp "$temporary_dir/fiscal.gen.go" internal/commerce/fiscal/models/fiscal.gen.go
cmp "$temporary_dir/fiscal.server.gen.go" internal/commerce/fiscal/models/fiscal.server.gen.go
