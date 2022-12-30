package message

import (
	"regexp"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

type Handler struct {
	Code  HandlerFunction
	Match *regexp.Regexp
}

type HandlerFunction func(
	state *state.State,
	event *gateway.MessageCreateEvent,
)

var messagehandlers = []Handler{}

// Register adds a handler for messages
func Register(handler Handler) {
	messagehandlers = append(messagehandlers, handler)
}

// Add the message handler to the given state
func AddHandler(state *state.State) {
	state.AddHandler(func(event *gateway.MessageCreateEvent) {
		for _, handler := range messagehandlers {
			if handler.Match == nil || handler.Match.MatchString(event.Content) {
				handler.Code(state, event)
			}
		}
	})
}
