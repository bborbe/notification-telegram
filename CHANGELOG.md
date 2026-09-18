# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## Unreleased

- feat: add `TELEGRAM_BOT`, naming the bot this instance serves. The send executor skips any command naming a different bot, returning `cdb.ErrCommandObjectSkipped` so the skip publishes neither a Success the instance never performed nor a Failure for every message belonging to the other bot. Empty (the default) serves commands that carry no bot, so a single-instance deployment is unchanged.
- feat: bump `github.com/bborbe/notification` to v0.7.0 for the `Bot` field on `SendCommand`.
- docs: correct `CreateCommandConsumer`'s doc comment, which claimed a consumer group is derived from the schema. There is none — offsets are tracked per instance, so every instance reads every message, which is why the bot filter is what keeps two instances from both delivering.

## v0.1.1

- fix: bump github.com/bborbe/notification to v0.6.1, which binds the notification controller's initiator to the telegram role. Without it the deployed service rejected every send command — `permissions([discord.send]) does not contains any of permissions([telegram.send telegram.admin])` — so the handler routed the notification correctly and the delivery never happened.

## v0.1.0

- feat: consume the telegram send command and deliver via the Telegram Bot API, logging the accepted `message_id` and chat id
