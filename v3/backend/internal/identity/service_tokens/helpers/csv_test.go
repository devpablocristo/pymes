package helpers

import "testing"

func TestCSVValuesTrimsAndDropsEmptyItems(t *testing.T) {
	values := CSVValues(" one, ,two ")
	if len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("unexpected values %#v", values)
	}
}
