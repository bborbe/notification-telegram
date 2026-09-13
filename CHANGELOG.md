# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## v0.1.1

- fix: bump github.com/bborbe/notification to v0.6.1, which binds the notification controller's initiator to the telegram role. Without it the deployed service rejected every send command — `permissions([discord.send]) does not contains any of permissions([telegram.send telegram.admin])` — so the handler routed the notification correctly and the delivery never happened.

## v0.1.0

- feat: consume the telegram send command and deliver via the Telegram Bot API, logging the accepted `message_id` and chat id
