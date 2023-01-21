package main

import (
	"log"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

type CommandHandler func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse

type Command struct {
	Handler CommandHandler
	Data    api.CreateCommandData
}

var pingHandler = func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
	latency := time.Since(e.ID.Time())
	response := "Pong! `" + latency.String() + "`"
	return makeEphemeralResponse(response)
}

var commandsGlobal = []Command{
	{
		Data: api.CreateCommandData{
			Name:        "echo",
			Description: "Just like Unix",
			Type:        discord.ChatInputCommand,
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "message",
					Description: "Echo me!",
					Required:    true,
				},
			},
		},
		Handler: func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
			if !sentByOwner(s, e) {
				return makeEphemeralResponse("Sorry, only the server owner can use this command.")
			}

			message := options.Find("message")
			return makeEphemeralResponse(message.String())
		},
	},

	{
		Data: api.CreateCommandData{
			Name:        "register",
			Description: "Join the server by registering your email",
			Type:        discord.ChatInputCommand,
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "email",
					Description: "The email that you'd like to register",
					Required:    true,
				},
			},
		},
		Handler: func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
			email := options.Find("email")

			editResponse := func(newContent string) error {
				editedResponseData := api.EditInteractionResponseData{Content: option.NewNullableString(newContent)}
				_, err := s.EditInteractionResponse(e.AppID, e.Token, editedResponseData)
				return err
			}

			// lowercase the email, trim whitespace
			msg, err := Register(s, editResponse, e.SenderID(), e.GuildID, strings.TrimSpace(strings.ToLower(email.String())))
			if err != nil {
				log.Println("registration error:", err)
			return makeEphemeralResponse(msg)
			}
			return errorResponse
		},
	},

	{
		Data: api.CreateCommandData{
			Name:        "verify",
			Description: "Finalize registration by verifying your email",
			Type:        discord.ChatInputCommand,
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "token",
					Description: "The that you recieved in your email",
					Required:    true,
				},
			},
		},
		Handler: func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
			// exit early if in DMs somehow
			if e.Member == nil {
				return nil
			}

			token := options.Find("token")

			msg, err := Verify(s, e.SenderID(), e.GuildID, strings.TrimSpace(token.String()))
			if err != nil {
				log.Println("verification error:", err)
			return makeEphemeralResponse(msg)
			}
			return errorResponse
		},
	},

	{
		Data: api.CreateCommandData{
			Name:        "ban",
			Description: "Unverify a user and block their email from verifying",
			Type:        discord.ChatInputCommand,
			Options: []discord.CommandOption{
				&discord.UserOption{
					OptionName:  "user",
					Description: "The user to be banned",
					Required:    true,
				},
			},
		},
		Handler: func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
			user, err := options.Find("user").SnowflakeValue()
			if err != nil {
				log.Println("error parsing user:", err)
				return errorResponse
			}

			msg, err := Ban(s, discord.UserID(user), e.GuildID)
			if err != nil {
				log.Println("ban error:", err)
			return makeEphemeralResponse(msg)
			}
			return errorResponse
		},
	},
	{
		Data: api.CreateCommandData{
			Name:        "config",
			Description: "Configure the verification channel and role",
			Type:        discord.ChatInputCommand,
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "domain",
					Description: "The domain to filter emails by (for example, gmail.com)",
					Required:    true,
				},
				&discord.RoleOption{
					OptionName:  "role",
					Description: "The role that Gatekeeper gives to verified users",
					Required:    true,
				},
			},
		},
		Handler: func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
			domain := options.Find("domain").String()
			role, err := options.Find("role").SnowflakeValue()
			if err != nil {
				log.Println("error parsing role:", err)
				return errorResponse
			}
			msg, err := Config(s, e.GuildID, domain, discord.RoleID(role))
			if err != nil {
				log.Println("ban error:", err)
			return makeEphemeralResponse(msg)
			}
			return errorResponse
		},
	},

	// // COPY ME
	// {
	// 	Data: api.CreateCommandData{
	// 		Name:        "",
	// 		Description: "",
	// 		Type:        discord.ChatInputCommand,
	// 		Options:     []discord.CommandOption{},
	// 	},
	// 	Handler: func(s *state.State, e *gateway.InteractionCreateEvent, options discord.CommandInteractionOptions) *api.InteractionResponse {
	// 		return makeEphemeralResponse("TODO")
	// 	},
	// },
}

var errorResponse = makeEphemeralResponse("Sorry, an error has occurred")

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

	return thisGuild.OwnerID == e.SenderID()
}

func MakeCommandHandlers(s *state.State, commands []Command) func(*gateway.InteractionCreateEvent) {
	handlers := make(map[string]CommandHandler, len(commands))

	handlers["ping"] = pingHandler

	for _, c := range commands {
		handlers[c.Data.Name] = c.Handler
	}

	return func(e *gateway.InteractionCreateEvent) {
		switch i := e.Data.(type) {
		case *discord.PingInteraction:
			data := api.InteractionResponse{
				Type: api.PongInteraction,
			}
			if err := s.RespondInteraction(e.ID, e.Token, data); err != nil {
				log.Println("failed to send interaction callback:", err)
			}
		case *discord.CommandInteraction:
			cmd := i
			name := cmd.Name
			options := cmd.Options

			handler, ok := handlers[name]
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
		default:
			log.Printf("Unknown interaction of type %T\n", i)
		}
	}
}
