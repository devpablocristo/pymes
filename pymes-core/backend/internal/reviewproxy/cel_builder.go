package reviewproxy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/reviewproxy/handler/dto"
)

var (
	percentageGTPattern  = regexp.MustCompile(`(?i)percentage_gt:(\d+(?:\.\d+)?)`)
	percentageLTEPattern = regexp.MustCompile(`(?i)percentage_lte:(\d+(?:\.\d+)?)`)
	daysWithinPattern    = regexp.MustCompile(`(?i)days_within:(\d+)`)
	amountGTPattern      = regexp.MustCompile(`(?i)amount_gt:(\d+(?:\.\d+)?)`)
)

// BuildCELExpression traduce una condición del frontend a expresión CEL.
// Si condition es nil o vacío, retorna un match simple por action_type.
func BuildCELExpression(actionType string, condition *string) string {
	base := fmt.Sprintf(`request.action_type == "%s"`, actionType)

	if condition == nil || strings.TrimSpace(*condition) == "" {
		return base
	}
	c := strings.TrimSpace(*condition)

	if m := percentageGTPattern.FindStringSubmatch(c); len(m) == 2 {
		val, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			return fmt.Sprintf(`%s && double(request.params.percentage) > %v`, base, val)
		}
	}

	if m := percentageLTEPattern.FindStringSubmatch(c); len(m) == 2 {
		val, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			return fmt.Sprintf(`%s && double(request.params.percentage) <= %v`, base, val)
		}
	}

	if m := daysWithinPattern.FindStringSubmatch(c); len(m) == 2 {
		val, err := strconv.Atoi(m[1])
		if err == nil {
			return fmt.Sprintf(`%s && int(request.params.days_from_now) <= %d`, base, val)
		}
	}

	if m := amountGTPattern.FindStringSubmatch(c); len(m) == 2 {
		val, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			return fmt.Sprintf(`%s && double(request.params.amount) > %v`, base, val)
		}
	}

	// Si no matchea ningún patrón conocido, retorna solo el match base
	return base
}

// GetConditionTemplates devuelve templates de condición disponibles para un action type.
func GetConditionTemplates(actionType string) []dto.ConditionTemplate {
	switch actionType {
	case "discount.apply":
		return []dto.ConditionTemplate{
			{
				Label:      "Descuento mayor a X%",
				Pattern:    "percentage_gt",
				ParamName:  "percentage",
				ParamType:  "number",
				DefaultVal: "10",
			},
			{
				Label:      "Descuento menor o igual a X%",
				Pattern:    "percentage_lte",
				ParamName:  "percentage",
				ParamType:  "number",
				DefaultVal: "10",
			},
		}
	case "appointment.reschedule":
		return []dto.ConditionTemplate{
			{
				Label:      "Dentro de X dias",
				Pattern:    "days_within",
				ParamName:  "days",
				ParamType:  "number",
				DefaultVal: "7",
			},
		}
	case "cashflow.movement", "purchase.draft", "procurement.request":
		return []dto.ConditionTemplate{
			{
				Label:      "Monto mayor a X",
				Pattern:    "amount_gt",
				ParamName:  "amount",
				ParamType:  "number",
				DefaultVal: "10000",
			},
		}
	default:
		return []dto.ConditionTemplate{}
	}
}
