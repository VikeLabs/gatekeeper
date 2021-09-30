package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

// To run, do `APP_ID="APP ID" GUILD_ID="GUILD ID" BOT_TOKEN="TOKEN HERE" go run .`

func main() {
	appID := discord.AppID(mustSnowflakeEnv("APP_ID"))
	guildID := discord.GuildID(mustSnowflakeEnv("GUILD_ID"))

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatalln("No $BOT_TOKEN given.")
	}

	s, err := state.New("Bot " + token)
	if err != nil {
		log.Fatalln("Session failed:", err)
		return
	}

	s.AddHandler(func(e *gateway.InteractionCreateEvent) {
		switch e.Data.Type() {
		case discord.CommandInteraction:
			cmd := e.Data.(*discord.CommandInteractionData)
			name := cmd.Name
			options := cmd.Options

			var data api.InteractionResponse
			switch name {
			case "ping":
				latency := time.Since(e.ID.Time())
				response := "Pong! `time=" + latency.String() + "`"

				data = api.InteractionResponse{
					Type: api.MessageInteractionWithSource,
					Data: &api.InteractionResponseData{
						Content: option.NewNullableString(response),
						Flags:   api.EphemeralResponse,
					},
				}
			case "echo":
				var echoedMsg string
				for _, v := range options {
					if v.Name == "message" {
						echoedMsg = v.String()
					}
				}
				thisGuild, err := s.Guild(e.GuildID)
				if err != nil {
					log.Println("wtf! guild doesn't exist")
					return
				}

				ownerID := thisGuild.OwnerID
				senderID := e.Member.User.ID
				if ownerID != senderID {
					echoedMsg = "Sorry, only the server owner can use this command."
				}

				data = api.InteractionResponse{
					Type: api.MessageInteractionWithSource,
					Data: &api.InteractionResponseData{
						Content: option.NewNullableString(echoedMsg),
						Flags:   api.EphemeralResponse,
					},
				}
			}

			if err := s.RespondInteraction(e.ID, e.Token, data); err != nil {
				log.Println("failed to send interaction callback:", err)
			}

		case discord.PingInteraction:
			data := api.InteractionResponse{
				Type: api.PongInteraction,
			}
			if err := s.RespondInteraction(e.ID, e.Token, data); err != nil {
				log.Println("failed to send interaction callback:", err)
			}
		}
	})

	s.AddIntents(gateway.IntentGuilds)

	if err := s.Open(context.Background()); err != nil {
		log.Fatalln("failed to open:", err)
	}
	defer s.Close()

	log.Println("Gateway connected. Getting all guild commands.")

	commands, err := s.GuildCommands(appID, guildID)
	if err != nil {
		log.Fatalln("failed to get guild commands:", err)
	}

	for _, command := range commands {
		log.Println("Existing command", command.Name, "found.")

		// delete pre-existing commands for this bot (in case a cleanup failed or something)
		if command.AppID == appID {
			s.DeleteGuildCommand(appID, command.GuildID, command.ID)
		}
	}

	newCommands := []api.CreateCommandData{
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

	// track commands so we can delete them on cleanup
	activeCommands := []*discord.Command{}

	for _, command := range newCommands {
		newCmd, err := s.CreateGuildCommand(appID, guildID, command)
		if err != nil {
			log.Fatalln("failed to create guild command:", err)
		}
		activeCommands = append(activeCommands, newCmd)

		if command.NoDefaultPermission {
			s.EditCommandPermissions(appID, guildID, newCmd.ID, []discord.CommandPermissions{
				// {
				// 	ID:
				// }
			})
		}
	}

	wait := func() func() {
		sigChan := make(chan os.Signal)

		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

		return func() {
			<-sigChan
		}
	}()

	// block until ctrl+c or panic
	fmt.Println("blocking...")
	wait()
	fmt.Print("\n")
	fmt.Println("cleaning up")

	// cleanup
	for _, cmd := range activeCommands {
		err := s.DeleteGuildCommand(cmd.AppID, cmd.GuildID, cmd.ID)
		if err != nil {
			log.Println("couldn't delete command with id", cmd)
		}
	}

	fmt.Println("exiting")
}

func mustSnowflakeEnv(env string) discord.Snowflake {
	s, err := discord.ParseSnowflake(os.Getenv(env))
	if err != nil {
		log.Fatalf("Invalid snowflake for $%s: %v", env, err)
	}
	return s
}
