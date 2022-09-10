# Gatekeeper

Gatekeeper is a Discord bot for email user verification.

## Installation

Just run `go build` in the root of the project directory.

You should have a Discord application configured for this bot. You will need permission to assign roles as well as read messages from users.

You also need a Gmail account and an application password.

When running the bot, all configuration is passed in as environment variables. Required environment variables are `APP_ID`, `GMAIL_EMAIL`, `GMAIL_PASSWORD`, `DISCORD_TOKEN`.

## Usage

Invite the bot to a server. The admins should restrict certain commands to admin roles like `/ban` or `/config`.

The bot also must also be configured with the `/config` command to select which domain to filter email by, as well as which role should be placed on verified users. Note that the bot's role should be higher than the verified user's role, so that the bot can actually assign it.

Users can verify themselves by registering their email with the `/register` command and verifying their email with the `/verify` command.

If users are banned, it's their email that gets banned, not their account. They can re-verify with a new email on the same account, but they can't re-verify on a different account using the same email. This assumes that emails are scarce, such as a work or school environment where only one email is given, or for bot protection if the email domains prevent automatic signup. Note that "plus address" emails are collapsed.