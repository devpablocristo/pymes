// Package ics serializa calendarios en el formato iCalendar (RFC 5545).
//
// Es deliberadamente agnóstico de cualquier producto: no conoce bookings, ni
// usuarios, ni tenants. Recibe primitivas (calendario + eventos) y devuelve el
// texto iCalendar listo para emitir como `text/calendar` o servir desde un
// endpoint que cualquier cliente compatible (Apple Calendar, Google Calendar,
// Outlook, Thunderbird) pueda suscribir vía URL.
//
// Cobertura: lo necesario para feeds de sólo lectura y suscripciones por URL.
// Eso significa VCALENDAR + VEVENT con summary, description, location, dtstart,
// dtend, uid, status, organizer, last-modified y dtstamp. NO cubre alarmas,
// recurrences (RRULE), zonas horarias VTIMEZONE personalizadas ni journals/todos.
// Si en algún momento hace falta RRULE, este package es el lugar correcto donde
// agregarlo, no en el código del producto.
package ics

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status modela los estados de evento que RFC 5545 define en la sección 3.8.1.11.
// El subset publicado acá es el que importa para feeds de calendario interno:
// confirmed (default cuando no se especifica), tentative, cancelled.
type Status string

const (
	StatusConfirmed Status = "CONFIRMED"
	StatusTentative Status = "TENTATIVE"
	StatusCancelled Status = "CANCELLED"
)

// Calendar es la unidad de salida del package: una lista de eventos más
// metadata del calendario en sí (PRODID obligatorio por la RFC, nombre y
// descripción opcionales que la mayoría de los clientes leen para titular la
// suscripción).
type Calendar struct {
	// ProdID identifica al producto que generó el calendario, obligatorio
	// por RFC 5545 §3.7.3. Convención: "-//Empresa//Producto//Idioma".
	ProdID string
	// Name es el título visible cuando el cliente suscribe al feed.
	// Se serializa como X-WR-CALNAME (extension de Google) y NAME (RFC 7986).
	Name string
	// Description es texto largo opcional. X-WR-CALDESC + DESCRIPTION (RFC 7986).
	Description string
	// Events es la lista de VEVENT.
	Events []Event
}

// Event modela un VEVENT mínimo. Los campos punteros u opcionales son los
// que la RFC marca como SHOULD/MAY; los obligatorios (UID, DTSTAMP, DTSTART)
// se validan en Render para fallar temprano si falta alguno.
type Event struct {
	// UID identifica al evento de forma globalmente única y estable.
	// Si el productor lo cambia entre renders, los clientes lo tratan como
	// un evento nuevo. Convención: <id>@<dominio>.
	UID string
	// DTSTAMP es el instante en que se generó el evento. Obligatorio por RFC.
	// Si está vacío, Render usa time.Now().UTC().
	DTSTAMP time.Time
	// Start y End delimitan el rango. End es exclusivo, igual que la
	// convención del package time de Go.
	Start time.Time
	End   time.Time
	// AllDay convierte DTSTART/DTEND a la forma DATE (sin hora) en vez de
	// DATE-TIME. Útil para vacaciones, feriados, eventos personales sin reloj.
	AllDay bool
	// Summary, Description, Location: texto visible al usuario en su cliente.
	Summary     string
	Description string
	Location    string
	Status      Status
	// Organizer es opcional. Formato CAL-ADDRESS: "mailto:foo@bar.com".
	Organizer string
	// LastModified es el último momento en que el productor modificó el
	// evento. Permite que clientes con caché detecten cambios.
	LastModified time.Time
	// URL opcional, RFC 5545 §3.8.4.6. Útil para volver al evento en la app.
	URL string
}

// Render serializa un Calendar a texto iCalendar válido. Devuelve error si
// falta algún campo obligatorio o algún evento es inválido.
//
// El output usa CRLF como line ending (lo que la RFC exige) y aplica
// "line folding" en líneas mayores a 75 octetos (también obligatorio por la
// RFC; sin esto, parsers estrictos como los de Apple Calendar rechazan el
// feed). El consumidor recibe un string limpio listo para escribir a un
// http.ResponseWriter con Content-Type: text/calendar; charset=utf-8.
func Render(cal Calendar) (string, error) {
	if strings.TrimSpace(cal.ProdID) == "" {
		return "", errors.New("ics: ProdID is required")
	}

	var b strings.Builder
	writeLine(&b, "BEGIN:VCALENDAR")
	writeLine(&b, "VERSION:2.0")
	writeLine(&b, "PRODID:"+escapeText(cal.ProdID))
	writeLine(&b, "CALSCALE:GREGORIAN")
	writeLine(&b, "METHOD:PUBLISH")

	if name := strings.TrimSpace(cal.Name); name != "" {
		writeLine(&b, "NAME:"+escapeText(name))
		writeLine(&b, "X-WR-CALNAME:"+escapeText(name))
	}
	if desc := strings.TrimSpace(cal.Description); desc != "" {
		writeLine(&b, "DESCRIPTION:"+escapeText(desc))
		writeLine(&b, "X-WR-CALDESC:"+escapeText(desc))
	}

	now := time.Now().UTC()
	for i := range cal.Events {
		ev := cal.Events[i]
		if err := writeEvent(&b, ev, now); err != nil {
			return "", fmt.Errorf("ics: event %d: %w", i, err)
		}
	}

	writeLine(&b, "END:VCALENDAR")
	return b.String(), nil
}

func writeEvent(b *strings.Builder, ev Event, fallbackStamp time.Time) error {
	if strings.TrimSpace(ev.UID) == "" {
		return errors.New("UID is required")
	}
	if ev.Start.IsZero() {
		return errors.New("Start is required")
	}
	if !ev.AllDay && ev.End.IsZero() {
		return errors.New("End is required for timed events")
	}
	if !ev.End.IsZero() && !ev.End.After(ev.Start) {
		return errors.New("End must be after Start")
	}

	dtstamp := ev.DTSTAMP
	if dtstamp.IsZero() {
		dtstamp = fallbackStamp
	}

	writeLine(b, "BEGIN:VEVENT")
	writeLine(b, "UID:"+escapeText(ev.UID))
	writeLine(b, "DTSTAMP:"+formatUTC(dtstamp))

	if ev.AllDay {
		writeLine(b, "DTSTART;VALUE=DATE:"+formatDate(ev.Start))
		if !ev.End.IsZero() {
			writeLine(b, "DTEND;VALUE=DATE:"+formatDate(ev.End))
		}
	} else {
		writeLine(b, "DTSTART:"+formatUTC(ev.Start))
		writeLine(b, "DTEND:"+formatUTC(ev.End))
	}

	if summary := strings.TrimSpace(ev.Summary); summary != "" {
		writeLine(b, "SUMMARY:"+escapeText(summary))
	}
	if desc := strings.TrimSpace(ev.Description); desc != "" {
		writeLine(b, "DESCRIPTION:"+escapeText(desc))
	}
	if loc := strings.TrimSpace(ev.Location); loc != "" {
		writeLine(b, "LOCATION:"+escapeText(loc))
	}
	if ev.Status != "" {
		writeLine(b, "STATUS:"+string(ev.Status))
	}
	if org := strings.TrimSpace(ev.Organizer); org != "" {
		writeLine(b, "ORGANIZER:"+escapeText(org))
	}
	if !ev.LastModified.IsZero() {
		writeLine(b, "LAST-MODIFIED:"+formatUTC(ev.LastModified))
	}
	if url := strings.TrimSpace(ev.URL); url != "" {
		writeLine(b, "URL:"+escapeText(url))
	}

	writeLine(b, "END:VEVENT")
	return nil
}

// formatUTC produce el formato DATE-TIME en UTC: 20260409T200000Z (RFC 5545 §3.3.5).
func formatUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// formatDate produce el formato DATE: 20260409 (RFC 5545 §3.3.4).
// Usa la fecha en UTC para evitar deriva por la zona del proceso productor.
func formatDate(t time.Time) string {
	return t.UTC().Format("20060102")
}

// escapeText aplica el escape de RFC 5545 §3.3.11: backslashes, comas, punto y
// coma, y newlines pasan a `\\`, `\,`, `\;`, `\n`. Sin esto, una descripción
// con una coma rompe el parser de Apple Calendar.
func escapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case ',':
			b.WriteString(`\,`)
		case ';':
			b.WriteString(`\;`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			// CR sólo se omite si después viene LF (CRLF), si no, se traduce
			// a \n. Mantiene la equivalencia semántica con el original.
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// writeLine emite una línea con CRLF + folding RFC 5545 §3.1: si la línea
// excede 75 octetos, se parte en chunks y los chunks subsiguientes empiezan
// con un espacio simple. Importante: el split es por bytes (octetos), no por
// runes — la RFC habla de octetos. Si un punto de corte cae en medio de una
// secuencia UTF-8 multi-byte, se retrocede al inicio de la rune para no
// romper el encoding.
func writeLine(b *strings.Builder, line string) {
	const maxOctets = 75
	if len(line) <= maxOctets {
		b.WriteString(line)
		b.WriteString("\r\n")
		return
	}

	// Primer chunk: hasta maxOctets, respetando boundary UTF-8.
	first := safeUTF8Cut(line, maxOctets)
	b.WriteString(first)
	b.WriteString("\r\n")
	rest := line[len(first):]

	// Chunks subsiguientes: empiezan con espacio (que también cuenta), así
	// que el contenido máximo por línea continuada es maxOctets-1.
	const contMax = maxOctets - 1
	for len(rest) > 0 {
		chunk := rest
		if len(chunk) > contMax {
			chunk = safeUTF8Cut(rest, contMax)
		}
		b.WriteString(" ")
		b.WriteString(chunk)
		b.WriteString("\r\n")
		rest = rest[len(chunk):]
	}
}

// safeUTF8Cut devuelve el prefijo más largo de s que no excede maxBytes y que
// no parte una rune UTF-8 a la mitad. Si maxBytes cae justo en un boundary,
// no se retrocede; si cae en medio de una secuencia continuation (0x80-0xBF),
// retrocede hasta el último start byte.
func safeUTF8Cut(s string, maxBytes int) string {
	if maxBytes >= len(s) {
		return s
	}
	cut := maxBytes
	for cut > 0 && isUTF8Continuation(s[cut]) {
		cut--
	}
	return s[:cut]
}

func isUTF8Continuation(b byte) bool {
	return b&0xC0 == 0x80
}
