// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libhttp "github.com/bborbe/http"
	libkafka "github.com/bborbe/kafka"
	libkv "github.com/bborbe/kv"
	"github.com/bborbe/notification-telegram/pkg"
	"github.com/bborbe/notification-telegram/pkg/factory"
	"github.com/bborbe/notification/db"
	libfactory "github.com/bborbe/notification/factory"
	libmetrics "github.com/bborbe/notification/metrics"
	"github.com/bborbe/run"
	libsentry "github.com/bborbe/sentry"
	"github.com/bborbe/service"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const serviceName = "core-telegram-controller"

func main() {
	app := &application{}
	os.Exit(service.Main(context.Background(), app, &app.SentryDSN, &app.SentryProxy))
}

type application struct {
	SentryDSN      string             `required:"true"  arg:"sentry-dsn"       env:"SENTRY_DSN"       usage:"SentryDSN"                             display:"length"`
	SentryProxy    string             `required:"false" arg:"sentry-proxy"     env:"SENTRY_PROXY"     usage:"Sentry Proxy"`
	Listen         string             `required:"true"  arg:"listen"           env:"LISTEN"           usage:"address to listen to"`
	KafkaBrokers   libkafka.Brokers   `required:"true"  arg:"kafka-brokers"    env:"KAFKA_BROKERS"    usage:"Comma separated list of Kafka brokers"`
	BatchSize      libkafka.BatchSize `required:"true"  arg:"batch-size"       env:"BATCH_SIZE"       usage:"batch consume size"                                     default:"1"`
	DataDir        string             `required:"true"  arg:"datadir"          env:"DATADIR"          usage:"data directory"`
	NoSync         bool               `required:"true"  arg:"no-sync"          env:"NO_SYNC"          usage:"no sync"                                                default:"false"`
	TelegramToken  string             `required:"true"  arg:"telegram-token"   env:"TELEGRAM_TOKEN"   usage:"Telegram bot token"                    display:"length"`
	Branch         base.Branch        `required:"true"  arg:"branch"           env:"BRANCH"           usage:"branch"`
	BuildGitCommit string             `required:"false" arg:"build-git-commit" env:"BUILD_GIT_COMMIT" usage:"Build Git commit hash"                                  default:"none"`
	BuildDate      *libtime.DateTime  `required:"false" arg:"build-date"       env:"BUILD_DATE"       usage:"Build timestamp (RFC3339)"`
}

func (a *application) Run(ctx context.Context, sentryClient libsentry.Client) error {
	libmetrics.NewBuildInfoMetrics().SetBuildInfo(a.BuildDate)

	saramaClientProvider, err := libkafka.NewSaramaClientProviderByType(
		ctx,
		libkafka.SaramaClientProviderTypeReused,
		a.KafkaBrokers,
	)
	if err != nil {
		return errors.Wrapf(ctx, err, "create sarama client provider failed")
	}
	defer saramaClientProvider.Close()

	syncProducer, err := libfactory.NewSyncProducerWithName(
		ctx,
		a.KafkaBrokers,
		serviceName,
	)
	if err != nil {
		return errors.Wrapf(ctx, err, "create sync producer failed")
	}
	defer syncProducer.Close()

	db, err := db.OpenBoltDB(ctx, a.DataDir, a.NoSync)
	if err != nil {
		return errors.Wrapf(ctx, err, "open db failed")
	}
	defer db.Close()

	messageSender := pkg.NewMessageSender(
		a.TelegramToken,
		&http.Client{Timeout: 30 * time.Second},
	)

	return service.Run(
		ctx,
		a.createCommandConsumer(
			saramaClientProvider,
			syncProducer,
			db,
			messageSender,
			sentryClient,
		),
		a.createHTTPServer(db),
	)
}

func (a *application) createCommandConsumer(
	saramaClientProvider libkafka.SaramaClientProvider,
	syncProducer libkafka.SyncProducer,
	db libkv.DB,
	messageSender pkg.MessageSender,
	sentryClient libsentry.Client,
) run.Func {
	return factory.CreateCommandConsumer(
		db,
		saramaClientProvider,
		syncProducer,
		a.Branch,
		a.BatchSize,
		messageSender,
		sentryClient,
	)
}

func (a *application) createHTTPServer(db libkv.DB) run.Func {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		router := mux.NewRouter()
		router.Path("/healthz").Handler(libhttp.NewPrintHandler("OK"))
		router.Path("/readiness").Handler(libhttp.NewPrintHandler("OK"))
		router.Path("/metrics").Handler(promhttp.Handler())
		router.Path("/resetdb").Handler(libkv.NewResetHandler(db, cancel))
		router.Path("/resetbucket/{BucketName}").Handler(libkv.NewResetBucketHandler(db, cancel))
		router.Path("/setloglevel/{level}").Handler(libfactory.CreateSetLoglevelHandler(ctx))
		router.Path("/offsetmanager").Handler(libfactory.CreateOffsetManagerHandler(db, cancel))

		glog.V(2).Infof("starting http server listen on %s", a.Listen)
		return libhttp.NewServer(
			a.Listen,
			router,
		).Run(ctx)
	}
}
