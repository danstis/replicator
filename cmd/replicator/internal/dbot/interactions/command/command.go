package command

import (
	"context"
	"log"

	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/response"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/utility"
	"github.com/danstis/replicator/pkg/logging"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap/zapcore"
)

type Command func(
	state *state.State,
	event *gateway.InteractionCreateEvent,
	command *discord.CommandInteraction,
) Response

type Response struct {
	Response api.InteractionResponse
	Callback func(message *discord.Message)
}

// IsEphemeral checks if the contained InteractionResponse is only shown to the user initiating the interaction.
func (cr *Response) IsEphemeral() bool {
	return cr.Response.Data.Flags&api.EphemeralResponse != 0
}

// Length returns the number of runes in the content string. This is not the same as the number of bytes!
func (cr *Response) Length() int {
	if cr.Response.Data.Content == nil {
		return 0
	}
	runes := []rune(cr.Response.Data.Content.Val)
	return len(runes)
}

type Handler struct {
	Description string
	Code        Command
	Type        discord.CommandType
	Options     []discord.CommandOption
	// TODO: handle command permissions.
	// https://discord.com/developers/docs/interactions/application-commands#application-command-permissions-object
	// also https://github.com/diamondburned/arikawa/blob/v3/discord/application.go
}

// log is the zap logger for this module.
var logger *otelzap.SugaredLogger

func init() {
	var err error
	logger, err = logging.InitLogger(zapcore.InfoLevel)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
}

// commands holds the Commands to be registered with each joined guild.
var commands = map[string]Handler{}

// The Token Bins. 5 and 10 are arbitrary numbers, and it decrements at 10 second intervals.
var userTokenBin = &utility.TokenBin{Max: 5, Interval: 10}
var channelTokenBin = &utility.TokenBin{Max: 10, Interval: 10}

func Register(name string, command Handler) {
	commands[name] = command
}

// AddHandler adds handler for commands, but also the GuildCreate event for command registration.
func AddHandler(ctx context.Context, state *state.State) {
	ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.command.AddHandler")
	defer span.End()

	state.AddHandler(func(e *gateway.InteractionCreateEvent) {
		if interaction, ok := e.Data.(*discord.CommandInteraction); ok {
			if e.GuildID == discord.NullGuildID || e.Member == nil { // Command issued in private
				state.RespondInteraction(e.ID, e.Token, response.Ephemeral("I'm sorry, I do not respond to commands in private."))
				return
			}
			if !userTokenBin.Allocate(discord.Snowflake(e.GuildID), discord.Snowflake(e.Member.User.ID)) {
				if err := state.RespondInteraction(e.ID, e.Token, response.Ephemeral("You are using too many commands too quickly. Slow down.")); err != nil {
					logger.Ctx(ctx).Errorw(
						"an error occurred posting throttle warning ephemeral response (user)",
						"username", e.Member.User.Username,
						"user id", e.Member.User.ID,
						"error", err,
					)
				}
				return
			}
			if !channelTokenBin.Allocate(discord.Snowflake(e.GuildID), discord.Snowflake(e.ChannelID)) {
				if err := state.RespondInteraction(e.ID, e.Token, response.Ephemeral("Too many commands being processed in this channel right now. Please wait.")); err != nil {
					logger.Ctx(ctx).Errorw(
						"an error occurred posting throttle warning ephemeral response (channel)",
						"channel id", e.ChannelID,
						"error", err,
					)
				}
				return
			}

			if val, ok := commands[interaction.Name]; ok {
				resp := val.Code(state, e, interaction)

				if resp.Length() > 1500 {
					if resp.IsEphemeral() {
						resp.Response.Data.Content = option.NewNullableString(resp.Response.Data.Content.Val)
					}
				}

				if err := state.RespondInteraction(e.ID, e.Token, resp.Response); err != nil {
					logger.Ctx(ctx).Errorw(
						"failed to send command interaction response",
						"guild id", e.GuildID,
						"error", err,
					)
				}
				if resp.Callback != nil {
					message, err := state.InteractionResponse(e.AppID, e.Token)
					if err != nil {
						logger.Ctx(ctx).Errorw(
							"failed to get message reference for command callback",
							"interaction name", interaction.Name,
							"error", err,
						)
						return
					}
					if message != nil && message.ID != discord.NullMessageID {
						resp.Callback(message)
					}
				}
			}
		}
	})
}

// RegisterCommands chews up the commands registered for the bot and actually registers them with Discord.
func RegisterCommands(ctx context.Context, state *state.State) error {
	ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.commands.RegisterCommands")
	defer span.End()

	app, err := state.CurrentApplication()
	if err != nil {
		return err
	}
	bulkCommands := []api.CreateCommandData{}
	for name, data := range commands {
		bulkCommands = append(bulkCommands, api.CreateCommandData{
			Name:                     name,
			Description:              data.Description,
			Options:                  data.Options,
			Type:                     data.Type,
			DefaultMemberPermissions: discord.NewPermissions(0),
		})
	}
	registered, err := state.BulkOverwriteCommands(app.ID, bulkCommands)
	if err != nil {
		return err
	}
	logger.Ctx(ctx).Infow(
		"commands successfully registered",
		"command quantity", len(registered),
	)
	return nil
}

// ClearObsoleteCommands removes the old, obsolete, per-guild-registered commands.
func ClearObsoleteCommands(ctx context.Context, state *state.State, guildID discord.GuildID) {
	ctx, span := otel.GetTracerProvider().Tracer("").Start(ctx, "dbot.commands.ClearObsoleteCommands")
	defer span.End()

	app, err := state.CurrentApplication()
	if err != nil {
		logger.Ctx(ctx).Errorw(
			"failed to clear obsolete commands, could not determine the app identifier",
			"error", err,
		)
		return
	}

	currentCommands, err := state.GuildCommands(app.ID, guildID)
	if err != nil {
		logger.Ctx(ctx).Errorw(
			"failed to clear obsolete commands, could not determine current guild commands",
			"guild id", guildID,
			"error", err,
		)
		return
	}
	for _, command := range currentCommands {
		if command.AppID == app.ID {
			if err := state.DeleteGuildCommand(app.ID, guildID, command.ID); err != nil {
				logger.Ctx(ctx).Errorw(
					"failed to remove obsolete command",
					"command", command.Name,
					"guid id", guildID,
					"error", err,
				)
			} else {
				log.Printf("[%s] Successfully removed obsolete Guild command /%s\n", guildID, command.Name)
				logger.Ctx(ctx).Infow(
					"successfully removed obsolete command from guild",
					"guild id", guildID,
					"command", command.Name,
				)
			}
		}
	}
}
