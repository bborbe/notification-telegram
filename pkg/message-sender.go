// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bborbe/errors"
	"github.com/bborbe/notification/telegram"
	"github.com/golang/glog"
)

const defaultBaseURL = "https://api.telegram.org"

type MessageSender interface {
	Send(ctx context.Context, chatID telegram.ChatID, message telegram.Message) error
}

type MessageSenderFunc func(ctx context.Context, chatID telegram.ChatID, message telegram.Message) error

func (m MessageSenderFunc) Send(
	ctx context.Context,
	chatID telegram.ChatID,
	message telegram.Message,
) error {
	return m(ctx, chatID, message)
}

// telegramPayload is the JSON body sent to the Bot API sendMessage endpoint.
//
// It carries only chat_id and text, and deliberately has no entities field. A
// Telegram text_link entity accepts only an http, https or tg:// URL; any other
// scheme is answered with 400 "Unsupported URL protocol", which fails the whole
// message rather than degrading it. An obsidian:// deeplink is therefore
// delivered as plain text, and attaching an entity for one is not a cosmetic
// regression — it is a delivery outage. Do not add an entities field back
// without a URL scheme the Bot API accepts.
type telegramPayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// telegramSendMessageResponse is the subset of the Bot API response this
// service needs: Telegram echoes the accepted message id and the resolved
// chat, which is what proves delivery (unlike Discord, a bot cannot read its
// own sent messages back through the Telegram API).
type telegramSendMessageResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"result"`
}

// NewMessageSender returns a MessageSender that delivers via the Telegram Bot
// API. The bot token travels in the request URL, so it is redacted from every
// returned error — an http error string otherwise carries the full URL.
func NewMessageSender(token string, httpClient *http.Client) MessageSender {
	return NewMessageSenderWithBaseURL(token, httpClient, defaultBaseURL)
}

// NewMessageSenderWithBaseURL returns a MessageSender using a custom base URL.
// Intended for testing only.
func NewMessageSenderWithBaseURL(
	token string,
	httpClient *http.Client,
	baseURL string,
) MessageSender {
	return MessageSenderFunc(
		func(ctx context.Context, chatID telegram.ChatID, message telegram.Message) error {
			if err := chatID.Validate(ctx); err != nil {
				return errors.Wrapf(ctx, err, "validate chat id failed")
			}
			if err := message.Validate(ctx); err != nil {
				return errors.Wrapf(ctx, err, "validate message failed")
			}

			text := message.String()
			body, err := json.Marshal(telegramPayload{
				ChatID: chatID.String(),
				Text:   text,
			})
			if err != nil {
				return errors.Wrapf(ctx, err, "marshal telegram payload failed")
			}

			url := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, token)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				return errors.Wrapf(
					ctx,
					err,
					"create http request failed: %s",
					redact(err.Error(), token),
				)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			if err != nil {
				return errors.Errorf(
					ctx,
					"execute http request failed: %s",
					redact(err.Error(), token),
				)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return errors.Wrapf(ctx, err, "read response body failed")
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return errors.Errorf(
					ctx,
					"telegram returned non-2xx status(%d): %s",
					resp.StatusCode,
					redact(string(respBody), token),
				)
			}

			var result telegramSendMessageResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return errors.Wrapf(ctx, err, "unmarshal telegram response failed")
			}
			if !result.Ok {
				return errors.Errorf(
					ctx,
					"telegram returned ok=false for chat(%d)",
					result.Result.Chat.ID,
				)
			}

			glog.V(2).Infof(
				"telegram message delivered to chat(%d) with messageId(%d)",
				result.Result.Chat.ID,
				result.Result.MessageID,
			)
			return nil
		},
	)
}

// redact removes the token from a string that may embed it (an http error
// string carries the full request URL, which contains the bot token).
func redact(value string, token string) string {
	if token == "" {
		return value
	}
	return strings.ReplaceAll(value, token, "***")
}
