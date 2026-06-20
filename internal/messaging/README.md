# Messaging

`human` sends and receives chat messages so an AI agent can post updates and take instructions without a human relaying them. Today it speaks Slack and Telegram through one consistent set of commands.

- Post messages to a Slack channel
- Read a Slack channel's recent history
- Receive tasks from a Telegram bot inbox
- Inspect and clear Telegram messages
- Guards Telegram with a default-deny allowlist
- Notifies on destructive operations for approval
- Reads tokens from config, env, or a vault
