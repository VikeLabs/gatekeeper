package main

import (
	"fmt"
	"gatekeeper/repository"
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
		Name:        "ping",
		Description: "Pong!",
	},
	{
		Name:        "register",
		Description: "Join the server by registering your email",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "email",
				Description: "Your email address",
				Required:    true,
			},
		},
	},
	{
		Name:        "verify",
		Description: "Finalize registration by verifying your email",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "code",
				Description: "The code that you received in your email",
				Required:    true,
			},
		},
	},
	{
		Name:        "echo",
		Description: "Just like unix",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			&discord.StringOption{
				OptionName:  "message",
				Description: "The message to echo",
				Required:    true,
			},
		},
	},
	{
		Name:        "gatekeeper",
		Description: "Collection of GateKeeper adminstration commands",
		Type:        discord.ChatInputCommand,
		Options: []discord.CommandOption{
			discord.NewSubcommandOption("unverified", "Unverified commands",
				&discord.ChannelOption{
					OptionName:  "channel",
					Description: "Channel to delete any messages sent to.",
					Required:    true,
					ChannelTypes: []discord.ChannelType{
						discord.GuildText,
					},
				},
			),
			discord.NewSubcommandGroupOption("verifcation", "verification commands",
				discord.NewSubcommandOption("set", "Set a domain and role for verification",
					&discord.StringOption{
						OptionName:  "domain",
						Description: "The domain to add",
						Required:    true,
					},
					&discord.RoleOption{
						OptionName:  "role",
						Description: "The role to add",
						Required:    true,
					},
				),
				discord.NewSubcommandOption("delete", "Delete a domain and role for verification",
					&discord.StringOption{
						OptionName:  "domain",
						Description: "The domain to add",
						Required:    true,
					},
					&discord.RoleOption{
						OptionName:  "role",
						Description: "The role to add",
						Required:    true,
					}),
			),
		},
		// requires explicit setting of permissions as true
		NoDefaultPermission: true,
	},
}

type Interaction struct {
	state   *state.State
	event   *gateway.InteractionCreateEvent
	options []discord.CommandInteractionOption
}

type CommandHandler func(i *Interaction) *api.InteractionResponse

var commandHandlerMap = map[string]CommandHandler{
	// basic ping command returning latency and an embed response
	"ping": func(i *Interaction) *api.InteractionResponse {
		latency := time.Since(i.event.ID.Time())
		response := "Pong! `time=" + latency.String() + "`"
		embed := embeddedMessage(response)
		res := api.InteractionResponse{
			Type: api.MessageInteractionWithSource,
			Data: &api.InteractionResponseData{
				Embeds: &[]discord.Embed{
					*embed,
				},
				Content: option.NewNullableString(response),
				Flags:   api.EphemeralResponse,
			},
		}
		return &res
	},
	"echo": func(i *Interaction) *api.InteractionResponse {
		// TODO: remove default permissions and make this command only accessible to the owner/admin.
		if !sentByOwner(i.state, i.event) {
			return makeEphemeralResponse("Sorry, only the server owner can use this command.")
		}

		for _, v := range i.options {
			if v.Name == "message" {
				return makeEphemeralResponse(v.String())
			}
		}

		log.Println("couldn't find 'message' param for command 'echo'")
		return nil
	},
	"register": func(i *Interaction) *api.InteractionResponse {
		for _, v := range i.options {
			if v.Name == "email" {
				// lowercase the email, trim whitespace

				msg, err := Register(i.state, i.event.Member.User.ID, i.event.GuildID, normalizeEmail(v.String()))
				if err != nil {
					log.Println("registration error:", err)
					return makeEphemeralResponse("Sorry, an error has occurred")
				}
				return makeEphemeralResponse(msg)
			}
		}

		log.Println("couldn't find 'email' param for command 'register'")
		return nil
	},
	"verify": func(i *Interaction) *api.InteractionResponse {
		for _, v := range i.options {
			if v.Name == "code" {
				// direct message case
				if i.event.Member == nil {
					return nil
				}

				msg, err := Verify(i.state, i.event.Member.User.ID, i.event.GuildID, v.String())
				if err != nil {
					log.Println("verification error:", err)
					return makeEphemeralResponse("Sorry, an error has occurred")
				}
				return makeEphemeralResponse(msg)
			}
		}

		log.Println("couldn't find 'code' param for command 'verify'")
		return nil
	},
	"gatekeeper": func(i *Interaction) *api.InteractionResponse {
		// gatekeeper has subcommands
		for _, v := range i.options {
			switch v.Name {
			case "unverified":
				return subCommandHandlerMap["unverified"](&Interaction{
					state:   i.state,
					event:   i.event,
					options: v.Options,
				})
			case "verifcation":
				// set, delete
				return subCommandHandlerMap["verifcation"](&Interaction{
					state:   i.state,
					event:   i.event,
					options: v.Options,
				})
			}
		}
		return nil
	},
}

var subCommandHandlerMap = map[string]CommandHandler{
	"unverified": func(i *Interaction) *api.InteractionResponse {
		for _, v := range i.options {
			if v.Name == "channel" {
				channel, err := v.SnowflakeValue()
				if err != nil {
					log.Println("error getting channel snowflake:", err)
					return makeEphemeralResponse("Sorry, an error has occurred")
				}

				log.Println("setting unverified channel to", channel, "for guild", i.event.GuildID)
				if err := repo.SetGuildVerificationChannel(i.event.GuildID, discord.ChannelID(channel)); err != nil {
					log.Println("error setting channel:", err)
					return makeEphemeralResponse("Sorry, an error has occurred")
				}

				return makeEphemeralResponse("Success")
			}
		}
		return nil
	},
	"verifcation": func(i *Interaction) *api.InteractionResponse {
		for _, v := range i.options {
			fmt.Println(v.Name)
			switch v.Name {
			case "set":
				var domain *string
				var role *discord.RoleID
				for _, p := range v.Options {
					switch p.Name {
					case "domain":
						s := p.String()
						domain = &s
					case "role":
						roleID, err := p.SnowflakeValue()
						if err != nil {
							log.Println("error getting role snowflake:", err)
							return makeEphemeralResponse("Sorry, an error has occurred")
						}
						r := discord.RoleID(roleID)
						role = &r
					}
				}

				if domain == nil || role == nil {
					log.Println("missing required params for command 'verifcation set'")
					return makeEphemeralResponse("Sorry, an error has occurred")
				}

				log.Println("setting verification domain to", *domain, "and role to", *role, "for guild", i.event.GuildID)
				if err := repo.InsertDomainRole(i.event.GuildID, &repository.DomainRole{
					Domain: *domain,
					RoleID: *role,
				}); err != nil {
					log.Println("error setting domain:", err)
					return makeEphemeralResponse("Sorry, an error has occurred")
				}
				return makeEphemeralResponse("Success")
			case "delete":
				var domain *string
				var role *discord.RoleID
				for _, p := range v.Options {
					switch p.Name {
					case "domain":
						s := p.String()
						domain = &s
					case "role":
						roleID, err := p.SnowflakeValue()
						if err != nil {
							log.Println("error getting role snowflake:", err)
							return makeEphemeralResponse("Sorry, an error has occurred")
						}
						r := discord.RoleID(roleID)
						role = &r
					}
				}

				if domain == nil || role == nil {
					log.Println("missing required params for command 'verifcation set'")
					return makeEphemeralResponse("Sorry, an error has occurred")
				}
				log.Println("deleting verification domain for", *domain, "and role to", *role, "for guild", i.event.GuildID)
				if err := repo.DeleteDomainRole(i.event.GuildID, &repository.DomainRole{
					Domain: *domain,
					RoleID: *role,
				}); err != nil {
					log.Println("error deleting domain role:", err)
					return makeEphemeralResponse("Sorry, an error has occurred")
				}
				return makeEphemeralResponse("Success")
			}
		}
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
	// a crude interaction event router
	return func(e *gateway.InteractionCreateEvent) {
		switch d := e.Data.(type) {
		case *discord.PingInteraction:
			data := api.InteractionResponse{
				Type: api.PongInteraction,
				Data: &api.InteractionResponseData{
					Content: option.NewNullableString("Pong!"),
					Flags:   api.EphemeralResponse,
				},
			}
			if err := s.RespondInteraction(e.ID, e.Token, data); err != nil {
				log.Println("failed to send interaction callback:", err)
			}
		case *discord.CommandInteraction:
			name := d.Name
			handler, ok := commandHandlerMap[name]

			if !ok {
				log.Println("unrecognised command:", name)
				return
			}

			data := handler(&Interaction{
				state:   s,
				event:   e,
				options: d.Options,
			})
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
