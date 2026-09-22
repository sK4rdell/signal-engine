package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

// Template names one embedded message template.
type Template string

// Built-in templates. Each has templates/<name>.txt with "subject" and
// "text" definitions, and templates/<name>.html with the HTML body.
const (
	TemplateVerifyEmail   Template = "verify_email"
	TemplateResetPassword Template = "reset_password"
	TemplateWelcome       Template = "welcome"
)

//go:embed templates/*.txt templates/*.html
var templateFS embed.FS

// Every template file is parsed into its own set. A shared set would let
// the "subject" and "text" definitions of one file overwrite another's.
type parsedTemplate struct {
	text *texttemplate.Template
	html *htmltemplate.Template
}

var templates = mustParseTemplates()

func mustParseTemplates() map[Template]parsedTemplate {
	out := map[Template]parsedTemplate{}
	for _, name := range []Template{TemplateVerifyEmail, TemplateResetPassword, TemplateWelcome} {
		text, err := texttemplate.ParseFS(templateFS, "templates/"+string(name)+".txt")
		if err != nil {
			panic(fmt.Sprintf("email: parse %s.txt: %v", name, err))
		}
		html, err := htmltemplate.ParseFS(templateFS, "templates/"+string(name)+".html")
		if err != nil {
			panic(fmt.Sprintf("email: parse %s.html: %v", name, err))
		}
		out[name] = parsedTemplate{text: text, html: html}
	}
	return out
}

// Render builds a message from a template. The recipient list is set by the
// caller; data is whatever the template expects.
func Render(tmpl Template, to []string, data any) (Message, error) {
	parsed, ok := templates[tmpl]
	if !ok {
		return Message{}, fmt.Errorf("email: unknown template %q", tmpl)
	}

	var subject, text, html bytes.Buffer
	if err := parsed.text.ExecuteTemplate(&subject, "subject", data); err != nil {
		return Message{}, fmt.Errorf("email: render %s subject: %w", tmpl, err)
	}
	if err := parsed.text.ExecuteTemplate(&text, "text", data); err != nil {
		return Message{}, fmt.Errorf("email: render %s text: %w", tmpl, err)
	}
	if err := parsed.html.ExecuteTemplate(&html, string(tmpl)+".html", data); err != nil {
		return Message{}, fmt.Errorf("email: render %s html: %w", tmpl, err)
	}

	return Message{
		To:      to,
		Subject: strings.TrimSpace(subject.String()),
		Text:    strings.TrimSpace(text.String()) + "\n",
		HTML:    html.String(),
	}, nil
}
