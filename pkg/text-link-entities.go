// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import "regexp"

// obsidianURLPattern matches an obsidian:// deeplink up to the next whitespace.
//
// Telegram auto-detects http and https URLs and renders them as links, but it
// does not recognise a custom scheme: an obsidian:// URL is delivered as plain
// text and is not tappable. Attaching a text_link entity naming the span is the
// only way to make one tappable.
var obsidianURLPattern = regexp.MustCompile(`obsidian://\S+`)

// textLinkEntity is a Telegram message entity rendering a span of the message
// text as a tappable link. Offset and Length are measured in UTF-16 code units
// from the start of the text — the unit the Bot API requires, which is neither
// bytes nor runes.
type textLinkEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url"`
}

// buildTextLinkEntities returns one text_link entity per obsidian:// URL found
// in text, so a delivered escalation deeplink is tappable rather than inert.
//
// A message carrying no obsidian:// URL yields no entities, and the caller
// omits the field, leaving the payload byte-identical to what it was before
// this existed.
func buildTextLinkEntities(text string) []textLinkEntity {
	matches := obsidianURLPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	entities := make([]textLinkEntity, 0, len(matches))
	for _, match := range matches {
		entities = append(entities, textLinkEntity{
			Type:   "text_link",
			Offset: utf16Length(text[:match[0]]),
			Length: utf16Length(text[match[0]:match[1]]),
			URL:    text[match[0]:match[1]],
		})
	}
	return entities
}

// utf16Length returns the number of UTF-16 code units in s, the unit the Bot
// API measures entity offsets in. A rune outside the Basic Multilingual Plane
// occupies two units, so neither len(s) nor the rune count is correct here.
func utf16Length(s string) int {
	units := 0
	for _, r := range s {
		if r > 0xFFFF {
			units += 2
			continue
		}
		units++
	}
	return units
}
