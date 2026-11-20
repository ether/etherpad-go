# MCP server

Etherpad serves the [Model Context Protocol](https://modelcontextprotocol.io) at `/mcp`, so AI
clients such as Claude can read and edit pads and use their chat.

## Connecting

The transport is Streamable HTTP, and the server is stateless. It keeps no MCP session, so every
instance of a cluster answers every request and you need no sticky sessions.

Every request carries a token in the `Authorization` header (`Bearer ep_…`). You issue the token on
the admin page under *Tokens*, or as a person under *My tokens* in the pad's profile menu. A request
without a valid token gets `401`. A revoked or expired token stops working at once.

For Claude Code:

```sh
claude mcp add --transport http etherpad https://pads.example.com/mcp \
  --header "Authorization: Bearer ep_…"
```

OAuth, so that clients such as claude.ai connectors can connect without a pasted token, comes in a
later release.

## What a token may do

A tool can do what the token may do, checked per pad as everywhere else: the account's rights of the
moment, limited to the token's scopes and pad pattern.

| Tool | Scope | What it does |
|------|-------|--------------|
| `list_pads` | `pad:read` | Lists the text pads the token may read, page by page (`query`, `limit`, `cursor`). |
| `read_pad` | `pad:read` | Returns a pad's text and revision. |
| `create_pad` | `pad:create` | Creates a pad, failing if it exists. Group pads (`g.…$name`) belong to the group API. |
| `append_text` | `pad:write` | Adds text at the end of the pad. |
| `replace_text` | `pad:write` | Replaces text that occurs exactly once, or every occurrence (`all`, at most 100). |
| `set_text` | `pad:write` | Replaces the whole text. |
| `read_chat` | `pad:read` | Returns the last chat messages. |
| `send_chat` | `chat:write` | Writes in the pad's chat. |

The tools work on text pads. Sheets are not supported yet.

## Who writes

A person's token writes as that person, with the same author as in the pad. A service account's
token writes under the service account's own name, so its text stays apart from any person's: use a
service account named "Claude" for notes Claude keeps, and a personal token for "Claude writes as
me".

## Edits keep the rest

`append_text` and `replace_text` change only their text. The formatting and authorship of the rest
of the pad stay, and people editing the pad at the same time keep their changes: the edit takes the
pad's lock and reaches every open pad, on every instance, like a change typed in the pad.
`set_text` replaces everything for the new revision. The history keeps every earlier revision.
