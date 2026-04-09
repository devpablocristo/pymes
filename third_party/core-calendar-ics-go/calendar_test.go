package ics

import (
	"strings"
	"testing"
	"time"
)

func TestRender_RejectsMissingProdID(t *testing.T) {
	t.Parallel()
	if _, err := Render(Calendar{}); err == nil {
		t.Fatal("expected error for missing ProdID")
	}
}

func TestRender_MinimalCalendarHasRequiredHeaders(t *testing.T) {
	t.Parallel()
	out, err := Render(Calendar{ProdID: "-//Test//Demo//EN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"BEGIN:VCALENDAR\r\n",
		"VERSION:2.0\r\n",
		"PRODID:-//Test//Demo//EN\r\n",
		"CALSCALE:GREGORIAN\r\n",
		"METHOD:PUBLISH\r\n",
		"END:VCALENDAR\r\n",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("expected output to contain %q, got:\n%s", w, out)
		}
	}
}

func TestRender_TimedEventEmitsDateTimeUTC(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:     "evt-1@example.com",
			DTSTAMP: time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC),
			Start:   start,
			End:     end,
			Summary: "Reunión equipo",
			Status:  StatusConfirmed,
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, line := range []string{
		"BEGIN:VEVENT\r\n",
		"UID:evt-1@example.com\r\n",
		"DTSTAMP:20260408T120000Z\r\n",
		"DTSTART:20260409T200000Z\r\n",
		"DTEND:20260409T210000Z\r\n",
		"SUMMARY:Reunión equipo\r\n",
		"STATUS:CONFIRMED\r\n",
		"END:VEVENT\r\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("expected line %q in output:\n%s", line, out)
		}
	}
}

func TestRender_AllDayEventUsesValueDate(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:     "all-day@example.com",
			Start:   day,
			AllDay:  true,
			Summary: "Día del trabajador",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "DTSTART;VALUE=DATE:20260501\r\n") {
		t.Errorf("expected DTSTART;VALUE=DATE in output, got:\n%s", out)
	}
}

func TestRender_RejectsEventWithEndBeforeStart(t *testing.T) {
	t.Parallel()
	_, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:   "bad@example.com",
			Start: time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC),
		}},
	})
	if err == nil {
		t.Fatal("expected error for End <= Start")
	}
}

func TestRender_EscapesSpecialCharsInText(t *testing.T) {
	t.Parallel()
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:         "esc@example.com",
			Start:       time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC),
			End:         time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
			Summary:     `with, comma; and \ backslash`,
			Description: "first line\nsecond line",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `SUMMARY:with\, comma\; and \\ backslash`) {
		t.Errorf("special chars not escaped in summary, got:\n%s", out)
	}
	if !strings.Contains(out, `DESCRIPTION:first line\nsecond line`) {
		t.Errorf("newline not escaped in description, got:\n%s", out)
	}
}

func TestRender_FoldsLongLinesAt75Octets(t *testing.T) {
	t.Parallel()
	// Una descripción de >75 octetos en un campo SUMMARY tiene que quedar
	// partida en una primera línea ≤75 y continuaciones que arrancan con
	// espacio ≤75 (incluyendo el espacio).
	long := strings.Repeat("a", 200)
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:     "long@example.com",
			Start:   time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
			Summary: long,
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Encontrar el bloque del SUMMARY y verificar que ninguna línea del feed
	// excede 75 octetos. Esto es el invariante real que clientes estrictos
	// validan.
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Fatalf("found line longer than 75 octets (len=%d): %q", len(line), line)
		}
	}
	// Y la concatenación de las continuaciones (sin el primer espacio) debe
	// reconstruir el contenido original del SUMMARY.
	var rebuilt strings.Builder
	inSummary := false
	for _, line := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(line, "SUMMARY:") {
			rebuilt.WriteString(strings.TrimPrefix(line, "SUMMARY:"))
			inSummary = true
			continue
		}
		if inSummary {
			if strings.HasPrefix(line, " ") {
				rebuilt.WriteString(strings.TrimPrefix(line, " "))
				continue
			}
			break
		}
	}
	if rebuilt.String() != long {
		t.Errorf("rebuilt summary does not match original.\n  got:  %q\n  want: %q", rebuilt.String(), long)
	}
}

func TestRender_FoldingDoesNotSplitUTF8Runes(t *testing.T) {
	t.Parallel()
	// "ñ" en UTF-8 es 2 bytes (0xC3 0xB1). Si paddeamos para que el corte
	// caiga justo en el segundo byte, el folder no debe dejar el continuation
	// byte huérfano.
	padding := strings.Repeat("a", 73) // SUMMARY: + 73 = 81 chars antes de la ñ → cae a la mitad
	summary := padding + "ñ" + "tail"
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:     "utf8@example.com",
			Start:   time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
			Summary: summary,
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Reconstruir como en el test anterior y verificar que la ñ se preserve.
	var rebuilt strings.Builder
	inSummary := false
	for _, line := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(line, "SUMMARY:") {
			rebuilt.WriteString(strings.TrimPrefix(line, "SUMMARY:"))
			inSummary = true
			continue
		}
		if inSummary {
			if strings.HasPrefix(line, " ") {
				rebuilt.WriteString(strings.TrimPrefix(line, " "))
				continue
			}
			break
		}
	}
	if rebuilt.String() != summary {
		t.Errorf("UTF-8 rune was split during folding.\n  got:  %q\n  want: %q", rebuilt.String(), summary)
	}
}

func TestRender_AppliesDTSTAMPFallbackWhenZero(t *testing.T) {
	t.Parallel()
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:   "fallback@example.com",
			Start: time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "DTSTAMP:") {
		t.Errorf("expected DTSTAMP fallback to be set, got:\n%s", out)
	}
}

func TestRender_OptionalFieldsOnlyEmittedWhenSet(t *testing.T) {
	t.Parallel()
	out, err := Render(Calendar{
		ProdID: "-//Test//Demo//EN",
		Events: []Event{{
			UID:   "minimal@example.com",
			Start: time.Date(2026, 4, 9, 20, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 9, 21, 0, 0, 0, time.UTC),
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, prefix := range []string{"SUMMARY:", "DESCRIPTION:", "LOCATION:", "STATUS:", "ORGANIZER:", "URL:", "LAST-MODIFIED:"} {
		if strings.Contains(out, prefix) {
			t.Errorf("optional %s should not be emitted when unset, got:\n%s", prefix, out)
		}
	}
}

func TestRender_CalendarNameAndDescription(t *testing.T) {
	t.Parallel()
	out, err := Render(Calendar{
		ProdID:      "-//Test//Demo//EN",
		Name:        "Mi agenda",
		Description: "Calendario interno",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, expected := range []string{
		"NAME:Mi agenda\r\n",
		"X-WR-CALNAME:Mi agenda\r\n",
		"DESCRIPTION:Calendario interno\r\n",
		"X-WR-CALDESC:Calendario interno\r\n",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in output:\n%s", expected, out)
		}
	}
}
