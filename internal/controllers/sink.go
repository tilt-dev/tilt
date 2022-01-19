package controllers

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"k8s.io/apiserver/pkg/registry/generic/registry"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// loosely adapted from funcr.Logger
type logSink struct {
	funcr.Formatter

	ctx    context.Context
	logger logger.Logger
}

func (l logSink) WithName(name string) logr.LogSink {
	l.Formatter.AddName(name)
	return &l
}

func (l logSink) WithValues(kvList ...interface{}) logr.LogSink {
	l.Formatter.AddValues(kvList)
	return &l
}

func (l logSink) WithCallDepth(depth int) logr.LogSink {
	l.Formatter.AddCallDepth(depth)
	return &l
}

func (l logSink) Info(level int, msg string, kvList ...interface{}) {
	if l.ctx.Err() != nil {
		return // Stop logging when the context is cancelled.
	}

	// We don't care about the startup or teardown sequence.
	if msg == "Starting EventSource" ||
		msg == "Starting Controller" ||
		msg == "Starting workers" ||
		msg == "error received after stop sequence was engaged" ||
		msg == "Shutdown signal received, waiting for all workers to finish" ||
		msg == "All workers finished" {
		return
	}

	prefix, args := l.FormatInfo(level, msg, kvList)

	// V(3) was picked because while controller-runtime is a bit chatty at
	// startup, once steady state is reached, most of the logging is generally
	// useful.
	if level < 3 {
		l.logger.Debugf("[%s] %s", prefix, args)
		return
	}

	l.logger.Infof("[%s] %s", prefix, args)
}

func (l logSink) Error(err error, msg string, kvList ...interface{}) {
	if l.ctx.Err() != nil {
		return // Stop logging when the context is cancelled.
	}

	// It's normal for reconcilers to fail with optimistic lock errors.
	// They'll just retry.
	if strings.Contains(err.Error(), registry.OptimisticLockErrorMsg) {
		return
	}

	// Print errors to the global log on all builds.
	//
	// TODO(nick): Once we have resource grouping, we should consider having some
	// some sort of system resource that we can hide by default and print
	// these kinds of apiserver problems to.
	prefix, args := l.FormatError(err, msg, kvList)
	l.logger.Errorf("[%s] %s", prefix, args)
}
