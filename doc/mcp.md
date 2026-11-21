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

### With OAuth (claude.ai connectors and other clients)

Clients that speak MCP's authorization (OAuth 2.1) need no pasted token. Give them the address
`https://pads.example.com/mcp`, and they find the rest themselves:

1. `/mcp` answers `401` and points to the protected resource metadata (RFC 9728) at
   `/.well-known/oauth-protected-resource/mcp`. That metadata names Etherpad's own OpenID Connect
   provider (the `sso.issuer`) as the authorization server.
2. The client reads `/.well-known/oauth-authorization-server` (RFC 8414) and registers itself at
   `/oauth2/register` (dynamic client registration, RFC 7591). It gets a public client and must
   use PKCE with S256. Its redirect URIs must be `https`, or `http` to localhost.
3. The person logs in and sees a consent page with the app's name, where it sends them back to,
   the rights it asked for and a field for the pads. They can untick rights and limit the pads
   (`notes-*`). A client can never get more than the person's own rights.
4. The client gets an access token (15 minutes) and a refresh token. At `/mcp` the token works
   with the person's rights of the moment, limited to what they allowed.

The person sees connected apps under *My tokens* in the pad's profile menu. "Disconnect" stops
the app at once, its refresh token included. Connecting an app needs the scope `tokens:own`, the
same as issuing a token. Allowing the same app again replaces the earlier consent.

The issuer of `sso` must be the address people reach Etherpad at (or set `publicURL`). Turn
self-registering clients off with `"mcp": {"oauth": false}` (or `ETHERPAD_MCP_OAUTH=false`);
tokens pasted by hand keep working.

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
