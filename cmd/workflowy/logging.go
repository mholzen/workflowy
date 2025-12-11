package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type simpleHandler struct {
	level  slog.Level
	writer io.Writer
}

func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		fmt.Printf("Error: log level must be one of: debug, info, warn, error\n")
		os.Exit(1)
	}

	handler := &simpleHandler{
		level:  logLevel,
		writer: os.Stderr,
	}
	slog.SetDefault(slog.New(handler))
}

func (h *simpleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *simpleHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	msg := r.Message
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s='%v'", a.Key, a.Value))
		return true
	})
	if len(attrs) > 0 {
		fmt.Fprintf(h.writer, "%s: %s (%s)\n", level, msg, strings.Join(attrs, " "))
	} else {
		fmt.Fprintf(h.writer, "%s: %s\n", level, msg)
	}
	return nil
}

func (h *simpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *simpleHandler) WithGroup(name string) slog.Handler {
	return h
}
