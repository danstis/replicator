package dbot

import (
	"context"
	"fmt"

	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/command"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/response"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/utility"
	"github.com/danstis/replicator/internal/version"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func init() {
	command.Register("version", commandOpenJourneyObject)
}

var commandVersionObject = command.Handler{
	Description: "Return the current bot version",
	Code:        CommandOpenJourney,
}

func CommandVersion(state *state.State, event *gateway.InteractionCreateEvent, cmd *discord.CommandInteraction) command.Response {
	_, span := otel.GetTracerProvider().Tracer("").Start(context.Background(), "dbot.CommandVersion")
	defer span.End()
	span.SetAttributes(
		attribute.String("user name", utility.ValueDefault(event.Member.Nick, event.Member.User.Username)),
		attribute.Int64("user id", int64(event.Member.User.ID)),
		attribute.Int64("guild id", int64(event.GuildID)),
		attribute.String("version", version.Version),
	)

	return command.Response{Response: response.Ephemeral(fmt.Sprintf("Replicator bot v%s", version.Version))}
}
