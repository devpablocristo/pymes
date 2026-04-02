package dataio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ImportCustomers(ctx context.Context, orgID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error) {
	result := ImportResult{TotalRows: len(rows)}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for idx, row := range rows {
			existingID, err := r.findPartyByRole(ctx, tx, orgID, "customer", row["email"], row["tax_id"])
			if err != nil {
				return err
			}
			if existingID != nil {
				if mode == "create_only" {
					result.Skipped++
					continue
				}
				if err := upsertCustomerParty(ctx, tx, orgID, *existingID, row); err != nil {
					result.Errors = append(result.Errors, ImportError{Row: idx + 2, Message: err.Error()})
					continue
				}
				if phone := strings.TrimSpace(row["phone"]); phone != "" {
					result.PartyPhones = append(result.PartyPhones, PartyPhone{PartyID: *existingID, Phone: phone})
				}
				result.Updated++
				continue
			}
			partyID, err := createCustomerParty(ctx, tx, orgID, row)
			if err != nil {
				result.Errors = append(result.Errors, ImportError{Row: idx + 2, Message: err.Error()})
				continue
			}
			if phone := strings.TrimSpace(row["phone"]); phone != "" {
				result.PartyPhones = append(result.PartyPhones, PartyPhone{PartyID: partyID, Phone: phone})
			}
			result.Created++
		}
		return nil
	})
	return result, err
}

func (r *Repository) ImportSuppliers(ctx context.Context, orgID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error) {
	result := ImportResult{TotalRows: len(rows)}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for idx, row := range rows {
			existingID, err := r.findPartyByRole(ctx, tx, orgID, "supplier", row["email"], row["tax_id"])
			if err != nil {
				return err
			}
			if existingID != nil {
				if mode == "create_only" {
					result.Skipped++
					continue
				}
				if err := upsertSupplierParty(ctx, tx, orgID, *existingID, row); err != nil {
					result.Errors = append(result.Errors, ImportError{Row: idx + 2, Message: err.Error()})
					continue
				}
				result.Updated++
				continue
			}
			if err := createSupplierParty(ctx, tx, orgID, row); err != nil {
				result.Errors = append(result.Errors, ImportError{Row: idx + 2, Message: err.Error()})
				continue
			}
			result.Created++
		}
		return nil
	})
	return result, err
}

func (r *Repository) ImportProducts(ctx context.Context, orgID uuid.UUID, rows []map[string]string, mode string) (ImportResult, error) {
	result := ImportResult{TotalRows: len(rows)}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for idx, row := range rows {
			existingID, err := r.findProduct(ctx, tx, orgID, row)
			if err != nil {
				return err
			}
			if existingID != nil {
				if mode == "create_only" {
					result.Skipped++
					continue
				}
				if err := upsertProduct(ctx, tx, orgID, *existingID, row); err != nil {
					result.Errors = append(result.Errors, ImportError{Row: idx + 2, Message: err.Error()})
					continue
				}
				result.Updated++
				continue
			}
			if err := createProduct(ctx, tx, orgID, row); err != nil {
				result.Errors = append(result.Errors, ImportError{Row: idx + 2, Message: err.Error()})
				continue
			}
			result.Created++
		}
		return nil
	})
	return result, err
}

func (r *Repository) ExportCustomers(ctx context.Context, orgID uuid.UUID) ([]string, [][]string, error) {
	headers := []string{"name", "type", "email", "phone", "tax_id", "address_street", "address_city", "address_state", "address_zip_code", "address_country", "notes", "tags"}
	rows, err := queryRows(ctx, r.db, `
		SELECT
			COALESCE(p.display_name,''),
			CASE WHEN p.party_type = 'organization' THEN 'company' ELSE 'person' END,
			COALESCE(p.email,''),
			COALESCE(p.phone,''),
			COALESCE(p.tax_id,''),
			COALESCE(p.address->>'street',''),
			COALESCE(p.address->>'city',''),
			COALESCE(p.address->>'state',''),
			COALESCE(p.address->>'zip_code',''),
			COALESCE(p.address->>'country',''),
			COALESCE(p.notes,''),
			COALESCE(array_to_string(p.tags, ','), '')
		FROM parties p
		JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'customer' AND pr.is_active = true
		WHERE p.org_id = ? AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
	`, orgID)
	return headers, rows, err
}

func (r *Repository) ExportProducts(ctx context.Context, orgID uuid.UUID) ([]string, [][]string, error) {
	headers := []string{"name", "type", "sku", "price", "cost_price", "unit", "tax_rate", "track_stock", "description", "tags"}
	rows, err := queryRows(ctx, r.db, `
		SELECT
			name,
			type,
			COALESCE(sku,''),
			price,
			cost_price,
			COALESCE(unit,''),
			COALESCE(tax_rate, 0),
			CASE WHEN track_stock THEN 'true' ELSE 'false' END,
			COALESCE(description,''),
			COALESCE(array_to_string(tags, ','), '')
		FROM products
		WHERE org_id = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, orgID)
	return headers, rows, err
}

func (r *Repository) ExportSuppliers(ctx context.Context, orgID uuid.UUID) ([]string, [][]string, error) {
	headers := []string{"name", "email", "phone", "tax_id", "contact_name", "address_street", "address_city", "address_state", "address_zip_code", "address_country", "notes", "tags"}
	rows, err := queryRows(ctx, r.db, `
		SELECT
			COALESCE(p.display_name,''),
			COALESCE(p.email,''),
			COALESCE(p.phone,''),
			COALESCE(p.tax_id,''),
			COALESCE(pr.metadata->>'contact_name', p.metadata->>'contact_name', ''),
			COALESCE(p.address->>'street',''),
			COALESCE(p.address->>'city',''),
			COALESCE(p.address->>'state',''),
			COALESCE(p.address->>'zip_code',''),
			COALESCE(p.address->>'country',''),
			COALESCE(p.notes,''),
			COALESCE(array_to_string(p.tags, ','), '')
		FROM parties p
		JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = 'supplier' AND pr.is_active = true
		WHERE p.org_id = ? AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
	`, orgID)
	return headers, rows, err
}

func (r *Repository) ExportSales(ctx context.Context, orgID uuid.UUID, from, to *time.Time) ([]string, [][]string, error) {
	headers := []string{"number", "date", "customer_name", "payment_method", "subtotal", "tax_total", "total", "status", "items_summary"}
	query := `
		SELECT
			s.number,
			to_char(s.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD'),
			COALESCE(s.party_name, ''),
			s.payment_method,
			s.subtotal,
			s.tax_total,
			s.total,
			s.status,
			COALESCE((
				SELECT string_agg(TRIM(si.description) || ' x' || TRIM(to_char(si.quantity, 'FM999999990.##')), ' | ' ORDER BY si.sort_order)
				FROM sale_items si
				WHERE si.sale_id = s.id
			), '')
		FROM sales s
		WHERE s.org_id = ? AND s.created_at >= ? AND s.created_at <= ?
		ORDER BY s.created_at DESC
	`
	rows, err := queryRows(ctx, r.db, query, orgID, *from, toEndOfDay(*to))
	return headers, rows, err
}

func (r *Repository) ExportCashflow(ctx context.Context, orgID uuid.UUID, from, to *time.Time) ([]string, [][]string, error) {
	headers := []string{"date", "type", "amount", "category", "description", "payment_method", "reference_type"}
	query := `
		SELECT
			to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD'),
			type,
			amount,
			category,
			COALESCE(description,''),
			payment_method,
			reference_type
		FROM cash_movements
		WHERE org_id = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
	`
	rows, err := queryRows(ctx, r.db, query, orgID, *from, toEndOfDay(*to))
	return headers, rows, err
}

func (r *Repository) findPartyByRole(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, role, email, taxID string) (*uuid.UUID, error) {
	query := tx.WithContext(ctx).
		Table("parties p").
		Select("p.id").
		Joins("JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id AND pr.role = ? AND pr.is_active = true", role).
		Where("p.org_id = ? AND p.deleted_at IS NULL", orgID)
	if strings.TrimSpace(taxID) != "" {
		query = query.Where("p.tax_id = ?", strings.TrimSpace(taxID))
	} else if strings.TrimSpace(email) != "" {
		query = query.Where("LOWER(p.email) = LOWER(?)", strings.TrimSpace(email))
	} else {
		return nil, nil
	}
	var id uuid.UUID
	if err := query.Take(&id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

func (r *Repository) findProduct(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, row map[string]string) (*uuid.UUID, error) {
	q := tx.WithContext(ctx).Table("products").Select("id").Where("org_id = ? AND deleted_at IS NULL", orgID)
	sku := strings.TrimSpace(row["sku"])
	name := strings.TrimSpace(row["name"])
	switch {
	case sku != "":
		q = q.Where("sku = ?", sku)
	case name != "":
		q = q.Where("LOWER(name) = LOWER(?)", name)
	default:
		return nil, nil
	}
	var id uuid.UUID
	if err := q.Take(&id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

func createCustomerParty(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, row map[string]string) (uuid.UUID, error) {
	partyID := uuid.New()
	partyType := customerPartyType(row["type"])
	address, meta, tags := customerPartyData(row)
	if err := tx.WithContext(ctx).Table("parties").Create(map[string]any{
		"id":           partyID,
		"org_id":       orgID,
		"party_type":   partyType,
		"display_name": strings.TrimSpace(row["name"]),
		"email":        strings.TrimSpace(row["email"]),
		"phone":        strings.TrimSpace(row["phone"]),
		"address":      address,
		"tax_id":       strings.TrimSpace(row["tax_id"]),
		"notes":        strings.TrimSpace(row["notes"]),
		"tags":         pq.StringArray(tags),
		"metadata":     meta,
		"created_at":   time.Now().UTC(),
		"updated_at":   time.Now().UTC(),
	}).Error; err != nil {
		return uuid.Nil, err
	}
	if err := upsertCustomerExtension(ctx, tx, partyID, partyType, strings.TrimSpace(row["name"])); err != nil {
		return uuid.Nil, err
	}
	return partyID, tx.WithContext(ctx).Exec(`
		INSERT INTO party_roles (id, party_id, org_id, role, is_active, metadata, created_at)
		VALUES (?, ?, ?, 'customer', true, '{}'::jsonb, now())
		ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = true
	`, uuid.New(), partyID, orgID).Error
}

func upsertCustomerParty(ctx context.Context, tx *gorm.DB, orgID, partyID uuid.UUID, row map[string]string) error {
	partyType := customerPartyType(row["type"])
	address, meta, tags := customerPartyData(row)
	res := tx.WithContext(ctx).Table("parties").Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, partyID).Updates(map[string]any{
		"party_type":   partyType,
		"display_name": strings.TrimSpace(row["name"]),
		"email":        strings.TrimSpace(row["email"]),
		"phone":        strings.TrimSpace(row["phone"]),
		"address":      address,
		"tax_id":       strings.TrimSpace(row["tax_id"]),
		"notes":        strings.TrimSpace(row["notes"]),
		"tags":         pq.StringArray(tags),
		"metadata":     meta,
		"updated_at":   time.Now().UTC(),
	})
	if res.Error != nil {
		return res.Error
	}
	if err := upsertCustomerExtension(ctx, tx, partyID, partyType, strings.TrimSpace(row["name"])); err != nil {
		return err
	}
	return nil
}

func createSupplierParty(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, row map[string]string) error {
	partyID := uuid.New()
	address, meta, roleMeta, tags := supplierPartyData(row)
	if err := tx.WithContext(ctx).Table("parties").Create(map[string]any{
		"id":           partyID,
		"org_id":       orgID,
		"party_type":   "organization",
		"display_name": strings.TrimSpace(row["name"]),
		"email":        strings.TrimSpace(row["email"]),
		"phone":        strings.TrimSpace(row["phone"]),
		"address":      address,
		"tax_id":       strings.TrimSpace(row["tax_id"]),
		"notes":        strings.TrimSpace(row["notes"]),
		"tags":         pq.StringArray(tags),
		"metadata":     meta,
		"created_at":   time.Now().UTC(),
		"updated_at":   time.Now().UTC(),
	}).Error; err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Exec(`
		INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
		VALUES (?, ?, ?, '')
		ON CONFLICT (party_id) DO UPDATE SET legal_name = EXCLUDED.legal_name, trade_name = EXCLUDED.trade_name
	`, partyID, strings.TrimSpace(row["name"]), strings.TrimSpace(row["name"])).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Exec(`
		INSERT INTO party_roles (id, party_id, org_id, role, is_active, metadata, created_at)
		VALUES (?, ?, ?, 'supplier', true, ?::jsonb, now())
		ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = true, metadata = EXCLUDED.metadata
	`, uuid.New(), partyID, orgID, string(roleMeta)).Error
}

func upsertSupplierParty(ctx context.Context, tx *gorm.DB, orgID, partyID uuid.UUID, row map[string]string) error {
	address, meta, roleMeta, tags := supplierPartyData(row)
	res := tx.WithContext(ctx).Table("parties").Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, partyID).Updates(map[string]any{
		"party_type":   "organization",
		"display_name": strings.TrimSpace(row["name"]),
		"email":        strings.TrimSpace(row["email"]),
		"phone":        strings.TrimSpace(row["phone"]),
		"address":      address,
		"tax_id":       strings.TrimSpace(row["tax_id"]),
		"notes":        strings.TrimSpace(row["notes"]),
		"tags":         pq.StringArray(tags),
		"metadata":     meta,
		"updated_at":   time.Now().UTC(),
	})
	if res.Error != nil {
		return res.Error
	}
	if err := tx.WithContext(ctx).Exec(`
		INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
		VALUES (?, ?, ?, '')
		ON CONFLICT (party_id) DO UPDATE SET legal_name = EXCLUDED.legal_name, trade_name = EXCLUDED.trade_name
	`, partyID, strings.TrimSpace(row["name"]), strings.TrimSpace(row["name"])).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Exec(`UPDATE party_roles SET metadata = ?::jsonb, is_active = true WHERE org_id = ? AND party_id = ? AND role = 'supplier'`, string(roleMeta), orgID, partyID).Error
}

func createProduct(ctx context.Context, tx *gorm.DB, orgID uuid.UUID, row map[string]string) error {
	price, err := parseMoney(row["price"])
	if err != nil {
		return err
	}
	costPrice, err := parseOptionalMoney(row["cost_price"])
	if err != nil {
		return err
	}
	taxRate, err := parseOptionalMoney(row["tax_rate"])
	if err != nil {
		return err
	}
	trackStock := parseBool(row["track_stock"], true)
	return tx.WithContext(ctx).Exec(`
		INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '{}'::jsonb, now(), now())
	`, uuid.New(), orgID, defaultProductType(row["type"]), strings.TrimSpace(row["sku"]), strings.TrimSpace(row["name"]), strings.TrimSpace(row["description"]), defaultString(row["unit"], "unit"), price, costPrice, taxRate, trackStock, pq.StringArray(splitTags(row["tags"]))).Error
}

func upsertProduct(ctx context.Context, tx *gorm.DB, orgID, id uuid.UUID, row map[string]string) error {
	price, err := parseMoney(row["price"])
	if err != nil {
		return err
	}
	costPrice, err := parseOptionalMoney(row["cost_price"])
	if err != nil {
		return err
	}
	taxRate, err := parseOptionalMoney(row["tax_rate"])
	if err != nil {
		return err
	}
	trackStock := parseBool(row["track_stock"], true)
	return tx.WithContext(ctx).Table("products").Where("org_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Updates(map[string]any{
		"type":        defaultProductType(row["type"]),
		"sku":         strings.TrimSpace(row["sku"]),
		"name":        strings.TrimSpace(row["name"]),
		"description": strings.TrimSpace(row["description"]),
		"unit":        defaultString(row["unit"], "unit"),
		"price":       price,
		"cost_price":  costPrice,
		"tax_rate":    taxRate,
		"track_stock": trackStock,
		"tags":        pq.StringArray(splitTags(row["tags"])),
		"updated_at":  time.Now().UTC(),
	}).Error
}

func customerPartyType(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "company") {
		return "organization"
	}
	return "person"
}

func customerPartyData(row map[string]string) ([]byte, []byte, []string) {
	address, _ := json.Marshal(map[string]string{
		"street":   strings.TrimSpace(row["address_street"]),
		"city":     strings.TrimSpace(row["address_city"]),
		"state":    strings.TrimSpace(row["address_state"]),
		"zip_code": strings.TrimSpace(row["address_zip_code"]),
		"country":  strings.TrimSpace(row["address_country"]),
	})
	meta, _ := json.Marshal(map[string]any{})
	return address, meta, splitTags(row["tags"])
}

func supplierPartyData(row map[string]string) ([]byte, []byte, []byte, []string) {
	address, _ := json.Marshal(map[string]string{
		"street":   strings.TrimSpace(row["address_street"]),
		"city":     strings.TrimSpace(row["address_city"]),
		"state":    strings.TrimSpace(row["address_state"]),
		"zip_code": strings.TrimSpace(row["address_zip_code"]),
		"country":  strings.TrimSpace(row["address_country"]),
	})
	meta, _ := json.Marshal(map[string]any{"contact_name": strings.TrimSpace(row["contact_name"])})
	roleMeta, _ := json.Marshal(map[string]any{"contact_name": strings.TrimSpace(row["contact_name"])})
	return address, meta, roleMeta, splitTags(row["tags"])
}

func upsertCustomerExtension(ctx context.Context, tx *gorm.DB, partyID uuid.UUID, partyType, name string) error {
	if partyType == "person" {
		first, last := splitName(name)
		if err := tx.WithContext(ctx).Exec("DELETE FROM party_organizations WHERE party_id = ?", partyID).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Exec(`
			INSERT INTO party_persons (party_id, first_name, last_name)
			VALUES (?, ?, ?)
			ON CONFLICT (party_id) DO UPDATE SET first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name
		`, partyID, first, last).Error
	}
	if err := tx.WithContext(ctx).Exec("DELETE FROM party_persons WHERE party_id = ?", partyID).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Exec(`
		INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
		VALUES (?, ?, ?, '')
		ON CONFLICT (party_id) DO UPDATE SET legal_name = EXCLUDED.legal_name, trade_name = EXCLUDED.trade_name
	`, partyID, name, name).Error
}

func splitName(name string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func splitTags(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func parseMoney(raw string) (float64, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	if value == "" {
		return 0, fmt.Errorf("missing numeric value")
	}
	return strconv.ParseFloat(value, 64)
}

func parseOptionalMoney(raw string) (float64, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func parseBool(raw string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "1", "true", "si", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func defaultProductType(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "service") {
		return "service"
	}
	return "product"
}

func toEndOfDay(day time.Time) time.Time {
	return day.UTC().Add(24*time.Hour - time.Nanosecond)
}

func queryRows(ctx context.Context, db *gorm.DB, query string, args ...any) ([][]string, error) {
	rows, err := db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([][]string, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]string, len(cols))
		for i, value := range values {
			row[i] = stringify(value)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	case sql.NullString:
		if v.Valid {
			return strings.TrimSpace(v.String)
		}
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
