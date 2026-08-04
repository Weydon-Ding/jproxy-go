package format

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// RadarrXML rewrites direct RSS channel item titles, or returns xmlText
// byte-for-byte unchanged when the Phase 1 formatter cannot safely apply.
func RadarrXML(xmlText string, cfg Config) string {
	if strings.TrimSpace(xmlText) == "" || strings.TrimSpace(cfg.Format) == "" || !strings.Contains(cfg.Format, "{title}") || !strings.Contains(xmlText, "<item") || ValidateConfig(cfg) != nil {
		return xmlText
	}
	items, ok := readItems(xmlText)
	if !ok {
		return xmlText
	}
	decoder := xml.NewDecoder(strings.NewReader(xmlText))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	var itemDepth, elementDepth, titleDepth int
	itemIndex := -1
	var itemTitle string
	var titleStart xml.StartElement
	var replacingTitle, changed bool
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return xmlText
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "item" {
				itemDepth++
				elementDepth = 1
				itemIndex++
			} else if itemDepth > 0 {
				elementDepth++
			}
			if itemDepth > 0 && elementDepth == 2 && value.Name.Local == "title" && titleDepth == 0 {
				titleDepth++
				titleStart, replacingTitle = value, true
				continue
			}
		case xml.CharData:
			if titleDepth > 0 {
				itemTitle += string(value)
				continue
			}
		case xml.EndElement:
			if titleDepth > 0 && value.Name.Local == "title" {
				formatted, ok := formatItemTitle(itemTitle, items[itemIndex].description, cfg)
				if !ok {
					formatted = itemTitle
				} else if formatted != itemTitle {
					changed = true
				}
				if err := encodeTitle(encoder, titleStart, formatted, value); err != nil {
					return xmlText
				}
				titleDepth--
				elementDepth--
				replacingTitle = false
				continue
			}
			if value.Name.Local == "item" {
				itemDepth--
				elementDepth = 0
				itemTitle = ""
			} else if itemDepth > 0 {
				elementDepth--
			}
		}
		if replacingTitle {
			continue
		}
		if err := encoder.EncodeToken(token); err != nil {
			return xmlText
		}
	}
	if err := encoder.Flush(); err != nil || !changed {
		return xmlText
	}
	return output.String()
}

func encodeTitle(encoder *xml.Encoder, start xml.StartElement, text string, end xml.EndElement) error {
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(text)); err != nil {
		return err
	}
	return encoder.EncodeToken(end)
}

type itemText struct{ description string }

func readItems(xmlText string) ([]itemText, bool) {
	decoder := xml.NewDecoder(strings.NewReader(xmlText))
	var items []itemText
	var itemDepth, elementDepth int
	var descriptionDepth int
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return items, true
		}
		if err != nil {
			return nil, false
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "item" {
				itemDepth++
				elementDepth = 1
				items = append(items, itemText{})
			} else if itemDepth > 0 {
				elementDepth++
			}
			if itemDepth > 0 && elementDepth == 2 && value.Name.Local == "description" {
				descriptionDepth++
			}
		case xml.CharData:
			if descriptionDepth > 0 {
				items[len(items)-1].description += string(value)
			}
		case xml.EndElement:
			if value.Name.Local == "description" && descriptionDepth > 0 {
				descriptionDepth--
			}
			if value.Name.Local == "item" {
				itemDepth--
				elementDepth = 0
			} else if itemDepth > 0 {
				elementDepth--
			}
		}
	}
}
