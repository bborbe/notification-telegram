// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commandhandler

import (
	"context"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	cqrsiam "github.com/bborbe/cqrs/iam"
	"github.com/bborbe/errors"
	libkv "github.com/bborbe/kv"
	"github.com/bborbe/notification-telegram/pkg"
	telegramcommand "github.com/bborbe/notification/command/telegram"
	"github.com/bborbe/notification/iam"
	"github.com/bborbe/notification/telegram"
	"github.com/golang/glog"
)

// NewSendCommandObjectExecutor returns the executor that delivers telegram send
// commands through messageSender.
//
// bot names the bot this instance serves. Every instance reads every message on
// the topic (offsets are per-instance, there is no consumer group), so the
// executor drops any command naming a different bot — that filter is what keeps
// two instances from both delivering the same message. The empty bot serves
// commands that carry no bot, which is every command written before the field
// existed.
func NewSendCommandObjectExecutor(
	permissionChecker cqrsiam.PermissionChecker,
	messageSender pkg.MessageSender,
	bot telegram.Bot,
) cdb.CommandObjectExecutorTx {
	return cdb.CommandObjectExecutorTxFunc(
		telegramcommand.SendCommandOperation,
		true,
		func(ctx context.Context, tx libkv.Tx, commandObject cdb.CommandObject) (*base.EventID, base.Event, error) {
			glog.V(2).Infof("send telegram message started")

			permissionCheck := iam.NewAnyPermissionCheck(
				iam.CoreTelegramSendPermission,
				iam.CoreTelegramAdminPermission,
			)
			if err := permissionChecker.Check(ctx, tx, commandObject.Command.Initiator, permissionCheck); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "permission denied")
			}

			event := commandObject.Command.Data
			var sendCommand telegramcommand.SendCommand
			if err := event.MarshalInto(ctx, &sendCommand); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "marshal into sendCommand failed")
			}
			if err := sendCommand.Validate(ctx); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "validate sendCommand failed")
			}
			if sendCommand.Bot != bot {
				// Not this instance's bot. Skipped rather than nil or an error:
				// nil would publish a Success the instance never performed, and
				// an error would publish a Failure for every message belonging
				// to the other bot — which is most of them.
				glog.V(2).Infof(
					"send telegram message skipped: command names bot(%s), this instance serves bot(%s)",
					sendCommand.Bot,
					bot,
				)
				return nil, nil, cdb.ErrCommandObjectSkipped
			}
			if err := messageSender.Send(ctx, sendCommand.ChatID, sendCommand.Message); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "send telegram message failed")
			}
			glog.V(2).Infof("send telegram message completed")
			return commandObject.Command.ID.Ptr(), event, nil
		},
	)
}
