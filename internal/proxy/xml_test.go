package proxy

import (
	"strings"
	"testing"
)

const emptyRSS = `<?xml version="1.0"?><rss><channel><title>feed</title></channel></rss>`

func rssWithItems(items ...string) string {
	return `<?xml version="1.0"?><rss><channel><title>feed</title>` + strings.Join(items, "") + `</channel></rss>`
}

func item(title string) string {
	return `<item><title>` + title + `</title></item>`
}

func TestCountItemsHandlesEmptyAndItems(t *testing.T) {
	if got := countItems(""); got != 0 {
		t.Fatalf("countItems(empty) = %d, want 0", got)
	}
	if got := countItems(rssWithItems(item("one"), item("two"))); got != 2 {
		t.Fatalf("countItems(two items) = %d, want 2", got)
	}
}

func TestMergeXMLAppendsItemsFromSecondFeed(t *testing.T) {
	first := rssWithItems(item("one"))
	second := rssWithItems(item("two"), item("three"))

	got := mergeXML(first, second)
	if countItems(got) != 3 {
		t.Fatalf("merged item count = %d, want 3; xml=%s", countItems(got), got)
	}
	for _, title := range []string{"one", "two", "three"} {
		if !strings.Contains(got, "<title>"+title+"</title>") {
			t.Fatalf("merged xml missing title %q: %s", title, got)
		}
	}
}

func TestMergeXMLPrefersFeedWithItemsWhenOtherHasNone(t *testing.T) {
	withItems := rssWithItems(item("one"))
	if got := mergeXML(emptyRSS, withItems); got != withItems {
		t.Fatalf("mergeXML(empty, withItems) = %q, want %q", got, withItems)
	}
	if got := mergeXML(withItems, emptyRSS); got != withItems {
		t.Fatalf("mergeXML(withItems, empty) = %q, want %q", got, withItems)
	}
}

func TestTrimXMLItemsLimitsItemsAndKeepsEnvelope(t *testing.T) {
	got := trimXMLItems(rssWithItems(item("one"), item("two"), item("three")), 2)
	if countItems(got) != 2 {
		t.Fatalf("trimmed item count = %d, want 2; xml=%s", countItems(got), got)
	}
	if strings.Contains(got, "<title>three</title>") {
		t.Fatalf("trimmed xml contains third item: %s", got)
	}
	if !strings.Contains(got, "</channel></rss>") {
		t.Fatalf("trimmed xml lost channel/rss close tags: %s", got)
	}
}

func TestTrimXMLItemsNoopsWhenLimitCoversAllOrNonPositive(t *testing.T) {
	xml := rssWithItems(item("one"), item("two"))
	if got := trimXMLItems(xml, 2); got != xml {
		t.Fatalf("trimXMLItems(limit=count) changed xml: %q", got)
	}
	if got := trimXMLItems(xml, 0); got != xml {
		t.Fatalf("trimXMLItems(limit=0) changed xml: %q", got)
	}
}

func TestHasChannel(t *testing.T) {
	if !hasChannel(emptyRSS) {
		t.Fatal("hasChannel(valid rss) = false, want true")
	}
	if hasChannel("<rss></rss>") {
		t.Fatal("hasChannel(xml without channel) = true, want false")
	}
}
