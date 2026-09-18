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
	"github.com/bborbe/notification/telegram"
	"github.com/bborbe/run"
	libsentry "github.com/bborbe/sentry"
)

// CreateCommandConsumer consumes the telegram send commands and executes them
// through the send executor.
//
// There is no Kafka consumer group: the cdb consumer tracks offsets in this
// instance's own database, so every instance reads every message on the topic
// rather than competing for them. Running a second instance therefore delivers
// each message twice unless the instances disagree about which they own, which
// is what bot is for — it is passed to the executor, which skips any command
// naming a different bot.
func CreateCommandConsumer(
	db libkv.DB,
	saramaClientProvider libkafka.SaramaClientProvider,
	syncProducer libkafka.SyncProducer,
	branch base.Branch,
	batchSize libkafka.BatchSize,
	messageSender pkg.MessageSender,
	sentryClient libsentry.Client,
	bot telegram.Bot,
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
				bot,
			),
		},
	)
}
