package main

import "github.com/diamondburned/arikawa/v3/discord"

func embeddedMessage(message string) *discord.Embed {
	embed := discord.NewEmbed()
	embed.Description = message
	embed.Color = 0x00FF00
	return embed
}
