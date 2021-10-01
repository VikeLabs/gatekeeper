package main

import (
	"log"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

var CommandDefinitions = []api.CreateCommandData{
	{
		Name:        "register",
		Description: "Join the server by registering your email",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			{
				Name:        "email",
				Description: "The email that you'd like to register",
				Type:        discord.StringOption,
				Required:    true,
			},
		},
	},
	{
		Name:        "verify",
		Description: "Finalize registration by verifying your email",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			{
				Name:        "token",
				Description: "The that you recieved in your email",
				Type:        discord.StringOption,
				Required:    true,
			},
		},
	},
	{
		Name:                "whois",
		Description:         "Admin only: get the user's indentifier",
		Type:                discord.ChatInputCommand,
		NoDefaultPermission: true,
		Options: []discord.CommandOption{
			{
				Name:        "user",
				Description: "The user whos identifier will be given",
				Type:        discord.UserOption,
				Required:    true,
			},
		},
	},
	{
		Name:                "setup",
		Description:         "Initialize the bot with necessary info to run",
		Type:                discord.ChatInputCommand,
		NoDefaultPermission: true,
		Options: []discord.CommandOption{
			{
				Name:        "domain",
				Description: "A domain to be allowlisted",
				Type:        discord.StringOption,
				Required:    true,
			},
			{
				Name:        "verified_role",
				Description: "The role that will be assigned to verified users",
				Type:        discord.RoleOption,
				Required:    true,
			},
			{
				Name:        "verification_channel",
				Description: "The channel that verification will occur in",
				Type:        discord.ChannelOption,
				Required:    true,
			},
		},
	},
	{
		Name:        "echo",
		Description: "Just like Unix",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			{
				Name:        "message",
				Description: "Echo me!",
				Type:        discord.StringOption,
				Required:    true,
			},
		},
	},
}

type CommandHandler func(s *state.State, e *gateway.InteractionCreateEvent, options []discord.InteractionOption) *api.InteractionResponse

var commandHandlerMap = map[string]CommandHandler{
	"ping": func(s *state.State, e *gateway.InteractionCreateEvent, options []discord.InteractionOption) *api.InteractionResponse {
		latency := time.Since(e.ID.Time())
		response := "Pong! `time=" + latency.String() + "`"

		return makeEphemeralResponse(response)
	},
	"echo": func(s *state.State, e *gateway.InteractionCreateEvent, options []discord.InteractionOption) *api.InteractionResponse {
		if !sentByOwner(s, e) {
			return makeEphemeralResponse("Sorry, only the server owner can use this command.")
		}

		for _, v := range options {
			if v.Name == "message" {
				return makeEphemeralResponse(v.String())
			}
		}

		log.Println("couldn't find 'message' param for command 'echo'")
		return nil
	},
}

func makeEphemeralResponse(msg string) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(msg),
			Flags:   api.EphemeralResponse,
		},
	}
}

func sentByOwner(s *state.State, e *gateway.InteractionCreateEvent) bool {
	thisGuild, err := s.Guild(e.GuildID)
	if err != nil {
		log.Println("wtf! guild", e.GuildID, "doesn't exist")
		return false
	}

	return thisGuild.OwnerID == e.Member.User.ID
}

func MakeCommandHandlers(s *state.State) func(*gateway.InteractionCreateEvent) {
	return func(e *gateway.InteractionCreateEvent) {
		switch e.Data.Type() {
		case discord.PingInteraction:
			data := api.InteractionResponse{
				Type: api.PongInteraction,
			}
			if err := s.RespondInteraction(e.ID, e.Token, data); err != nil {
				log.Println("failed to send interaction callback:", err)
			}
		case discord.CommandInteraction:
			cmd := e.Data.(*discord.CommandInteractionData)
			name := cmd.Name
			options := cmd.Options

			handler, ok := commandHandlerMap[name]
			if !ok {
				log.Println("Unrecognised command:", name)
				return
			}

			data := handler(s, e, options)
			if data == nil {
				// no response
				return
			}

			if err := s.RespondInteraction(e.ID, e.Token, *data); err != nil {
				log.Println("failed to send interaction callback:", err)
			}
		}
	}
}
