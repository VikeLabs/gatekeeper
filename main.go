package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

func init() {
	loadDB()
}

func main() {
	appID := discord.AppID(mustSnowflakeEnv("APP_ID"))
	guildID := discord.GuildID(mustSnowflakeEnv("GUILD_ID"))
	token := mustEnv("BOT_TOKEN")

	s, err := state.New("Bot " + token)
	if err != nil {
		log.Fatalln("Session failed:", err)
		return
	}

	s.AddHandler(MakeCommandHandlers(s))
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

	// track commands so we can delete them on cleanup
	activeCommands := map[string]*discord.Command{}

	for _, command := range CommandDefinitions {
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

	wait := func() func() {
		sigChan := make(chan os.Signal)

		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

		return func() {
			<-sigChan
		}
	}()

	cleanupPersistence := make(chan struct{})
	go PersistenceRoutine(cleanupPersistence)

	// block until ctrl+c or panic
	log.Println("blocking...")
	wait()
	log.Println("cleaning up")

	close(cleanupPersistence)

	// cleanup
	for name, cmd := range activeCommands {
		err := s.DeleteGuildCommand(cmd.AppID, cmd.GuildID, cmd.ID)
		if err != nil {
			log.Println("couldn't delete command", name, "with id", cmd)
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

func mustEnv(name string) string {
	s := os.Getenv(name)
	if s == "" {
		panic(fmt.Sprintln("No environment variable named", name))
	}
	return s
}
