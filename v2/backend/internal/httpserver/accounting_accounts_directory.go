package httpserver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/google/uuid"
)

type accountingAccountDirectoryRow struct {
	detail  accounting.AccountDetail
	path    []uuid.UUID
	sortKey string
}

func accountingAccountDirectoryRows(
	details []accounting.AccountDetail,
) ([]accountingAccountDirectoryRow, error) {
	result := make([]accountingAccountDirectoryRow, 0, len(details))
	for _, detail := range details {
		result = append(result, accountingAccountDirectoryRow{
			detail:  detail,
			sortKey: strings.ToLower(detail.Account.Code),
		})
	}
	if err := populateAccountingAccountPaths(result); err != nil {
		return nil, err
	}
	return result, nil
}

func populateAccountingAccountPaths(
	items []accountingAccountDirectoryRow,
) error {
	byID := make(map[uuid.UUID]int, len(items))
	for index := range items {
		byID[items[index].detail.Account.ID] = index
	}

	var resolve func(int, map[uuid.UUID]struct{}) ([]uuid.UUID, error)
	resolve = func(
		index int,
		visiting map[uuid.UUID]struct{},
	) ([]uuid.UUID, error) {
		if len(items[index].path) > 0 {
			return items[index].path, nil
		}
		id := items[index].detail.Account.ID
		if _, found := visiting[id]; found {
			return nil, errAccountingAccountHierarchyCycle
		}
		visiting[id] = struct{}{}
		defer delete(visiting, id)

		parentID := items[index].detail.Account.ParentID
		if parentID == nil {
			items[index].path = []uuid.UUID{id}
			return items[index].path, nil
		}
		parentIndex, found := byID[*parentID]
		if !found {
			return nil, errAccountingAccountParentInvalid
		}
		parentPath, err := resolve(parentIndex, visiting)
		if err != nil {
			return nil, err
		}
		items[index].path = append(
			append(make([]uuid.UUID, 0, len(parentPath)+1), parentPath...),
			id,
		)
		return items[index].path, nil
	}

	for index := range items {
		if _, err := resolve(index, make(map[uuid.UUID]struct{})); err != nil {
			return err
		}
	}
	return nil
}

func accountDirectorySummary(
	item accountingAccountDirectoryRow,
	contextOnly bool,
) api.AccountingAccountSummary {
	account := item.detail.Account
	return api.AccountingAccountSummary{
		AccountType:            apiAccountTypeFromDB(string(account.Class)),
		Capabilities:           apiAccountCapabilities(item.detail.Capabilities),
		Code:                   account.Code,
		ContextOnly:            contextOnly,
		Depth:                  len(item.path) - 1,
		HasChildren:            item.detail.Usage.Children > 0,
		Id:                     account.ID,
		LifecycleState:         api.LifecycleState(account.LifecycleState()),
		Mapped:                 item.detail.Usage.Mappings > 0,
		MonetaryClassification: api.MonetaryClassification(account.Monetary),
		Name:                   account.Name,
		NodeType:               api.AccountingAccountNodeType(account.EffectiveNodeType()),
		NormalBalance:          api.AccountingNormalBalance(account.NormalBalance),
		ParentId:               account.ParentID,
		Path:                   append([]uuid.UUID(nil), item.path...),
		Postable:               account.Postable,
		SystemManaged:          account.SystemManaged,
		Used:                   item.detail.Usage.HasDependencies(),
		Version:                account.Version,
	}
}

func accountDirectoryDetail(
	item accountingAccountDirectoryRow,
) api.AccountingAccountDetail {
	summary := accountDirectorySummary(item, false)
	return api.AccountingAccountDetail{
		AccountType:            summary.AccountType,
		Capabilities:           summary.Capabilities,
		Code:                   summary.Code,
		ContextOnly:            false,
		Depth:                  summary.Depth,
		HasChildren:            summary.HasChildren,
		Id:                     summary.Id,
		LifecycleState:         summary.LifecycleState,
		Mapped:                 summary.Mapped,
		MappedRoles:            nonNilStrings(item.detail.MappingRoles),
		MonetaryClassification: summary.MonetaryClassification,
		Name:                   summary.Name,
		NodeType:               summary.NodeType,
		NormalBalance:          summary.NormalBalance,
		ParentId:               summary.ParentId,
		Path:                   summary.Path,
		Postable:               summary.Postable,
		SystemManaged:          summary.SystemManaged,
		Usage:                  apiAccountUsage(item.detail.Usage),
		Used:                   summary.Used,
		Version:                summary.Version,
	}
}

func apiAccountCapabilities(
	value accounting.AccountCapabilities,
) api.AccountingAccountCapabilities {
	return api.AccountingAccountCapabilities{
		ArchiveBlockers:  nonNilStrings(value.ArchiveBlockers),
		CanArchive:       value.CanArchive,
		CanDuplicate:     value.CanDuplicate,
		CanEditName:      value.CanEditName,
		CanEditStructure: value.CanEditStructure,
		CanRestore:       value.CanRestore,
		CanTrash:         value.CanTrash,
		EditBlockers:     nonNilStrings(value.EditBlockers),
		RestoreBlockers:  nonNilStrings(value.RestoreBlockers),
		TrashBlockers:    nonNilStrings(value.TrashBlockers),
	}
}

func apiAccountUsage(value accounting.AccountUsage) api.AccountingAccountUsage {
	total := value.JournalLines +
		value.DraftLines +
		value.Mappings +
		value.Children +
		value.FinancialAccounts +
		value.OpenItems +
		value.InflationLines +
		value.RevaluationLines
	return api.AccountingAccountUsage{
		ActiveChildren:          int(value.ActiveChildren),
		ActiveFinancialAccounts: int(value.ActiveFinancialAccounts),
		Children:                int(value.Children),
		DraftLines:              int(value.DraftLines),
		FinancialAccounts:       int(value.FinancialAccounts),
		InflationLines:          int(value.InflationLines),
		JournalLines:            int(value.JournalLines),
		Mappings:                int(value.Mappings),
		OpenItems:               int(value.OpenItems),
		RevaluationLines:        int(value.RevaluationLines),
		TotalDependencies:       int(total),
		Used:                    value.HasDependencies(),
	}
}

func apiAccountMappingDefinition(
	value accounting.AccountMappingDefinition,
) api.AccountingMappingDefinition {
	accountTypes := make(
		[]api.AccountingAccountType,
		0,
		len(value.CompatibleAccountClasses),
	)
	for _, accountClass := range value.CompatibleAccountClasses {
		accountTypes = append(
			accountTypes,
			apiAccountTypeFromDB(string(accountClass)),
		)
	}
	normalBalances := make(
		[]api.AccountingNormalBalance,
		0,
		len(value.CompatibleNormalBalances),
	)
	for _, normal := range value.CompatibleNormalBalances {
		normalBalances = append(
			normalBalances,
			api.AccountingNormalBalance(normal),
		)
	}
	monetaryClasses := make(
		[]api.MonetaryClassification,
		0,
		len(value.CompatibleMonetaryClasses),
	)
	for _, monetary := range value.CompatibleMonetaryClasses {
		monetaryClasses = append(
			monetaryClasses,
			api.MonetaryClassification(monetary),
		)
	}
	var canonicalRole *string
	if strings.TrimSpace(value.CanonicalRole) != "" {
		canonical := value.CanonicalRole
		canonicalRole = &canonical
	}
	return api.AccountingMappingDefinition{
		CanonicalRole:                     canonicalRole,
		CompatibleAccountTypes:            accountTypes,
		CompatibleMonetaryClassifications: monetaryClasses,
		CompatibleNormalBalances:          normalBalances,
		DescriptionEn:                     value.DescriptionEN,
		DescriptionEs:                     value.DescriptionES,
		DisplayOrder:                      value.DisplayOrder,
		IsAlias:                           value.Alias,
		LabelEn:                           value.LabelEN,
		LabelEs:                           value.LabelES,
		Required:                          value.Required,
		Role:                              value.Role,
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return make([]string, 0)
	}
	return values
}

type accountDirectoryFilter struct {
	lifecycle   api.LifecycleState
	query       string
	nodeType    *api.AccountingAccountNodeType
	accountType *api.AccountingAccountType
	parentID    *uuid.UUID
	used        *bool
}

func validateAccountDirectoryFilter(
	lifecycle *api.LifecycleState,
	query *string,
	nodeType *api.AccountingAccountNodeType,
	accountType *api.AccountingAccountType,
	parentID *uuid.UUID,
	used *bool,
	postable *bool,
) (accountDirectoryFilter, error) {
	result := accountDirectoryFilter{
		lifecycle:   api.LifecycleStateActive,
		nodeType:    nodeType,
		accountType: accountType,
		parentID:    parentID,
		used:        used,
	}
	if lifecycle != nil {
		if !lifecycle.Valid() {
			return accountDirectoryFilter{}, fmt.Errorf(
				"%w: invalid lifecycle state",
				errBusinessInvalidRequest,
			)
		}
		result.lifecycle = *lifecycle
	}
	if query != nil {
		result.query = strings.TrimSpace(*query)
	}
	if nodeType != nil && !nodeType.Valid() {
		return accountDirectoryFilter{}, fmt.Errorf(
			"%w: invalid account node type",
			errBusinessInvalidRequest,
		)
	}
	if accountType != nil && !accountType.Valid() {
		return accountDirectoryFilter{}, fmt.Errorf(
			"%w: invalid account type",
			errBusinessInvalidRequest,
		)
	}
	if postable != nil {
		legacyNodeType := api.Group
		if *postable {
			legacyNodeType = api.Posting
		}
		if result.nodeType != nil && *result.nodeType != legacyNodeType {
			return accountDirectoryFilter{}, fmt.Errorf(
				"%w: node_type and postable filters disagree",
				errBusinessInvalidRequest,
			)
		}
		result.nodeType = &legacyNodeType
	}
	return result, nil
}

func (filter accountDirectoryFilter) matches(
	item accountingAccountDirectoryRow,
	includeLifecycle bool,
) bool {
	account := item.detail.Account
	if includeLifecycle &&
		api.LifecycleState(account.LifecycleState()) != filter.lifecycle {
		return false
	}
	if filter.query != "" {
		query := strings.ToLower(filter.query)
		if !strings.Contains(strings.ToLower(account.Code), query) &&
			!strings.Contains(strings.ToLower(account.Name), query) {
			return false
		}
	}
	if filter.nodeType != nil &&
		api.AccountingAccountNodeType(account.EffectiveNodeType()) != *filter.nodeType {
		return false
	}
	if filter.accountType != nil &&
		apiAccountTypeFromDB(string(account.Class)) != *filter.accountType {
		return false
	}
	if filter.parentID != nil &&
		(account.ParentID == nil || *account.ParentID != *filter.parentID) {
		return false
	}
	if filter.used != nil &&
		item.detail.Usage.HasDependencies() != *filter.used {
		return false
	}
	return true
}

func sortAccountingDirectoryRows(items []accountingAccountDirectoryRow) {
	sort.SliceStable(items, func(left, right int) bool {
		return accountingNaturalCodeLess(
			items[left].detail.Account.Code,
			items[left].detail.Account.ID,
			items[right].detail.Account.Code,
			items[right].detail.Account.ID,
		)
	})
}

func accountingNaturalCodeLess(
	leftCode string,
	leftID uuid.UUID,
	rightCode string,
	rightID uuid.UUID,
) bool {
	leftParts := strings.Split(strings.ToLower(leftCode), ".")
	rightParts := strings.Split(strings.ToLower(rightCode), ".")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		left, right := leftParts[index], rightParts[index]
		if left == right {
			continue
		}
		leftNumber, leftOK := normalizeNaturalInteger(left)
		rightNumber, rightOK := normalizeNaturalInteger(right)
		if leftOK && rightOK {
			if len(leftNumber) != len(rightNumber) {
				return len(leftNumber) < len(rightNumber)
			}
			if leftNumber != rightNumber {
				return leftNumber < rightNumber
			}
			return left < right
		}
		return left < right
	}
	if len(leftParts) != len(rightParts) {
		return len(leftParts) < len(rightParts)
	}
	return leftID.String() < rightID.String()
}

func normalizeNaturalInteger(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", false
		}
	}
	normalized := strings.TrimLeft(value, "0")
	if normalized == "" {
		normalized = "0"
	}
	return normalized, true
}

func preorderAccountingDirectory(
	items []accountingAccountDirectoryRow,
	included map[uuid.UUID]struct{},
) ([]accountingAccountDirectoryRow, error) {
	children := make(map[uuid.UUID][]accountingAccountDirectoryRow)
	roots := make([]accountingAccountDirectoryRow, 0)
	for _, item := range items {
		id := item.detail.Account.ID
		if _, found := included[id]; !found {
			continue
		}
		parentID := item.detail.Account.ParentID
		if parentID == nil {
			roots = append(roots, item)
			continue
		}
		if _, parentIncluded := included[*parentID]; !parentIncluded {
			roots = append(roots, item)
			continue
		}
		children[*parentID] = append(children[*parentID], item)
	}
	sortAccountingDirectoryRows(roots)
	for parentID := range children {
		sortAccountingDirectoryRows(children[parentID])
	}

	result := make([]accountingAccountDirectoryRow, 0, len(included))
	visiting := make(map[uuid.UUID]bool, len(included))
	visited := make(map[uuid.UUID]bool, len(included))
	var visit func(accountingAccountDirectoryRow) error
	visit = func(item accountingAccountDirectoryRow) error {
		id := item.detail.Account.ID
		if visiting[id] {
			return errAccountingAccountHierarchyCycle
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		result = append(result, item)
		for _, child := range children[id] {
			if err := visit(child); err != nil {
				return err
			}
		}
		delete(visiting, id)
		visited[id] = true
		return nil
	}
	for _, root := range roots {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	if len(result) != len(included) {
		return nil, errAccountingAccountHierarchyCycle
	}
	return result, nil
}
