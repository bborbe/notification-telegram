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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testToken = "123456:test-token-abc"

var _ = Describe("MessageSender", func() {
	var ctx context.Context
	var err error
	var server *httptest.Server
	var receivedBody []byte
	var receivedPath string
	var statusCode int
	var responseBody string
	var sender pkg.MessageSender

	BeforeEach(func() {
		ctx = context.Background()
		receivedBody = nil
		receivedPath = ""
		statusCode = http.StatusOK
		responseBody = `{"ok":true,"result":{"message_id":42,"chat":{"id":112230768}}}`
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			receivedBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_, _ = w.Write([]byte(responseBody))
		}))
		sender = pkg.NewMessageSenderWithBaseURL(testToken, server.Client(), server.URL)
	})
	AfterEach(func() {
		server.Close()
	})

	Context("Send", func() {
		JustBeforeEach(func() {
			err = sender.Send(ctx, "112230768", "hello world")
		})
		Context("success", func() {
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("posts the {chat_id, text} payload to the sendMessage endpoint", func() {
				Expect(receivedPath).To(Equal("/bot" + testToken + "/sendMessage"))
				var payload map[string]string
				Expect(json.Unmarshal(receivedBody, &payload)).To(BeNil())
				Expect(payload).To(Equal(map[string]string{
					"chat_id": "112230768",
					"text":    "hello world",
				}))
			})
		})
		Context("non-2xx status", func() {
			BeforeEach(func() {
				statusCode = http.StatusBadRequest
				responseBody = `{"ok":false,"description":"chat not found"}`
			})
			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
		})
		Context("ok=false", func() {
			BeforeEach(func() {
				responseBody = `{"ok":false,"description":"chat not found"}`
			})
			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
		})
		Context("unreachable server", func() {
			BeforeEach(func() {
				server.Close()
			})
			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
			It("never leaks the token into the error", func() {
				Expect(err.Error()).NotTo(ContainSubstring(testToken))
			})
		})
		Context("invalid chat id", func() {
			JustBeforeEach(func() {
				err = sender.Send(ctx, "", "hello world")
			})
			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
		})
		Context("invalid message", func() {
			JustBeforeEach(func() {
				err = sender.Send(ctx, "112230768", "")
			})
			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
		})
	})

	// Telegram accepts only http, https and tg:// in a text_link entity URL and
	// answers any other scheme with 400 "Unsupported URL protocol", which fails
	// the whole message rather than degrading it. A payload naming an
	// obsidian:// URL in an entity is therefore a delivery outage, not a
	// cosmetic regression — these specs are the guard against re-introducing it.
	Context("an obsidian:// deeplink in the message body", func() {
		// Shaped like a real escalation body: the deeplink is the last line and
		// the line above carries an em dash, so a naive byte offset would be wrong.
		const message = "escalation: pr-reviewer-agent cleared its assignee — status in_progress, phase human_review\n" +
			"obsidian://open?vault=openclaw&file=tasks%2FPR%20Review%20github%20-%202"

		JustBeforeEach(func() {
			err = sender.Send(ctx, "112230768", message)
		})

		It("delivers the message", func() {
			Expect(err).To(BeNil())
		})

		It("attaches no entity whose URL is not http, https or tg", func() {
			var payload struct {
				Entities []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"entities"`
			}
			Expect(json.Unmarshal(receivedBody, &payload)).To(Succeed())
			for _, entity := range payload.Entities {
				Expect(entity.URL).To(Or(
					HavePrefix("http://"),
					HavePrefix("https://"),
					HavePrefix("tg://"),
				), "entity type(%s) carries a scheme the Bot API rejects", entity.Type)
			}
		})

		It("sends no entities field at all", func() {
			var payload map[string]any
			Expect(json.Unmarshal(receivedBody, &payload)).To(Succeed())
			Expect(payload).NotTo(HaveKey("entities"))
		})
	})
})
