use std::env;

use serenity::async_trait;
use serenity::model::gateway::Ready;
use serenity::model::id::GuildId;
use serenity::model::interactions::application_command::{
    ApplicationCommandInteractionData, ApplicationCommandInteractionDataOptionValue,
    ApplicationCommandOptionType,
};
use serenity::model::interactions::{Interaction, InteractionResponseType};
use serenity::prelude::*;

struct Handler;

fn unwrap_appcmd_param(
    data: &ApplicationCommandInteractionData,
    param: String,
) -> Option<ApplicationCommandInteractionDataOptionValue> {
    data.options
        .iter()
        .find(|x| x.name == param)?
        .clone()
        .resolved
}

#[async_trait]
impl EventHandler for Handler {
    async fn interaction_create(&self, ctx: Context, interaction: Interaction) {
        if let Interaction::ApplicationCommand(cmd) = interaction {
            let content = unwrap_appcmd_param(&cmd.data, "content".to_string())
                .expect("should have content value");

            let content = match content {
                ApplicationCommandInteractionDataOptionValue::String(s) => s,
                _ => "".to_string(),
            };

            cmd.create_interaction_response(&ctx.http, |resp| {
                resp.kind(InteractionResponseType::ChannelMessageWithSource)
                    .interaction_response_data(|msg| msg.ephemeral(true).content(content))
            })
            .await
            .expect("interaction response failed");
        }
    }

    async fn ready(&self, ctx: Context, ready: Ready) {
        println!("{} is up!", ready.user.name);

        let guild_id = GuildId(
            env::var("GUILD_ID")
                .expect("Expected GUILD_ID in environment")
                .parse()
                .expect("GUILD_ID must be an integer"),
        );

        let commands = GuildId::set_application_commands(&guild_id, ctx.http, |cmds| {
            cmds.create_application_command(|cmd| {
                cmd.name("echo")
                    .description("echoes input")
                    .create_option(|opt| {
                        opt.name("content")
                            .description("i will be echoed")
                            .kind(ApplicationCommandOptionType::String)
                            .required(true)
                    })
            })
        })
        .await;

        commands.expect("error setting up commands");
    }
}

#[tokio::main]
async fn main() {
    let token = env::var("DISCORD_TOKEN").expect("Need BOT_TOKEN");
    let intents = GatewayIntents::GUILDS;

    let mut client = Client::builder(token, intents)
        .event_handler(Handler)
        .await
        .expect("Error creating client");

    if let Err(why) = client.start().await {
        println!("Client error: {:?}", why);
    }
}
