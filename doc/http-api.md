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

Set `authenticationMethod` in `settings.json`:

- `"apikey"`: calls need the API key in the `apikey` or `api_key` parameter or in the
  `Authorization` header. The key is read from `APIKEY.txt` in the Etherpad directory, or from
  the file named by the environment variable `EP_APIKEY_FILE`. If the file does not exist it is
  created with a random key, readable by the owner only. This is what Moodle and most plugins
  expect.
- `"sso"` (default): calls need `Authorization: Bearer <token>`. Accepted are an
  administrator's token, as for `/admin/api`, and an access token that an SSO client with the
  `client_credentials` grant got from Etherpad's OpenID provider.

## Differences to etherpad-lite

- `createGroupIfNotExistsFor` derives the group id from the mapper. After `deleteGroup` the
  same mapper gets the same id again, of a new, empty group. etherpad-lite gives it a new id.
  `deleteGroup` removes the group's pads and sessions in both.
- `getHTML` leaves out the `start` attributes etherpad-lite writes on imported ordered lists
  (`<ol start="2">` on a nested list, which browsers then number from 2).
- `anonymizeAuthor` answers `{authorId, anonymized}`, not etherpad-lite's counters.
- The REST-style paths `/rest/{version}/...` are not served.
