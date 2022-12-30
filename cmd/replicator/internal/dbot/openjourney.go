package dbot

import (
	"context"
	"fmt"

	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/command"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/response"
	"github.com/danstis/replicator/cmd/replicator/internal/dbot/interactions/utility"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/sausheong/goreplicate"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/exp/slices"
)

const (
	modelOwner   = "prompthero"
	modelName    = "openjourney"
	modelVersion = "9936c2001faa2194a261c01381f90e65261879985476014a0a37a334593a05eb"
)

func init() {
	command.Register("openjourney", commandOpenJourneyObject)
}

var commandOpenJourneyObject = command.Handler{
	Description: "Have replicate.com generate an image using the openjourney model",
	Code:        CommandOpenJourney,
	Options: []discord.CommandOption{
		&discord.StringOption{
			OptionName:  "prompt",
			Description: "Prompt to pass to replicate.com",
			Required:    true,
		},
	},
}

func CommandOpenJourney(state *state.State, event *gateway.InteractionCreateEvent, cmd *discord.CommandInteraction) command.Response {
	ctx, span := otel.GetTracerProvider().Tracer("").Start(context.Background(), "dbot.CommandOpenJourney")
	defer span.End()
	span.SetAttributes(
		attribute.String("user name", utility.ValueDefault(event.Member.Nick, event.Member.User.Username)),
		attribute.Int64("user id", int64(event.Member.User.ID)),
		attribute.Int64("guild id", int64(event.GuildID)),
	)
	log.Ctx(ctx).Infow("openjourney command called")

	if cmd.Options != nil && len(cmd.Options) > 1 {
		log.Ctx(ctx).Errorf("[%s] /openjourney command structure is incorrect", event.GuildID)
		return command.Response{Response: response.Ephemeral("/openjourney command structure is incorrect, check your input."), Callback: nil}
	}

	rm := goreplicate.NewModel(modelOwner, modelName, modelVersion)
	prompt := cmd.Options[slices.IndexFunc(cmd.Options, func(o discord.CommandInteractionOption) bool { return o.Name == "prompt" })].Value.String()
	rm.Input["prompt"] = prompt
	rm.Input["width"] = 512
	rm.Input["height"] = 512
	rm.Input["num_outputs"] = 1
	rm.Input["num_inference_steps"] = 50
	rm.Input["guidance_scale"] = 7.5
	repClient := goreplicate.NewClient(replicateToken, rm)
	err := repClient.Create()
	if err != nil {
		span.SetAttributes(
			attribute.String("prompt", prompt),
		)
		log.Ctx(ctx).Errorf("failed to create prediction: %v", err)
		return command.Response{Response: response.Ephemeral(fmt.Sprintf("failed to create prediction: %v", err)), Callback: nil}
	}
	err = repClient.Get(repClient.Response.ID)
	if err != nil {
		span.SetAttributes(
			attribute.String("output", fmt.Sprintf("%#v", repClient.Response.Output)),
			attribute.String("predictionID", repClient.Response.ID),
		)
		log.Ctx(ctx).Errorf("failed to get response from replicate.com: %v", err)
		return command.Response{Response: response.Ephemeral(fmt.Sprintf("failed to get response from replicate.com: %v", err)), Callback: nil}
	}

	return command.Response{Response: response.Ephemeral(fmt.Sprintf("Output: %v", repClient.Response.Output))}
}
