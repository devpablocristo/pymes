package notifications

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

var baseTemplate = template.Must(template.ParseFS(templatesFS, "templates/base.html"))

type templateData struct {
	Title          string
	Message        string
	ActionURL      string
	ActionLabel    string
	OrgName        string
	PreferencesURL string
}

func renderBaseTemplate(data templateData) (string, error) {
	var b bytes.Buffer
	if err := baseTemplate.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
