// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"log/slog"
)

var (
	loggerKey int
	attrKey   int
)

type Handler struct {
	slog.Handler
}

func ContextHandler(handler slog.Handler) slog.Handler {
	return &Handler{Handler: handler}
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if attrs, ok := ctx.Value(&attrKey).([]slog.Attr); ok {
		record.AddAttrs(attrs...)
	}
	return h.Handler.Handle(ctx, record)
}

func SetContextLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, &loggerKey, logger)
}

func GetContextLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(&loggerKey).(*slog.Logger)
	if ok {
		return logger
	}
	return nil
}

func SetContextAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if contextAttrs, ok := ctx.Value(&attrKey).([]slog.Attr); ok {
		contextAttrs = append(contextAttrs, attrs...)
		return context.WithValue(ctx, &attrKey, contextAttrs)
	}
	return context.WithValue(ctx, &attrKey, attrs)
}
