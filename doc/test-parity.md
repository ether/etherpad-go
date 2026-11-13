# Test parity with etherpad-lite

etherpad-lite has 172 backend test files with 1734 test cases (`src/tests/backend` and
`src/tests/backend-new`, at etherpad-lite commit 1ddd7e272). This file tracks which of them
have a Go counterpart. Go tests are written for the Go implementation; they cover the same
behaviour, they are not line-by-line translations.

| Status | Files | Test cases |
|---|---|---|
| Ported | 53 | 558 |
| Partly ported | 3 | 127 |
| Not applicable to Go | 36 | 306 |
| To compare case by case | 80 | 743 |

"To compare" does not mean untested: most of these areas have Go tests (the Go code refers
to more than 20 upstream issues). It means nobody has checked yet, case by case, which
upstream cases the Go tests cover and which are missing.

## Ported

| Spec | Cases | Go tests |
|---|---|---|
| `appendRevisionAtomicity` | 7 | `lib/pad/append_atomicity_test.go` |
| `clientvar_rev_consistency` | 3 | `lib/test/ws/join_consistency_test.go` |
| `joinCatchUpRace` | 1 | `lib/test/ws/join_consistency_test.go` |
| `largePaste` | 1 | `lib/test/api/pad/pad_api_test.go` (plus a multi-byte variant) |
| `new/easysync-inverseRandom` | 2 | `lib/changeset/changeset_test.go` (`TestInverseRandom`, `TestInverse`) |
| `new/AttributeMap` | 14 | `lib/changeset/attributemap_test.go`, `attributemap_parity_test.go` |
| `new/StringIteratorTest` | 4 | `lib/changeset/StringIterator_test.go` |
| `new/attributes` | 23 | `lib/changeset/attributes_test.go`, `attributes_parity_test.go` |
| `new/easysync-assembler` | 15 | `lib/changeset/OPAssembler_test.go`, `SmartOpAssembler_test.go`, `smartopassembler_parity_test.go` |
| `new/easysync-subAttribution` | 1 | `lib/changeset/changeset_test.go` |
| `new/stampAuthorOnInserts` | 5 | `ui/src/js/stampAuthorOnInserts.test.ts` (client code, run with vitest) |
| `new/easysync-compose` | 2 | `lib/changeset/property_test.go` (`TestPropertyCompose`), `changeset_test.go` (`TestComposeAttributes`) |
| `new/easysync-mutations` | 5 | `lib/changeset/TextLinesMutator_test.go`, `mutations_parity_test.go` |
| `new/pad_utils` | 1 | `ui/src/js/pad_utils.test.ts` (client code, run with vitest) |
| `colorutils` | 20 | `ui/src/js/colorutils.test.ts` (client code, run with vitest) |
| `PadManager` | 4 | `lib/pad/pad_parity_test.go` (`TestIsValidPadId`) |
| `padIdColonLegacyPad` | 3 | `lib/test/api/static/pad_url_colon_test.go` |
| `padUrlColonRedirect` | 3 | `lib/test/api/static/pad_url_colon_test.go` |
| `copyPadColonInjection` | 3 | `lib/test/api/pad/pad_api_test.go` (`TestCopyPadColonDestination`) |
| `webaccess` | 49 | `lib/test/pad/webaccess_test.go`, `webaccess_parity_test.go` (plus malformed credentials) |
| `socketio` | 45 | `lib/test/ws/authn_access_test.go`, `pad_deletion_test.go`, `pad_settings_test.go`; the SocketIORouter cases test Node's socket.io router, which Go does not have |
| `sessionIdCookie` | 6 | `lib/server/session_cookie_test.go`, `lib/test/ws/session_id_cookie_test.go` |
| `staticPathTraversal` | 3 | `lib/api/static/plugin_static_test.go` |
| `SecretRotator` | 27 | `lib/security/secretrotator_parity_test.go`; the prefix-character cases do not apply (the prefix is a column, not part of a key pattern), "does not replace the array" is about JS array identity |
| `oidcCookieRotator` | 1 | `lib/api/oidc/provider_security_test.go` (keys from the database) |
| `OidcProviderSecurity` | 40 | `lib/api/oidc/provider_security_test.go`, `authenticator_test.go`; hash-only accounts and client_credentials admin tokens do not exist in the Go provider |
| `padDeletionManager` | 10 | `lib/test/pad/deletion_token_test.go` |
| `padDeletionUiPlacement` | 2 | `ui/src/js/pad_deletion.test.ts`, `lib/test/ws/pad_deletion_test.go` |
| `Pad` | 25 | `lib/pad/pad_test.go`, `pad_parity_test.go`, `insert_author_test.go`; normalizePadSettings in `lib/models/pad/settings_test.go`; pad-wide settings end to end in `lib/test/ws/pad_settings_test.go`, client logic in `ui/src/js/pad_settings.test.ts` |
| `ImportEtherpad` | 21 | `lib/test/api/io/import_etherpad_parity_test.go`, `import_etherpad_security_test.go`, `import_etherpad_roundtrip_test.go`; an author's pads come from the revisions they wrote. The two exportEtherpadAdditionalContent cases do not apply: Go plugins have no free-form keys in the database |
| `ExportEtherpad` | 9 | `lib/test/api/io/export_etherpad_parity_test.go`, `export_etherpad_revbound_test.go`; the three exportEtherpadAdditionalContent cases do not apply (see ImportEtherpad) |
| `padInsertAuthorInvariant` | 8 | `lib/pad/insert_author_test.go`; legacy `.etherpad` import and restore in `lib/test/api/io/import_etherpad_parity_test.go`, restoreRevision in `lib/test/api/pad/restore_revision_test.go`; copyPadWithoutHistory creates the copy from the text, so the source's attributes cannot break it |

| `export_list` | 5 | `lib/test/api/io/export_list_test.go` |
| `exportHtmlAuthorColorXss` | 7 | `lib/io/importer_test.go` (`TestSanitizeImportedColorId`), `exportHtml_test.go` (`TestGetHTMLFromAtext_AuthorColorXSS`), `lib/test/api/io/export_author_color_test.go` |
| `api/importexport` | 4 | `lib/test/api/io/import_html_cases_test.go` (the setHTML/getText table, 27 inputs); etherpad-go adds no empty line at the end of an import and stores a non-breaking space as a space; the HTML output is compared by round trip instead (`import_html_test.go`) |
| `api/importexportGetPost` | 49 | `lib/test/api/io/importexport_parity_test.go`; invalid revisions (`1test`, `test1`) answer 400, malformed `.etherpad` files 400, where etherpad-lite answers 200/500; the LibreOffice cases run against the native DOCX/ODT/PDF import and export |
| `import` | 14 | `lib/test/api/io/import_roundtrip_parity_test.go`, `importexport_parity_test.go`; the mammoth/html-to-docx converter cases do not apply (DOCX is read and written natively). DOCX and ODT import keep the text only (alignment, headings and formatting are lost) |
| `cssFontInlining` | 6 | `ui/src/css_font_inlining.test.ts` (vitest) |
| `cleanupHoleSafety` | 2 | `lib/test/api/pad/compact_parity_test.go` (the history is swapped in one transaction, the cached pad is dropped on failure) |
| `cleanupMissingRevisions` | 11 | `lib/test/api/pad/compact_parity_test.go` (findMissingRevisions is internal: the refusal names the holes); a full compaction repairs a pad only when its latest text can be rebuilt (from its last key revision on), since etherpad-go does not keep the full text in the pad row |
| `cleanupNoDestructiveWindow` | 6 | `lib/test/api/pad/compact_parity_test.go` |
| `padRetention` | 20 | `lib/pad/retention_test.go`, `retention_parity_test.go`, `lib/settings/pad_retention_defaults_test.go` |
| `api/deletePad` | 7 | `lib/test/api/pad/pad_api_test.go` (`TestPadDeletionTokensAPI`), on the admin REST API |
| `api/movePadDeletionToken` | 5 | `lib/test/api/pad/pad_api_test.go` (`TestPadDeletionTokensAPI`) |
| `chat` | 4 | `lib/test/ws/pad_message_handler_test.go` (chatNewMessage hook: text, pad, padId, drop), `lib/hooks/hook_test.go` |
| `messages` | 14 | `lib/test/ws/messages_parity_test.go`, `pad_message_handler_test.go` (handleMessageSecurity) |
| `undo_clear_authorship` | 6 | `lib/test/ws/undo_clear_authorship_test.go` |
| `lowerCasePadIds` | 4 | `lib/test/ws/lower_case_pad_ids_test.go`, `lib/test/api/static/lower_case_pad_ids_test.go` |
| `pads-with-spaces` | 2 | `lib/test/api/static/lower_case_pad_ids_test.go` |
| `padCheckErrorContext` | 7 | `lib/test/pad/pad_check_error_context_test.go`; Go errors have no stack, the message carries the context; a pad with a missing revision already fails to load, naming it |
| `padOptionsLegacyDefaults` | 11 | `lib/settings/pad_options_legacy_test.go`, `lib/test/ws/pad_options_clientvars_test.go` |
| `nullAuthorToken` | 4 | `lib/test/pad/null_author_token_test.go`, `lib/test/ws/null_author_token_test.go` |
| `copyPadWithoutHistoryIntegrity` | 7 | `lib/test/api/pad/copy_without_history_parity_test.go`, `compact_parity_test.go` |
## Partly ported

| Spec | Cases | State |
|---|---|---|
| `new/easysync-other` | 9 | moveOpsToNewPool, makeSplice, applyToAttribution and split/join are covered in `lib/changeset`; filterAttribNumbers, makeAttribsString, toSplices and opAttributeValue do not exist in the Go package (deprecated or client-only upstream) |
| `export` | 86 | the HTTP cases (500 on error, DOCX/PDF/ODT signatures and content types) in `lib/test/api/io/importexport_parity_test.go`. sanitizeExportHtml, extractBody, wrapLooseLines, dropEmptyBlocks, collapseRedundantBrAfterBlocks, separateAdjacentHeadingBlocks, applyMonospaceToCode and htmlToPdfBuffer test etherpad-lite's HTML-to-DOCX/PDF converter, which etherpad-go does not have: it builds the documents from the pad and fetches no URLs. Open: the PDF has no link annotations and ignores font-family (ep_font_family) and an exportPdfFonts setting, which etherpad-go does not have |
| `compactPad` | 32 | API.compactPad on `/admin/api/pads/{id}/compact` in `lib/test/api/pad/compact_parity_test.go`, `pad_compact_diff_api_test.go`; the 23 runCompactAll/runCompactStale cases test etherpad-lite's bin/compactAllPads and bin/compactStalePads scripts, which etherpad-go does not have |

## `:` in pad ids

Like etherpad-lite (GHSA-wg58-mhwv-35pq), a new pad id may not contain `:`. etherpad-go keys
pads by column, so the injection itself does not apply; the rule keeps pad ids portable. Pads
that already have a `:` in their id stay usable: the pad manager, the HTTP API and the pad URL
serve them as long as a pad with exactly that id exists. `/p/a:b` and `/p/a b` redirect to
`/p/a_b` when no such pad exists. Unlike upstream, the HTTP API also serves existing `:` pads
(upstream's `getPadSafe` refuses them), so no stored pad becomes unreachable through the API.

## To compare case by case

### Changeset core

Existing Go tests in this area: `lib/changeset`, `lib/apool`

| Spec | Cases | Note |
|---|---|---|

### HTTP API

Existing Go tests in this area: `lib/test/api/*`, `lib/api/pad`

| Spec | Cases |
|---|---|
| `api/api` | 13 |
| `api/pad` | 54 |
| `api/chat` | 7 |
| `api/characterEncoding` | 7 |
| `api/appendTextAuthor` | 2 |
| `api/createDiffHTML` | 3 |
| `api/restoreRevision` | 3 |
| `api/sessionsAndGroups` | 32 |
| `api/instance` | 2 |
| `api/jwtAdminClaim` | 9 |
| `apicalls` | 2 |
| `openapi` | 7 |
| `openapi-admin` | 20 |
| `downstream/wire-http-api` | 3 |
| `downstream/wire-socket-sequence` | 2 |
| `downstream/wire-vectors` | 2 |

### Authors and privacy

Existing Go tests in this area: `lib/test/author`, `lib/test/api/author`

| Spec | Cases |
|---|---|
| `anonymizeAuthor` | 8 |
| `api/anonymizeAuthor` | 3 |
| `admin/anonymizeAuthorSocket` | 7 |
| `admin/authorSearch` | 6 |
| `anonymizeIp` | 11 |
| `ipLoggingSetting` | 11 |
| `authorTokenCookie` | 2 |
| `ensureAuthorTokenCookie` | 5 |
| `tokenTransfer` | 10 |
| `new/hooks/express/firstAuthorOf` | 6 |
| `new/privacy/settings-defaults` | 2 |

### Access, sessions, security

Existing Go tests in this area: `lib/test/pad/webaccess_test.go`, `lib/security`, `lib/api/oidc`

| Spec | Cases |
|---|---|

### Settings and admin

Existing Go tests in this area: `lib/settings`, `lib/test/ws/admin_message_handler_test.go`

| Spec | Cases |
|---|---|
| `settings` | 21 |
| `admin/adminSettingsAuthError` | 1 |
| `admin/adminSettingsRedact` | 10 |
| `admin/adminSettingsResolved` | 4 |
| `admin/adminSettingsSave` | 3 |
| `admin/padLoadFilter` | 5 |
| `admin/padLoadResilience` | 4 |
| `new/admin_utils` | 7 |
| `settingsModalHeading` | 2 |

### Pages, paths, metadata

Existing Go tests in this area: `lib/test/api/static`, `lib/test/api/stats`, `lib/utils`

| Spec | Cases |
|---|---|
| `specialpages` | 14 |
| `favicon` | 6 |
| `i18n` | 1 |
| `pwaManifest` | 5 |
| `socialMeta` | 15 |
| `socialMeta-unit` | 37 |
| `urlBasePath` | 10 |
| `proxyPathRedirect` | 5 |
| `proxyPathVary` | 6 |
| `timesliderRedirect` | 3 |
| `health` | 3 |
| `new/sanitizePathname` | 4 |
| `new/sanitizeProxyPath` | 26 |
| `new/getHomeUrl` | 3 |
| `new/timesliderReturnUrl` | 5 |
| `new/path_exists` | 3 |
| `new/SkinColors` | 5 |
| `new/takeOverSignal` | 2 |
| `new/prom-instruments` | 5 |

### Updater

Existing Go tests in this area: `lib/updater`, `lib/server/update_test.go` (Go has its own updater design)

| Spec | Cases |
|---|---|
| `new/hooks/express/updateStatus` | 9 |
| `new/privacy/updateCheck-optout` | 2 |
| `new/updater/InstallMethodDetector` | 6 |
| `new/updater/MaintenanceWindow` | 22 |
| `new/updater/Notifier` | 13 |
| `new/updater/RollbackHandler` | 10 |
| `new/updater/Scheduler` | 29 |
| `new/updater/SessionDrainer` | 8 |
| `new/updater/UpdateExecutor` | 7 |
| `new/updater/UpdatePolicy` | 15 |
| `new/updater/VersionChecker` | 8 |
| `new/updater/applyPipeline` | 14 |
| `new/updater/lock` | 8 |
| `new/updater/preflight` | 22 |
| `new/updater/refSafety` | 9 |
| `new/updater/smtpTransportKey` | 5 |
| `new/updater/state` | 32 |
| `new/updater/trustedKeys` | 6 |
| `new/updater/updateLog` | 11 |
| `new/updater/versionCompare` | 18 |
| `updateActions` | 18 |
| `updateStatus` | 6 |
| `updater-integration` | 5 |
| `updater-scheduler-integration` | 2 |
| `updater-window-integration` | 4 |

## Not applicable to Go

| Spec | Cases | Why |
|---|---|---|
| `security` | 12 | tests the Node `security` package for plugins; Go plugins use Go's html escaping |
| `sessionFixation` | 3 | there is no server-side login session to fixate: Basic credentials are checked per request, the admin UI uses OIDC tokens |
| `LinkInstaller` | 7 | npm plugin installer |
| `OIDCAdapter` | 4 | adapter for the Node oidc-provider library; Go uses zitadel/oidc |
| `SessionStore` | 29 | express-session store adapter; Go's session storage has its own tests |
| `Stream` | 31 | JavaScript stream helper |
| `api/fuzzImportTest` | 0 | no test cases |
| `contentcollector` | 3 | client-side DOM collector (JavaScript) |
| `crossPadWriteToctou` | 1 | Node pauses in an async hook between two reads of the session's pad; Go handles a socket's messages one after the other. The handler now reads the pad id once. |
| `crypto` | 0 | no test cases |
| `dockerfilePnpmPin` | 3 | checks etherpad-lite's Dockerfile |
| `domline_class_token_injection` | 8 | client-side line rendering (JavaScript) |
| `domline_line_role` | 4 | client-side line rendering (JavaScript) |
| `domline_list_start` | 3 | client-side line rendering (JavaScript) |
| `downstream/generate-vectors` | 0 | no test cases (generator) |
| `filterUpdatablePluginNames` | 7 | npm plugin updates |
| `hooks` | 94 | the JavaScript plugin hook framework; Go has its own hook system with tests in `lib/hooks` |
| `new/admin-i18n-source-lint` | 7 | lints the Node admin UI sources |
| `new/backend-tests-flake-mitigation` | 2 | tests the Node test runner setup |
| `new/backend-tests-glob` | 1 | tests the Node test runner setup |
| `new/dependencyPath` | 2 | Node module resolution |
| `new/installerTasks` | 7 | npm plugin installer |
| `new/locale-files` | 2 | checks etherpad-lite's locale files |
| `new/pluginEngineCheck` | 8 | npm plugin engine field |
| `new/privacy/installer-optout` | 2 | npm plugin installer |
| `new/promises` | 8 | JavaScript promise helpers |
| `new/skiplist` | 3 | client-side JavaScript data structure |
| `new/template-l10n-keys` | 1 | checks etherpad-lite's EJS templates |
| `npmpublish-workflow` | 3 | checks etherpad-lite's GitHub workflow |
| `pluginCatalog` | 16 | npm plugin catalog |
| `pluginCatalogFilter` | 19 | npm plugin catalog |
| `regression-db` | 1 | ueberDB driver regression |
| `run_cmd` | 3 | Node child-process helper |
| `sanitizePluginsForWire` | 4 | npm plugin metadata |
| `setup-trusted-publishers` | 5 | npm publishing setup |
| `sofficeExportRemoteImage` | 3 | there is no LibreOffice converter and no HTML-to-document step; exports are built from the pad and fetch nothing |
