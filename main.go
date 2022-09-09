package main

import (
	"context"
	"fmt"
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
	token := mustEnv("DISCORD_TOKEN")

	// setup bot
	s := state.New("Bot " + token)
	s.AddIntents(gateway.IntentGuilds)

	s.AddHandler(MakeCommandHandlers(s, commandsGlobal))

	// executed when either you join a guild or when the bot starts
	// https://discord.com/developers/docs/topics/gateway#guilds
	s.AddHandler(func(e *gateway.GuildCreateEvent) {
		// TODO add a better option for whether to clobber old commands
		guild := e.Guild.ID
		if _, ok := os.LookupEnv("CLOBBER_CMDS"); ok {
			cmds, err := s.Commands(appID)
			if err != nil {
				log.Println("error getting commands:", err)
			}
			removeCommands(s, guild, cmds)
		}
		registerCommands(s, appID, guild)
	})

	if err := s.Open(context.Background()); err != nil {
		log.Fatalln("failed to open:", err)
	}
	defer s.Close()

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

	// cleanupWaitGroup.Add(1)
	// go func() {
	// 	// remove commands from guilds so people don't try and use the bot while it's down
	// 	cleanupCommands(s, activeCommands)
	// 	cleanupWaitGroup.Done()
	// }()

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

func registerCommands(s *state.State, appID discord.AppID, guildID discord.GuildID) error {
	// extract command definitions from command global variable
	definitions := make([]api.CreateCommandData, 0, len(commandsGlobal))
	for _, v := range commandsGlobal {
		definitions = append(definitions, v.Data)
	}

	// register command definitions with discord
	for _, command := range definitions {
		_, err := s.CreateGuildCommand(appID, guildID, command)
		if err != nil {
			return fmt.Errorf("failed to create guild command: %w", err)
		}
	}

	return nil
}

func removeCommands(s *state.State, guild discord.GuildID, activeCommands []discord.Command) {
	for _, cmd := range activeCommands {
		// only remove commands from current guild
		if cmd.GuildID != guild {
			continue
		}
		err := s.DeleteGuildCommand(cmd.AppID, guild, cmd.ID)
		if err != nil {
			log.Println("couldn't delete command", cmd.Name, "with id", cmd)
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
