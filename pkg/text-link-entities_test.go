// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/bborbe/notification-telegram/pkg"
	"github.com/bborbe/notification/telegram"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sentPayload mirrors the subset of the sendMessage body these specs assert on.
// Entities is a pointer so the specs can distinguish "absent" from "empty": the
// field is omitempty, and a message with no obsidian:// URL must serialize
// exactly as it did before entities existed.
type sentPayload struct {
	ChatID   string `json:"chat_id"`
	Text     string `json:"text"`
	Entities *[]struct {
		Type   string `json:"type"`
		Offset int    `json:"offset"`
		Length int    `json:"length"`
		URL    string `json:"url"`
	} `json:"entities"`
}

var _ = Describe("text_link entities", func() {
	var ctx context.Context
	var server *httptest.Server
	var receivedBody []byte
	var sender pkg.MessageSender

	// sendAndDecode posts message through the real sender and returns the body
	// the Bot API would have received. Asserting on the wire payload rather than
	// on the helper is deliberate: the helper is unexported and the test package
	// is external, so this is the only path that also proves the field is wired
	// into the request.
	sendAndDecode := func(message string) sentPayload {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"chat":{"id":112230768}}}`))
		}))
		defer server.Close()
		sender = pkg.NewMessageSenderWithBaseURL(testToken, server.Client(), server.URL)
		Expect(sender.Send(ctx, "112230768", telegram.Message(message))).To(BeNil())
		var payload sentPayload
		Expect(json.Unmarshal(receivedBody, &payload)).To(BeNil())
		return payload
	}

	BeforeEach(func() {
		ctx = context.Background()
		receivedBody = nil
	})

	Context("a message carrying an obsidian:// deeplink", func() {
		// Shaped like a real escalation body: the deeplink is the last line, and
		// the line above it contains an em dash, so a byte offset would be wrong.
		const message = "escalation: pr-reviewer-agent cleared its assignee — status in_progress, phase human_review\n" +
			"obsidian://open?vault=openclaw&file=tasks%2FPR%20Review%20github%20-%202"

		It("attaches exactly one text_link entity", func() {
			payload := sendAndDecode(message)
			Expect(payload.Entities).NotTo(BeNil())
			Expect(*payload.Entities).To(HaveLen(1))
			Expect((*payload.Entities)[0].Type).To(Equal("text_link"))
		})

		It("points the entity at the deeplink itself", func() {
			payload := sendAndDecode(message)
			Expect((*payload.Entities)[0].URL).To(Equal(
				"obsidian://open?vault=openclaw&file=tasks%2FPR%20Review%20github%20-%202",
			))
			Expect(
				(*payload.Entities)[0].Length,
			).To(Equal(len("obsidian://open?vault=openclaw&file=tasks%2FPR%20Review%20github%20-%202")))
		})

		It("measures the offset in UTF-16 code units, not bytes", func() {
			payload := sendAndDecode(message)
			prefix := "escalation: pr-reviewer-agent cleared its assignee — status in_progress, phase human_review\n"
			// The em dash is one UTF-16 unit but three bytes, so a byte offset
			// would report len(prefix) and land the entity two characters late.
			Expect((*payload.Entities)[0].Offset).To(Equal(utf16Units(prefix)))
			Expect((*payload.Entities)[0].Offset).NotTo(Equal(len(prefix)))
		})

		It("still sends the message text verbatim", func() {
			payload := sendAndDecode(message)
			Expect(payload.Text).To(Equal(message))
		})
	})

	Context("a message carrying no obsidian:// deeplink", func() {
		It("omits the entities field entirely", func() {
			payload := sendAndDecode("hello world")
			Expect(payload.Entities).To(BeNil())
		})

		It("leaves the http URL Telegram already auto-detects alone", func() {
			payload := sendAndDecode("see https://example.com/x for details")
			Expect(payload.Entities).To(BeNil())
		})
	})

	Context("offset unit", func() {
		It("counts a rune outside the BMP as two UTF-16 units", func() {
			// The emoji is one rune, two UTF-16 units, four bytes. Runes and
			// bytes both give the wrong answer, so this is the case that pins the
			// unit the Bot API actually specifies.
			payload := sendAndDecode("😀\nobsidian://x")
			Expect(payload.Entities).NotTo(BeNil())
			Expect((*payload.Entities)[0].Offset).To(Equal(3))
		})
	})

	Context("a message carrying two deeplinks", func() {
		It("attaches one entity per deeplink, in order", func() {
			payload := sendAndDecode("obsidian://a\nmid\nobsidian://b")
			Expect(payload.Entities).NotTo(BeNil())
			Expect(*payload.Entities).To(HaveLen(2))
			Expect((*payload.Entities)[0].URL).To(Equal("obsidian://a"))
			Expect((*payload.Entities)[1].URL).To(Equal("obsidian://b"))
			Expect((*payload.Entities)[1].Offset).To(Equal(utf16Units("obsidian://a\nmid\n")))
		})
	})
})

// utf16Units is an independent count of UTF-16 code units, written here rather
// than reusing the production helper so the offset assertion is not a tautology.
func utf16Units(s string) int {
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
