package commands

import (
	"github.com/celestix/gotgproto/dispatcher"
	"go.uber.org/zap"
)

type command struct {
	log *zap.Logger
}

func Load(log *zap.Logger, dispatcher dispatcher.Dispatcher) {
	log = log.Named("commands")
	defer log.Info("Initialized all command handlers")
	commands := &command{log}
	commands.LoadStart(dispatcher)
	commands.LoadStream(dispatcher)
	commands.LoadUserStream(dispatcher)
}
