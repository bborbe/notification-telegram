// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"time"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	cqrsiam "github.com/bborbe/cqrs/iam"
	libkafka "github.com/bborbe/kafka"
	libkv "github.com/bborbe/kv"
	core "github.com/bborbe/notification"
	"github.com/bborbe/notification-telegram/pkg"
	"github.com/bborbe/notification-telegram/pkg/commandhandler"
	"github.com/bborbe/run"
	libsentry "github.com/bborbe/sentry"
)

// CreateCommandConsumer consumes the telegram send commands and executes them
// through the send executor. The consumer group is derived by the cdb
// consumer from the schema, so no group is passed here.
func CreateCommandConsumer(
	db libkv.DB,
	saramaClientProvider libkafka.SaramaClientProvider,
	syncProducer libkafka.SyncProducer,
	branch base.Branch,
	batchSize libkafka.BatchSize,
	messageSender pkg.MessageSender,
	sentryClient libsentry.Client,
) run.Func {
	permissionChecker := cqrsiam.NewPermissionChecker(
		sentryClient,
		cqrsiam.NewPermissionCheckerMetrics(),
	)
	return cdb.RunCommandConsumerTx(
		saramaClientProvider,
		syncProducer,
		db,
		core.TelegramV1SchemaID,
		batchSize,
		base.TopicPrefixFromBranch(branch),
		false,
		24*time.Hour,
		run.NewTrigger(),
		cdb.CommandObjectExecutorTxs{
			commandhandler.NewSendCommandObjectExecutor(
				permissionChecker,
				messageSender,
			),
		},
	)
}
