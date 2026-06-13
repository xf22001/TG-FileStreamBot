package bot

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/commands"
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
)

var Bot *gotgproto.Client

func StartClient(log *zap.Logger) (*gotgproto.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resultChan := make(chan struct {
		client *gotgproto.Client
		err    error
	})
	go func(ctx context.Context) {
		client, err := gotgproto.NewClient(
			int(config.ValueOf.ApiID),
			config.ValueOf.ApiHash,
			gotgproto.ClientTypeBot(config.ValueOf.BotToken),
			&gotgproto.ClientOpts{
				Session: sessionMaker.SqlSession(
					sqlite.Open("fsb.session"),
				),
				Resolver: dcs.Plain(dcs.PlainOptions{
					Dial: config.ValueOf.GetDialer(),
				}),
				DisableCopyright: true,
			},
		)
		resultChan <- struct {
			client *gotgproto.Client
			err    error
		}{client, err}
	}(ctx)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		if result.err != nil {
			return nil, result.err
		}
		commands.Load(log, result.client.Dispatcher)
		log.Info("Client started", zap.String("username", result.client.Self.Username))

		// Clear and register bot commands
		go func() {
			time.Sleep(2 * time.Second) // Small delay to ensure everything is settled
			_, err := result.client.API().BotsSetBotCommands(context.Background(), &tg.BotsSetBotCommandsRequest{
				Commands: []tg.BotCommand{
					{Command: "start", Description: "Start the bot and get help"},
				},
				Scope:    &tg.BotCommandScopeDefault{},
				LangCode: "",
			})
			if err != nil {
				log.Error("Failed to register bot commands", zap.Error(err))
			} else {
				log.Info("Bot commands (menu) registered successfully")
			}
		}()

		Bot = result.client
		return result.client, nil
	}
}
