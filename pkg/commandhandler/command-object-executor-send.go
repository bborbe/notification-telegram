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
	"github.com/golang/glog"
)

func NewSendCommandObjectExecutor(
	permissionChecker cqrsiam.PermissionChecker,
	messageSender pkg.MessageSender,
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
			if err := messageSender.Send(ctx, sendCommand.ChatID, sendCommand.Message); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "send telegram message failed")
			}
			glog.V(2).Infof("send telegram message completed")
			return commandObject.Command.ID.Ptr(), event, nil
		},
	)
}
