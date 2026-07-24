package accounting

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type ChartTemplate struct {
	Code               string                 `json:"code"`
	Version            int                    `json:"version"`
	CountryCode        string                 `json:"country_code"`
	FunctionalCurrency Currency               `json:"functional_currency"`
	Name               string                 `json:"name"`
	Source             string                 `json:"source"`
	SourceChecksum     string                 `json:"source_checksum"`
	Accounts           []ChartTemplateAccount `json:"accounts"`
	Mappings           []ChartTemplateMapping `json:"mappings"`
}

type ChartTemplateAccount struct {
	Code          string                 `json:"code"`
	Name          string                 `json:"name"`
	Class         AccountClass           `json:"class"`
	ParentCode    string                 `json:"parent_code,omitempty"`
	NormalBalance NormalBalance          `json:"normal_balance"`
	Monetary      MonetaryClassification `json:"monetary_classification"`
	Postable      bool                   `json:"postable"`
	DisplayOrder  int                    `json:"display_order"`
}

type ChartTemplateMapping struct {
	Role        string `json:"role"`
	AccountCode string `json:"account_code"`
	Description string `json:"description"`
}

func (template ChartTemplate) Validate() error {
	if strings.TrimSpace(template.Code) == "" || template.Version <= 0 ||
		len(template.CountryCode) != 2 || strings.TrimSpace(template.Name) == "" ||
		strings.TrimSpace(template.Source) == "" {
		return fmt.Errorf("%w: invalid chart template metadata", ErrInvalidArgument)
	}
	accounts := make(map[string]ChartTemplateAccount, len(template.Accounts))
	for _, account := range template.Accounts {
		if strings.TrimSpace(account.Code) == "" || strings.TrimSpace(account.Name) == "" ||
			!account.Class.Valid() || !account.NormalBalance.Valid() || !account.Monetary.Valid() {
			return fmt.Errorf("%w: invalid template account %q", ErrInvalidArgument, account.Code)
		}
		if _, duplicate := accounts[account.Code]; duplicate {
			return fmt.Errorf("%w: duplicate template account %q", ErrDuplicate, account.Code)
		}
		if account.Postable && account.Monetary == NotApplicable {
			return fmt.Errorf("%w: postable template account %q needs monetary classification", ErrInvalidArgument, account.Code)
		}
		accounts[account.Code] = account
	}
	for _, account := range template.Accounts {
		if account.ParentCode == "" {
			continue
		}
		parent, ok := accounts[account.ParentCode]
		if !ok || parent.Postable || parent.Class != account.Class {
			return fmt.Errorf("%w: invalid parent of template account %q", ErrInvalidArgument, account.Code)
		}
	}
	roles := make(map[string]struct{}, len(template.Mappings))
	for _, mapping := range template.Mappings {
		if _, ok := accounts[mapping.AccountCode]; !ok {
			return fmt.Errorf("%w: mapping %q references unknown account", ErrInvalidArgument, mapping.Role)
		}
		if _, duplicate := roles[mapping.Role]; duplicate {
			return fmt.Errorf("%w: duplicate mapping %q", ErrDuplicate, mapping.Role)
		}
		roles[mapping.Role] = struct{}{}
	}
	return nil
}

func (template ChartTemplate) CanonicalChecksum() string {
	accounts := append([]ChartTemplateAccount(nil), template.Accounts...)
	mappings := append([]ChartTemplateMapping(nil), template.Mappings...)
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Code < accounts[j].Code })
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Role < mappings[j].Role })
	var canonical strings.Builder
	fmt.Fprintf(
		&canonical,
		"%s|%d|%s|%s|%s|%s\n",
		template.Code,
		template.Version,
		template.CountryCode,
		template.FunctionalCurrency.Code(),
		template.Name,
		template.Source,
	)
	for _, account := range accounts {
		fmt.Fprintf(
			&canonical,
			"A|%s|%s|%s|%s|%s|%s|%t|%d\n",
			account.Code,
			account.Name,
			account.Class,
			account.ParentCode,
			account.NormalBalance,
			account.Monetary,
			account.Postable,
			account.DisplayOrder,
		)
	}
	for _, mapping := range mappings {
		fmt.Fprintf(&canonical, "M|%s|%s|%s\n", mapping.Role, mapping.AccountCode, mapping.Description)
	}
	hash := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(hash[:])
}
