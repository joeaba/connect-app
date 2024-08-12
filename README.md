# Slack Connect Manager

Slack Connect Manager is a Go application designed to help manage external Slack Connect users more effectively. It provides functionality that the native Slack app lacks, such as the ability to add external users to groups and manage them more efficiently.

## Features

- Create and manage teams of Slack Connect users
- Add and remove users from teams
- Ping entire teams in specific channels
- Track and manage channels
- Automatically update user information

## Prerequisites

- Go 1.15 or higher
- A Slack workspace with Slack Connect enabled
- A Slack Bot Token with necessary permissions

## Setup

1. Clone the repository:

```
git clone https://github.com/yourusername/slack-connect-manager.git
cd slack-connect-manager
```

2. Install dependencies:

`go mod tidy`

3. Create a `.env` file in the project root with the following content:

`SLACK_BOT_TOKEN=xoxb-your-bot-token-here`

4. Build the application:

`go build`

5. Run the application:

`./slack-connect-manager`

## Slack App Configuration

1. Create a new Slack App in your workspace:
- Go to https://api.slack.com/apps
- Click "Create New App"
- Choose "From scratch"
- Name your app and select your workspace

2. Set up OAuth & Permissions:
- Navigate to "OAuth & Permissions" in your app's settings
- Under "Scopes", add the following Bot Token Scopes:
  - `channels:read`
  - `channels:write`
  - `chat:write`
  - `commands`
  - `groups:read`
  - `groups:write`
  - `users:read`
  - `users:read.email`

3. Install the app to your workspace:
- Go to "Install App" in your app's settings
- Click "Install to Workspace"
- Authorize the app

4. Set up Slash Commands:
- Go to "Slash Commands" in your app's settings
- Click "Create New Command"
- Set the command to `/connect`
- Set the Request URL to `http://your-server-url:3000/slack/command`
- Add a short description and usage hint

5. Set up Event Subscriptions:
- Go to "Event Subscriptions" in your app's settings
- Enable events
- Set the Request URL to `http://your-server-url:3000/slack/events`
- Subscribe to the following bot events:
  - `channel_created`
  - `channel_renamed`
  - `channel_deleted`
  - `member_joined_channel`
  - `member_left_channel`

6. Retrieve your Bot Token:
- Go to "OAuth & Permissions" in your app's settings
- Copy the "Bot User OAuth Token" (starts with `xoxb-`)
- Paste this token into your `.env` file

## Usage

Once the application is running and configured, you can use the following slash commands in your Slack workspace:

- `/connect create-team <team>`: Create a new team
- `/connect remove-team <team>`: Remove an existing team
- `/connect add <team> <member_id>`: Add a member to a team
- `/connect remove <team> <member_id>`: Remove a member from a team
- `/connect print teams`: Print all teams
- `/connect print channels`: Print all tracked channels
- `/connect print members <team>`: Print all members of a specific team
- `/connect invite <team>`: Get member IDs for inviting a team
- `/connect ping <team> <channel>`: Ping all members of a team in a specific channel
- `/connect add-channel`: Add the current channel to the tracking list
- `/connect remove-channel <channel>`: Remove a channel from the tracking list
- `/connect help`: Show help message

## Deployment

To deploy this application, you'll need a server that can run Go applications and is accessible via HTTPS. Here are some general steps:

1. Set up a server (e.g., on AWS, DigitalOcean, or Heroku)
2. Install Go on the server
3. Clone your repository to the server
4. Build the application
5. Set up environment variables (including the `SLACK_BOT_TOKEN`)
6. Run the application (consider running it with `systemd`)
7. Set up a reverse proxy (e.g., Nginx) to handle HTTPS
8. Update your Slack App configuration with the new HTTPS URL

## License

This project is licensed under the MIT License.
