package main

import (
	"context"
	"fmt"
	"gatekeeper/repository"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/handler"
	"gopkg.in/gomail.v2"
)

var repo *repository.Repository
var botEmail string
var emailDialer *gomail.Dialer

func cleanup(s *state.State, app *discord.Application) {
	// cleanup
	guilds, err := repo.ListGuildIDs()
	if err != nil {
		log.Fatalln("failed to get guilds:", err)
	}
	for _, id := range guilds {
		commands, err := s.GuildCommands(app.ID, id)
		if err != nil {
			log.Fatalln("failed to get guild commands:", err)
		}

		log.Println("Guild:", id)
		for name, cmd := range commands {
			err := s.DeleteGuildCommand(cmd.AppID, cmd.GuildID, cmd.ID)
			if err != nil {
				log.Println("couldn't delete command", name, "with id", cmd)
			}
		}
	}
}

func registerCommands(s *state.State, app *discord.Application) {
	guilds, err := repo.ListGuildIDs()
	if err != nil {
		log.Fatalln("failed to get guilds:", err)
	}

	for _, guildID := range guilds {
		log.Println("Guild:", guildID)

		g, err := s.Guild(guildID)
		if err != nil {
			log.Println("failed to get guild:", err)
			return
		}

		if os.Getenv("CLEANUP") != "" {
			commands, err := s.GuildCommands(app.ID, guildID)
			if err != nil {
				log.Fatalln("failed to get guild commands:", err)
			}
			// performs cleanup of commands this bot has registered and might not have cleaned up
			for _, command := range commands {
				log.Println("Existing command", command.Name, "found.")
				// delete pre-existing commands for this bot (in case a cleanup failed or something)
				if command.AppID == app.ID {
					s.DeleteGuildCommand(app.ID, command.GuildID, command.ID)
				}
			}
		}

		// register commands for guild
		for _, command := range CommandDefinitions {
			// WARNING: this is rate-limited fairly hard.
			cmd, err := s.CreateGuildCommand(app.ID, guildID, command)
			if err != nil {
				log.Fatalln("failed to create guild command:", err)
			}
			log.Println("Registering command", command.Name, "for guild", guildID)

			// for commands without default permissions, only allow the server owner to use them by default.
			// TODO: allow a configurable role to be used.
			if command.NoDefaultPermission {
				s.EditCommandPermissions(app.ID, guildID, cmd.ID, []discord.CommandPermissions{
					{
						ID:         discord.Snowflake(g.OwnerID),
						Type:       discord.UserCommandPermission,
						Permission: true,
					},
				})
			}
		}
	}
}

func init() {
	// note: this runs before the main function and within tests
	var err error
	repo, err = repository.New(mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalln("Database failed:", err)
	}

}

func main() {
	defer repo.Close()

	log.Println("Starting bot...")
	// Discord configuration
	token := mustEnv("BOT_TOKEN")

	// Email configuration
	botEmail = mustEnv("BOT_EMAIL")
	botPassword := mustEnv("BOT_PASSWORD")
	smtpHost := mustEnv("SMTP_HOST")
	smtpPort := mustEnv("SMTP_PORT")

	// string to int
	port, err := strconv.Atoi(smtpPort)
	if err != nil {
		log.Fatalln("SMTP port is not a number:", err)
	}

	emailDialer = gomail.NewDialer(smtpHost, port, botEmail, botPassword)

	// Create a new Discord gateway state
	s := state.New("Bot " + token)
	app, err := s.CurrentApplication()
	if err != nil {
		log.Fatalln("Session failed:", err)
	}

	// Add handlers and intents to the state
	s.AddHandler(MakeCommandHandlers(s))
	s.AddIntents(gateway.IntentGuilds)
	// For reading and deleting messages if configured
	s.AddIntents(gateway.IntentGuildMessages)
	// Make a pre-handler
	s.PreHandler = handler.New()

	// TODO: move
	// Discord bot event handlers
	s.PreHandler.AddSyncHandler(func(c *gateway.GuildCreateEvent) {
		err := repo.CreateGuild(c.Guild.ID)
		if err == repository.ErrGuildExists {
			log.Println("Guild already exists:", c.Guild.ID)
		} else if err != nil {
			log.Println("Failed to create guild:", err)
		} else {
			log.Println("Created guild:", c.Guild.ID)
			// register commands for guild
			for _, command := range CommandDefinitions {
				// WARNING: this is rate-limited fairly hard.
				cmd, err := s.CreateGuildCommand(app.ID, c.Guild.ID, command)
				if err != nil {
					log.Fatalln("failed to create guild command:", err)
				}
				log.Println("Registering command", command.Name, "for guild", c.Guild.ID)

				// for commands without default permissions, only allow the server owner to use them by default.
				// TODO: allow a configurable role to be used.
				if command.NoDefaultPermission {
					s.EditCommandPermissions(app.ID, c.Guild.ID, cmd.ID, []discord.CommandPermissions{
						{
							ID:         discord.Snowflake(c.Guild.OwnerID),
							Type:       discord.UserCommandPermission,
							Permission: true,
						},
					})
				}
			}
		}
	})

	s.PreHandler.AddSyncHandler(func(c *gateway.GuildDeleteEvent) {
		log.Println("Guild deleted:", c.ID)
	})

	s.PreHandler.AddSyncHandler(func(c *gateway.MessageCreateEvent) {
		if c.Message.GuildID.IsNull() {
			log.Println("Message is not in a guild:", c.Message.ID)
			return
		}

		channel, err := s.Channel(c.Message.ChannelID)
		if err != nil {
			log.Println("Failed to get channel:", err)
			return
		}
		id, err := repo.GetGuildVerificationChannel(channel.GuildID)
		if err != nil {
			return
		}
		if channel.ID == id {
			s.DeleteMessage(c.Message.ChannelID, c.Message.ID, api.AuditLogReason("Verification channel message deleted"))
		}
	})

	// Sent when a user is banned from a guild.
	s.PreHandler.AddSyncHandler(func(c *gateway.GuildBanAddEvent) {
		// TODO: add messaging to server admins/owner etc.
		if c.User.ID.IsNull() || c.GuildID.IsNull() {
			return
		}
		err := repo.BanUser(c.GuildID, c.User.ID)
		if err != nil {
			log.Println("Failed to ban user:", err)
		}
	})

	// Sent when a user is unbanned from a guild.
	s.PreHandler.AddSyncHandler(func(c *gateway.GuildBanRemoveEvent) {
		// TODO: add messaging to server admins/owner etc.
		if c.User.ID.IsNull() || c.GuildID.IsNull() {
			return
		}
		err := repo.UnbanUser(c.GuildID, c.User.ID)
		if err != nil {
			log.Println("Failed to unban user:", err)
		}
	})

	s.PreHandler.AddSyncHandler(func(c *gateway.MessageDeleteEvent) {
		m, err := s.Message(c.ChannelID, c.ID)
		if err != nil {
			log.Println("Not found:", c.ID)
		} else {
			log.Println(m.Author.Username, "deleted", m.Content)
		}
	})

	if err := s.Open(context.Background()); err != nil {
		log.Fatalln("failed to open:", err)
	}
	defer s.Close()

	// setup blocking goroutine to handle SIGINT and SIGTERM
	wait := func() func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
		return func() {
			<-sigChan
		}
	}()

	// block until ctrl+c or panic
	log.Println("blocking...")
	wait()
	log.Println("cleaning up")
	if os.Getenv("CLEANUP") != "" {
		cleanup(s, app)
	}
}

func mustEnv(name string) string {
	s := os.Getenv(name)
	if s == "" {
		panic(fmt.Sprintln("No environment variable named", name))
	}
	return s
}
