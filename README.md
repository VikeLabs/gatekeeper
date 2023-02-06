# Gatekeeper

Gatekeeper is a Discord bot for email user verification.

## Installation

Just run `go build` in the root of the project directory.

You should have a Discord application configured for this bot. You will need permission to assign roles as well as read messages from users.

You also need a Gmail account and an application password.

When running the bot, all configuration is passed in as environment variables. Required environment variables are `APP_ID`, `GMAIL_EMAIL`, `GMAIL_PASSWORD`, `DISCORD_TOKEN`.

## Usage

Invite the bot to a server. Some commands require specific permissions to view and use.

| **Restricted Commands** | **[Permission][p]** | **[Flag][f]** |
|-------------------------|---------------------|---------------|
| `/ban`                  | Ban Members         | BAN_MEMBERS   |
| `/config`, `/ban`       | Administrator       | ADMINISTRATOR |

The bot also must also be configured with the `/config` command to select which domain to filter email by, as well as which role should be placed on verified users. Note that the bot's role should be higher than the verified user's role, so that the bot can actually assign it.

Users can verify themselves by registering their email with the `/register` command and verifying their email with the `/verify` command.

If users are banned, it's their email that gets banned, not their account. They can re-verify with a new email on the same account, but they can't re-verify on a different account using the same email. This assumes that emails are scarce, such as a work or school environment where only one email is given, or for bot protection if the email domains prevent automatic signup. Note that "plus address" emails are collapsed.

<!-- MARKDOWN LINKS -->
[p]: https://support.discord.com/hc/en-us/articles/206029707-Setting-Up-Permissions-FAQ#h_01FFTVYZ40ZBHKZWTN1N8WPDTG
[f]: https://discord.com/developers/docs/topics/permissions#permissions-bitwise-permission-flags