package products

import (
	"reflect"
	"testing"
)

func TestParseImageURLsFromMetadata(t *testing.T) {
	t.Parallel()

	urls, ok := parseImageURLsFromMetadata(map[string]any{
		"image_urls": []string{" https://a.example/x ", "https://b.example/y"},
	})
	if !ok {
		t.Fatalf("expected key present")
	}
	if !reflect.DeepEqual(urls, []string{" https://a.example/x ", "https://b.example/y"}) {
		t.Fatalf("unexpected urls: %#v", urls)
	}

	_, ok = parseImageURLsFromMetadata(map[string]any{"other": 1})
	if ok {
		t.Fatalf("expected key absent")
	}
}

func TestMergeProductMetadataImageURLsPreservesOtherKeys(t *testing.T) {
	t.Parallel()

	out := mergeProductMetadataImageURLs(map[string]any{"k": 1}, []string{"https://z"})
	if out["k"] != 1 {
		t.Fatalf("expected other key preserved")
	}
	got, _ := out[metadataImageURLsKey].([]string)
	if len(got) != 1 || got[0] != "https://z" {
		t.Fatalf("unexpected image_urls: %#v", got)
	}
}
