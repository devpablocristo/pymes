// Package fakeservice provides contract-faithful private service mocks for E2E.
// architecture:adapter handler
package fakeservice

import (
	"net/http"

	accountingapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/accounting/models"
	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/handler/helpers"
)

func HandlerForKind(kind string) (http.Handler, error) {
	parsed, err := handlerhelpers.ParseKind(kind)
	if err != nil {
		return nil, err
	}
	switch parsed {
	case "accounting":
		return accountingapi.Handler(newAccountingFakeServer()), nil
	case "fiscal":
		return fiscalapi.Handler(newFiscalFakeServer()), nil
	}
	panic("unreachable fake service kind")
}
