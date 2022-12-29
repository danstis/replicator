package dbot

import (
	"context"
	"log"
	"strconv"

	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/autocomplete"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/command"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/component"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/delete"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/edit"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/message"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/modal"
	"github.com/danstis/replicator/pkg/logging"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap/zapcore"
)

type Bot struct {
	// State contains the bot state.
	State *state.State

	// log is used to log messages using zap and the otel logger.
	log *otelzap.SugaredLogger
}

// Connect creates a connection to the discord server.
func (b *Bot) Connect(ctx context.Context, token string) {
	ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.Connect")
	defer span.End()
	var err error
	b.log, err = logging.InitLogger(zapcore.InfoLevel)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	b.State = state.New("Bot " + token)

	addIntents(ctx, b.State)

	b.State.AddHandler(func(e *gateway.GuildCreateEvent) {
		command.ClearObsoleteCommands(ctx, b.State, e.ID)
	})

	command.AddHandler(ctx, b.State)
	autocomplete.AddHandler(b.State)
	modal.AddHandler(b.State)
	component.AddHandler(b.State)
	message.AddHandler(b.State)
	delete.AddHandler(b.State)
	edit.AddHandler(b.State)

	b.log.Ctx(ctx).Infof("connecting to discord")
	if err := b.State.Open(context.Background()); err != nil {
		b.log.Ctx(ctx).Fatalw(
			"failed to connect to discord",
			"error", err,
		)
	}

	b.log.Ctx(ctx).Infof("getting bot discord user")
	u, err := b.State.Me()
	if err != nil {
		b.log.Ctx(ctx).Fatalw(
			"failed to get bot user",
			"error", err,
		)
	}
	b.log.Ctx(ctx).Infow(
		"connected to discord",
		"username", u.Username,
		"discriminator", u.Discriminator,
	)

	if err := command.RegisterCommands(ctx, b.State); err != nil {
		b.log.Ctx(ctx).Fatalw(
			"failed to register commands",
			"error", err,
		)
	}
}

func addIntents(ctx context.Context, s *state.State) {
	_, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.addIntents")
	defer span.End()
	s.AddIntents(
		gateway.IntentGuilds |
			gateway.IntentGuildEmojis |
			gateway.IntentGuildMessages |
			gateway.IntentGuildMessageReactions |
			gateway.IntentGuildMessageTyping |
			gateway.IntentDirectMessages |
			gateway.IntentDirectMessageReactions |
			gateway.IntentDirectMessageTyping,
	)
}

// HexColourToInt converts a hex color string (#121212) to an int colour.
func HexColourToInt(ctx context.Context, color string) (int64, error) {
	_, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.HexColourToInt")
	defer span.End()
	return strconv.ParseInt((color)[1:], 16, 64)
}

// PostMessage creates an embed in the defined channel.
//
// embed should be an embed in the following format:
//
//	discord.Embed{
//		    Title:       "Some Title",
//		    Description: "Body Text",
//		    Footer:      &discord.EmbedFooter{Text: "Footer Text"},
//		    Timestamp:   timestamp,
//	}
func (b *Bot) PostMessage(ctx context.Context, colour string, message string, embed discord.Embed, channel discord.ChannelID) {
	ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.PostMessage")
	defer span.End()
	// Parse the color into decimal numbers.
	colorHex, err := HexColourToInt(ctx, colour)
	if err != nil {
		b.log.Ctx(ctx).Errorw(
			"failed to convert color",
			"error", err,
		)
	}

	embed.Color = discord.Color(colorHex)

	_, err = b.State.SendMessage(channel, message, embed)
	if err != nil {
		b.log.Ctx(ctx).Errorw(
			"failed to post message",
			"channel", channel,
			"title", embed.Title,
			"footer", embed.Footer.Text,
			"colour", embed.Color,
			"error", err,
		)
	}
}
