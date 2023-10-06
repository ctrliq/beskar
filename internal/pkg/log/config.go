// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"log/slog"
	"os"
)

type Config struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func (c *Config) Logger(handlerWrapper func(handler slog.Handler) slog.Handler) (*slog.Logger, error) {
	var handler slog.Handler
	var opts slog.HandlerOptions

	switch c.Level {
	case "debug":
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	case "info":
		opts.Level = slog.LevelInfo
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		return nil, fmt.Errorf("unknown log level %s", c.Level)
	}

	switch c.Format {
	case "text":
		handler = slog.NewTextHandler(os.Stderr, &opts)
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, &opts)
	default:
		return nil, fmt.Errorf("unknown log format %s", c.Format)
	}

	if handlerWrapper != nil {
		return slog.New(handlerWrapper(handler)), nil
	}

	return slog.New(handler), nil
}
