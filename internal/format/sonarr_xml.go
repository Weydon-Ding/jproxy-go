package format

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// SonarrXML rewrites direct RSS channel item titles, or returns xmlText
// byte-for-byte unchanged when the static formatter cannot safely apply.
func SonarrXML(xmlText string, cfg SonarrConfig) string {
	if strings.TrimSpace(xmlText) == "" || !strings.Contains(xmlText, "<item") || ValidateSonarrConfig(cfg) != nil {
		return xmlText
	}
	items, ok := readSonarrItems(xmlText)
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
	var elements []xml.Name
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
			isChannelItem := value.Name.Local == "item" && len(elements) > 0 && elements[len(elements)-1].Local == "channel"
			if isChannelItem {
				itemDepth++
				elementDepth = 1
				itemIndex++
			} else if itemDepth > 0 {
				elementDepth++
			}
			if itemDepth > 0 && elementDepth == 2 && value.Name.Local == "title" && titleDepth == 0 {
				titleDepth++
				titleStart, replacingTitle = value, true
				elements = append(elements, value.Name)
				continue
			}
			elements = append(elements, value.Name)
		case xml.CharData:
			if titleDepth > 0 {
				itemTitle += string(value)
				continue
			}
		case xml.EndElement:
			if titleDepth > 0 && value.Name.Local == "title" {
				formatted, ok := formatSonarrItemTitle(itemTitle, items[itemIndex].description, cfg)
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
				elements = elements[:len(elements)-1]
				continue
			}
			isChannelItem := value.Name.Local == "item" && len(elements) > 1 && elements[len(elements)-2].Local == "channel"
			if isChannelItem {
				itemDepth--
				elementDepth = 0
				itemTitle = ""
			} else if itemDepth > 0 {
				elementDepth--
			}
			elements = elements[:len(elements)-1]
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

func readSonarrItems(xmlText string) ([]itemText, bool) {
	decoder := xml.NewDecoder(strings.NewReader(xmlText))
	var items []itemText
	var elements []xml.Name
	var itemDepth, elementDepth, descriptionDepth int
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
			isChannelItem := value.Name.Local == "item" && len(elements) > 0 && elements[len(elements)-1].Local == "channel"
			if isChannelItem {
				itemDepth++
				elementDepth = 1
				items = append(items, itemText{})
			} else if itemDepth > 0 {
				elementDepth++
			}
			if itemDepth > 0 && elementDepth == 2 && value.Name.Local == "description" {
				descriptionDepth++
			}
			elements = append(elements, value.Name)
		case xml.CharData:
			if descriptionDepth > 0 {
				items[len(items)-1].description += string(value)
			}
		case xml.EndElement:
			if value.Name.Local == "description" && descriptionDepth > 0 {
				descriptionDepth--
			}
			isChannelItem := value.Name.Local == "item" && len(elements) > 1 && elements[len(elements)-2].Local == "channel"
			if isChannelItem {
				itemDepth--
				elementDepth = 0
			} else if itemDepth > 0 {
				elementDepth--
			}
			elements = elements[:len(elements)-1]
		}
	}
}
