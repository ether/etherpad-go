# Test parity with etherpad-lite

etherpad-lite has 172 backend test files with 1734 test cases (`src/tests/backend` and
`src/tests/backend-new`, at etherpad-lite commit 1ddd7e272). This file tracks which of them
have a Go counterpart. Go tests are written for the Go implementation; they cover the same
behaviour, they are not line-by-line translations.

| Status | Files | Test cases |
|---|---|---|
| Ported | 14 | 84 |
| Partly ported | 2 | 17 |
| Not applicable to Go | 33 | 288 |
| To compare case by case | 123 | 1345 |

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

## Partly ported

| Spec | Cases | State |
|---|---|---|
| `new/easysync-other` | 9 | moveOpsToNewPool, makeSplice, applyToAttribution and split/join are covered in `lib/changeset`; filterAttribNumbers, makeAttribsString, toSplices and opAttributeValue do not exist in the Go package (deprecated or client-only upstream) |
| `padInsertAuthorInvariant` | 8 | pad-level cases in `lib/pad/insert_author_test.go`; the `.etherpad` import and copyPadWithoutHistory cases are open |

## To compare case by case

### Changeset core

Existing Go tests in this area: `lib/changeset`, `lib/apool`

| Spec | Cases | Note |
|---|---|---|
| `colorutils` | 20 | needs a feature port first: the WCAG contrast helpers of etherpad-lite #7377 (`relativeLuminance`, `contrastRatio`, `ensureReadableBackground`) are not in `ui/src/js/colorutils.ts` |

### Pad and pad manager

Existing Go tests in this area: `lib/pad`, `lib/test/pad`, `lib/test/ws`

| Spec | Cases |
|---|---|
| `Pad` | 25 |
| `PadManager` | 4 |
| `chat` | 4 |
| `messages` | 14 |
| `undo_clear_authorship` | 6 |
| `lowerCasePadIds` | 4 |
| `pads-with-spaces` | 2 |
| `padIdColonLegacyPad` | 3 |
| `padUrlColonRedirect` | 3 |
| `padCheckErrorContext` | 7 |
| `padOptionsLegacyDefaults` | 11 |
| `nullAuthorToken` | 4 |
| `copyPadColonInjection` | 3 |
| `copyPadWithoutHistoryIntegrity` | 7 |

### Cleanup, compaction, retention, deletion

Existing Go tests in this area: `lib/pad/retention_test.go`, `lib/test/api/pad`

| Spec | Cases |
|---|---|
| `cleanupHoleSafety` | 2 |
| `cleanupMissingRevisions` | 11 |
| `cleanupNoDestructiveWindow` | 6 |
| `compactPad` | 32 |
| `padRetention` | 20 |
| `padDeletionManager` | 10 |
| `api/deletePad` | 7 |
| `api/movePadDeletionToken` | 5 |

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

### Import and export

Existing Go tests in this area: `lib/io`, `lib/test/api/io`

| Spec | Cases |
|---|---|
| `ExportEtherpad` | 9 |
| `ImportEtherpad` | 21 |
| `export` | 86 |
| `export_list` | 5 |
| `exportHtmlAuthorColorXss` | 7 |
| `import` | 14 |
| `api/importexport` | 4 |
| `api/importexportGetPost` | 49 |
| `sofficeExportRemoteImage` | 3 |
| `cssFontInlining` | 6 |

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
| `webaccess` | 49 |
| `socketio` | 45 |
| `security` | 12 |
| `sessionFixation` | 3 |
| `sessionIdCookie` | 6 |
| `staticPathTraversal` | 3 |
| `SecretRotator` | 27 |
| `oidcCookieRotator` | 1 |
| `OidcProviderSecurity` | 40 |

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
| `padDeletionUiPlacement` | 2 |

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
