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
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	var err error

	db, err = InitDB()
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	appID := discord.AppID(mustSnowflakeEnv("APP_ID"))
	guildID := discord.GuildID(mustSnowflakeEnv("GUILD_ID"))
	token := mustEnv("DISCORD_TOKEN")

	// setup bot
	s := state.New("Bot " + token)
	s.AddIntents(gateway.IntentGuilds)

	s.AddHandler(MakeCommandHandlers(s, commandsGlobal))

	if err := s.Open(context.Background()); err != nil {
		log.Fatalln("failed to open:", err)
	}
	defer s.Close()

	// add application commands to guild
	activeCommands := registerCommands(s, appID, guildID)

	// setup cleanup channel for ctrl+c
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

	// block until ctrl+c or kill
	log.Println("bot is running")
	<-cleanup
	log.Println("cleaning up")

	cleanupWaitGroup.Add(1)
	go func() {
		// remove commands from guilds so people don't try and use the bot while it's down
		cleanupCommands(s, activeCommands)
		cleanupWaitGroup.Done()
	}()

	cleanupWaitGroup.Add(1)
	go func() {
		err := db.db.Close()
		if err != nil {
			log.Println("error closing db:", err)
		}
		cleanupWaitGroup.Done()
	}()

	cleanupWaitGroup.Wait()
	log.Println("exiting")
}

func registerCommands(s *state.State, appID discord.AppID, guildID discord.GuildID) map[string]*discord.Command {
	log.Println("Gateway connected. Getting all guild commands.")
	existingCommands, err := s.GuildCommands(appID, guildID)
	if err != nil {
		log.Fatalln("failed to get guild commands:", err)
	}

	// clear out existing commands in case the bot didn't clean up on last exit
	for _, command := range existingCommands {
		log.Printf("Removing existing command %v. The bot may have crashed last time.", command.Name)

		if command.AppID == appID {
			s.DeleteGuildCommand(appID, command.GuildID, command.ID)
		}
	}

	// track commands so we can delete them on cleanup
	activeCommands := make(map[string]*discord.Command)

	// extract command definitions from command global variable
	definitions := make([]api.CreateCommandData, 0, len(commandsGlobal))
	for _, v := range commandsGlobal {
		definitions = append(definitions, v.Data)
	}

	// register command definitions with discord
	for _, command := range definitions {
		newCmd, err := s.CreateGuildCommand(appID, guildID, command)
		if err != nil {
			log.Fatalln("failed to create guild command:", err)
		}
		activeCommands[command.Name] = newCmd
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
