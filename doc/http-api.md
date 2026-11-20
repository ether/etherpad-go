# HTTP API

etherpad-go has two HTTP APIs:

- `/admin/api/...`: its own REST API, documented in `docs/swagger.yaml`. It takes an
  administrator's bearer token.
- `/api/{version}/{function}`: etherpad-lite's HTTP API, for the clients written against it
  (Moodle, Nextcloud, WordPress and other integrations). This page is about this one.

## Calls

```
GET  /api                              → {"currentVersion": "1.3.1"}
GET  /api/openapi.json                 → OpenAPI document of 1.3.1
GET  /api/{version}/openapi.json       → OpenAPI document of a version
GET  /api/{version}/{function}?padID=…&apikey=…
POST /api/{version}/{function}         (query string, form, url-encoded or JSON body)
```

Versions 1 to 1.3.1 are served. Each version has the functions and parameters it had in
etherpad-lite: a parameter of a later version is ignored (for example `authorId` of `setText`
before 1.3.0). Parameters in the body win over the query string.

Every answer is `{"code": …, "message": …, "data": …}`:

| code | meaning | HTTP status |
|---|---|---|
| 0 | ok | 200 |
| 1 | wrong parameter, for example `padID does not exist` | 200 |
| 2 | internal error | 500 |
| 3 | no such function or version | 404 |
| 4 | no or wrong API key or token | 401 |

## Authentication

Calls need one of these:

- **A token.** Tokens are issued on the admin page under *Tokens*, for an account: a person
  or a service account (Moodle, a backup script, Claude, …). A token has its own scopes, an
  optional pad pattern and an expiry ("never" where `tokens` in the settings allows it). It
  can never do more than its account: on every call it gets the account's rights of the moment,
  cut down to its scopes and pattern; take a role from the account and its tokens lose it at
  once. The key (`ep_` and 40 characters) is shown once; Etherpad stores only its SHA-256 hash
  and its first ten characters. A token can be rotated (a new key with the same rights; the old
  one keeps working for a grace period: not at all, 10 or 20 days, or until a date), its expiry
  changed, and revoked; the list shows when each was last used, and from which IP. The key goes
  in the `apikey` or `api_key` parameter, or in the `Authorization` or `apikey` header.
- **A bearer token** (`Authorization: Bearer <token>`): an administrator's token, as for
  `/admin/api`, or an access token that an SSO client with the `client_credentials` grant got
  from Etherpad's OpenID provider.

Both work whatever `authenticationMethod` says.

What a token may call: the functions on one pad need a scope on that pad (`padID`): reading
(`getText`, `getHTML`, `getChatHistory`, …) `pad:read`; writing (`setText`, `appendText`,
`restoreRevision`, …) `pad:write`; `appendChatMessage` `chat:write`; `setPublicStatus`
`pad:settings`; `createPad` `pad:create` and `pad:write`; `deletePad` and `compactPad`
`pad:delete`. `copyPad` and `copyPadWithoutHistory` need `pad:read` on the source and
`pad:create` and `pad:write` on the destination (`movePad` also `pad:delete` on the source,
`force=true` also on the destination). Everything else (groups, authors, sessions,
`listAllPads`, statistics, calls with a read-only id) needs `admin:pads` for all pads, and
`checkToken` any valid token. A token without the right gets code 4 with status 403.
`/admin/api` takes tokens with `admin:pads` for all pads.

Tokens live in the database, so all instances of a cluster know them, and a revoked token is
refused on every instance at once. The API keys of earlier releases became tokens of the
service account `legacy-api` with every right, never expiring, under the same hash: they keep
working. A key from an `APIKEY.txt` (etherpad-lite, or an earlier etherpad-go release) is taken
over the same way once at start-up, named "APIKEY.txt"; the file is then no longer needed.
`EP_APIKEY_FILE` names another path for it. etherpad-go no longer creates the file.

## Path styles

The functions answer under three path styles, as in etherpad-lite, with the same tokens and
response format:

- `/api/{version}/{function}`, GET or POST, e.g. `/api/1.3.1/createGroup`. Documented at
  `/api/openapi.json` and `/api/{version}/openapi.json`.
- `/rest/{version}/{resource}/{action}`, GET or POST, e.g. `/rest/1.3.1/group/create`.
  Documented at `/rest/openapi.json` and `/rest/{version}/openapi.json`.
- `/api/2/...`, one HTTP method and path per function, e.g. `POST /api/2/groups`,
  `GET /api/2/pads/text?padID=...`, `PATCH /api/2/pads/text`. Documented at `/api-docs.json`,
  which can be browsed at `/api-docs`. An unknown route answers `{"code":1,"message":"not found"}`.

## Differences to etherpad-lite

- `createGroupIfNotExistsFor` derives the group id from the mapper. After `deleteGroup` the
  same mapper gets the same id again, of a new, empty group. etherpad-lite gives it a new id.
  `deleteGroup` removes the group's pads and sessions in both.
- `getHTML` leaves out the `start` attributes etherpad-lite writes on imported ordered lists
  (`<ol start="2">` on a nested list, which browsers then number from 2).
- etherpad-lite parses no body for `PATCH` and `PUT` on `/api/2`, so `appendText`,
  `appendChatMessage` and `restoreRevision` there only take query parameters. etherpad-go also reads
  the body.

## Erasing an author (GDPR Art. 17)

Erasing an author removes what identifies them and keeps the pad text: the name and colour,
the browser token and external ids (`createAuthorIfNotExistsFor` mappers) that lead to the
author, and the authorship of their chat messages. The author id stays in the pads'
history, but nothing ties it to a person any more.
A plain second erasure changes nothing and returns zeros.

- `anonymizeAuthor` (`/api/{version}/anonymizeAuthor?authorID=...`) and the admin page
  *Authors* need `"gdprAuthorErasure": {"enabled": true}` in `settings.json`, as in
  etherpad-lite. Without it the call fails and the page says the feature is off. The call
  answers etherpad-lite's counters: `affectedPads`, `removedTokenMappings`,
  `removedExternalMappings`, `clearedChatMessages`.
- `POST /admin/api/author/{authorId}/anonymize` (admin REST API) is under the same switch
  (403 when it is off); add `?dryRun=true` to get the counters without erasing.
- Sessions still connected as the erased author are disconnected, on every instance of a
  cluster. An erased author takes no new name, colour or chat. Erasing again cleans up
  whatever raced past that, and keeps the time of the first erasure.
- `createAuthorIfNotExistsFor` gives one author per external id, also for concurrent first
  calls: the mapper is unique (migration 16).
- The admin page lists authors with their last activity (`lastSeen`), finds them by name, id
  or external id, and shows a dry run before erasing.

## Moving a session to another browser

The author token is an HttpOnly cookie, so a browser cannot copy it to another one. The welcome
page has a *Transfer session* section instead (etherpad-lite's `tokenTransfer`):

- `POST /tokenTransfer` with `{"prefs": "<JSON>"}` reads the author cookie of the request and
  returns `{"id": "<32 hex>"}`. Without an author cookie (the browser never opened a pad) it
  answers 400.
- `POST /tokenTransfer/{id}` sets the author cookie (`SameSite=Lax`) in the browser that sends
  it and returns `{"ok": true, "prefs": ..., "prefsHttp": ...}`. A code works once and for 5
  minutes; unknown codes answer 404, expired ones 410. Unlike etherpad-lite, `GET` is refused
  (405), and so are requests from another site's page (`Sec-Fetch-Site` other than
  `same-origin`, or an `Origin` of another host; 403). Otherwise a link or an image elsewhere
  could swap a visitor's identity for one the attacker also holds. Creating a code is
  same-origin only too.

The link the welcome page copies (`/?transfer=<id>`) only fills in the code; the session is
replaced only when the user clicks *Receive session*.
