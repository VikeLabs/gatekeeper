package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

func init() {
	db.Load()
}

func main() {
	appID := discord.AppID(mustSnowflakeEnv("APP_ID"))
	guildID := discord.GuildID(mustSnowflakeEnv("GUILD_ID"))
	token := mustEnv("BOT_TOKEN")

	s := state.New("Bot " + token)
	s.AddIntents(gateway.IntentGuilds)

	s.AddHandler(MakeCommandHandlers(s, commandsGlobal))

	if err := s.Open(context.Background()); err != nil {
		log.Fatalln("failed to open:", err)
	}
	defer s.Close()

	activeCommands := registerCommands(s, appID, guildID)

	// closing this unblocks
	cleanup := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		close(cleanup)
	}()

	// make this so all background goroutines can finish cleaning up
	cleanupWaitGroup := &sync.WaitGroup{}

	// db will flush right before we die
	go PersistenceRoutine(cleanupWaitGroup, cleanup)
	cleanupWaitGroup.Add(1)

	// block until ctrl+c or kill
	log.Println("blocking...")
	<-cleanup
	log.Println("cleaning up")

	// cleanup
	cleanupCommands(s, activeCommands)

	cleanupWaitGroup.Wait()
	log.Fatalln("exiting")
}

func registerCommands(s *state.State, appID discord.AppID, guildID discord.GuildID) map[string]*discord.Command {
	log.Println("Gateway connected. Getting all guild commands.")
	existingCommands, err := s.GuildCommands(appID, guildID)
	if err != nil {
		log.Fatalln("failed to get guild commands:", err)
	}

	for _, command := range existingCommands {
		log.Println("Existing command", command.Name, "found.")

		// delete pre-existing commands for this bot (in case a cleanup failed or something)
		if command.AppID == appID {
			s.DeleteGuildCommand(appID, command.GuildID, command.ID)
		}
	}

	// track commands so we can delete them on cleanup
	activeCommands := make(map[string]*discord.Command)

	definitions := make([]api.CreateCommandData, 0, len(commandsGlobal))
	for _, v := range commandsGlobal {
		definitions = append(definitions, v.Data)
	}

	for _, command := range definitions {
		newCmd, err := s.CreateGuildCommand(appID, guildID, command)
		if err != nil {
			log.Fatalln("failed to create guild command:", err)
		}
		activeCommands[command.Name] = newCmd

		// TODO still not sure what to do about initial admin permissions
		// if command.NoDefaultPermission {
		// 	s.EditCommandPermissions(appID, guildID, newCmd.ID, []discord.CommandPermissions{})
		// }
	}

	return activeCommands
}

func cleanupCommands(s *state.State, activeCommands map[string]*discord.Command) {
	for name, cmd := range activeCommands {
		err := s.DeleteGuildCommand(cmd.AppID, cmd.GuildID, cmd.ID)
		if err != nil {
			log.Println("couldn't delete command", name, "with id", cmd)
		}
	}
}

func mustSnowflakeEnv(env string) discord.Snowflake {
	s, err := discord.ParseSnowflake(os.Getenv(env))
	if err != nil {
		log.Fatalf("Invalid snowflake for $%s: %v", env, err)
	}
	return s
}

func mustEnv(name string) string {
	s := os.Getenv(name)
	if s == "" {
		log.Fatalln("No environment variable named", name)
	}
	return s
}
