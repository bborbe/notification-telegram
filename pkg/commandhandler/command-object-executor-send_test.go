// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commandhandler_test

import (
	"context"
	"errors"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	cqrsmocks "github.com/bborbe/cqrs/mocks"
	"github.com/bborbe/notification-telegram/pkg"
	"github.com/bborbe/notification-telegram/pkg/commandhandler"
	"github.com/bborbe/notification/telegram"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SendCommandObjectExecutor", func() {
	var ctx context.Context
	var err error
	var permissionChecker *cqrsmocks.IAMPermissionChecker
	var sentChatIDs []telegram.ChatID
	var messageSender pkg.MessageSender
	var executor cdb.CommandObjectExecutorTx
	// instanceBot is the bot the running instance serves; commandBot is the bot
	// named by the incoming command. The filter compares exactly these two.
	var instanceBot telegram.Bot
	var commandBot telegram.Bot

	BeforeEach(func() {
		ctx = context.Background()
		permissionChecker = &cqrsmocks.IAMPermissionChecker{}
		sentChatIDs = nil
		messageSender = pkg.MessageSenderFunc(
			func(ctx context.Context, chatID telegram.ChatID, message telegram.Message) error {
				sentChatIDs = append(sentChatIDs, chatID)
				return nil
			},
		)
		instanceBot = ""
		commandBot = ""
	})

	JustBeforeEach(func() {
		executor = commandhandler.NewSendCommandObjectExecutor(
			permissionChecker,
			messageSender,
			instanceBot,
		)
		event := base.Event{
			"chatId":  "112230768",
			"message": "hello world",
		}
		if commandBot != "" {
			event["bot"] = commandBot.String()
		}
		_, _, err = executor.HandleCommand(ctx, nil, cdb.CommandObject{
			Command: base.Command{
				ID:        "event-id",
				Operation: "send",
				Data:      event,
			},
		})
	})

	Context("instance serves the default bot", func() {
		BeforeEach(func() {
			instanceBot = ""
		})
		Context("command names no bot", func() {
			BeforeEach(func() {
				commandBot = ""
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("delivers the message", func() {
				Expect(sentChatIDs).To(HaveLen(1))
				Expect(sentChatIDs[0]).To(Equal(telegram.ChatID("112230768")))
			})
		})
		Context("command names another bot", func() {
			BeforeEach(func() {
				commandBot = "noise"
			})
			It("returns ErrCommandObjectSkipped", func() {
				Expect(errors.Is(err, cdb.ErrCommandObjectSkipped)).To(BeTrue())
			})
			It("delivers nothing", func() {
				Expect(sentChatIDs).To(BeEmpty())
			})
		})
	})

	Context("instance serves the noise bot", func() {
		BeforeEach(func() {
			instanceBot = "noise"
		})
		Context("command names that bot", func() {
			BeforeEach(func() {
				commandBot = "noise"
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("delivers the message", func() {
				Expect(sentChatIDs).To(HaveLen(1))
			})
		})
		Context("command names no bot", func() {
			BeforeEach(func() {
				commandBot = ""
			})
			It("returns ErrCommandObjectSkipped", func() {
				Expect(errors.Is(err, cdb.ErrCommandObjectSkipped)).To(BeTrue())
			})
			It("delivers nothing", func() {
				Expect(sentChatIDs).To(BeEmpty())
			})
		})
	})

	// The pair below is the property the whole two-instance split rests on:
	// every command is delivered by exactly one instance, never zero and never
	// both. Asserting only the positive half would pass on a build with no
	// filter at all, which would deliver every message twice.
	Context("two instances seeing the same command", func() {
		It("delivers exactly once across the pair", func() {
			for _, c := range []telegram.Bot{"", "noise"} {
				delivered := 0
				for _, instance := range []telegram.Bot{"", "noise"} {
					sent := 0
					sender := pkg.MessageSenderFunc(
						func(ctx context.Context, chatID telegram.ChatID, message telegram.Message) error {
							sent++
							return nil
						},
					)
					event := base.Event{"chatId": "112230768", "message": "hello world"}
					if c != "" {
						event["bot"] = c.String()
					}
					_, _, execErr := commandhandler.NewSendCommandObjectExecutor(
						&cqrsmocks.IAMPermissionChecker{},
						sender,
						instance,
					).HandleCommand(ctx, nil, cdb.CommandObject{
						Command: base.Command{
							ID:        "event-id",
							Operation: "send",
							Data:      event,
						},
					})
					if sent == 0 {
						Expect(errors.Is(execErr, cdb.ErrCommandObjectSkipped)).To(BeTrue())
					}
					delivered += sent
				}
				Expect(delivered).To(Equal(1), "bot %q must be delivered exactly once", c)
			}
		})
	})
})
