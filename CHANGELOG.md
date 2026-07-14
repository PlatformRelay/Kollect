# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Release notes are generated from [Conventional Commits](https://www.conventionalcommits.org/)
on the default branch using [git-cliff](https://git-cliff.org/).

## [Unreleased]

### Bug Fixes

- **docs:** Correct OpenSSF Scorecard badge repo-name casing [4984cc7](https://github.com/platformrelay/kollect/commit/4984cc772611f4e88d4348f96b50d24178a9e48f)


### Features

- **branding:** Add favicon and social preview assets ([#74](https://github.com/platformrelay/kollect/pull/74))[930a82b](https://github.com/platformrelay/kollect/commit/930a82b30334aab63ae48d98bd1e56ccd4b5d604)

- **pipeline:** Sink credentials from env vars via ${env:VAR} secret placeholders [53b3d48](https://github.com/platformrelay/kollect/commit/53b3d48de4bd83df8484fe35a5de6a3e0ef13e59)

- **api:** Per-sink-binding maxExportBytes override (merge feat/export-partitioning, AR-01/EC-P0-01 Option B) [37596e9](https://github.com/platformrelay/kollect/commit/37596e9685fd89c179ad729e7487b1d8d0e268bb)

- **api:** Per-sink-binding maxExportBytes override (AR-01/EC-P0-01 Phase C, Option B) [1d26bfc](https://github.com/platformrelay/kollect/commit/1d26bfcc0ca10cfabd20621b1bfb471a92cee2be)


### Refactoring

- **sink:** Extract event-sink secret->auth helper into secretkv [4f14450](https://github.com/platformrelay/kollect/commit/4f144500d9d7e7828522a89e6ae46ef38279a20a)

## [0.8.0](https://github.com/platformrelay/kollect/compare/v0.7.0..v0.8.0) - 2026-07-10

### Bug Fixes

- **pipeline:** Drop computed make() capacity flagged by CodeQL overflow [11b9ee3](https://github.com/platformrelay/kollect/commit/11b9ee3aca8538fc2f18b4658fe973ea42795fd6)

- **cli:** Context selection, skip-to-exitcode, and dead --namespace flag [ba53f06](https://github.com/platformrelay/kollect/commit/ba53f06f5aaae4f6370294b234ab75bb0215a52e)

- **controller:** Requeue on family-sink status conflict instead of dropping it [7cb5cbe](https://github.com/platformrelay/kollect/commit/7cb5cbe742bbf30e2f8fed2c69aad01a7681823b)

- **collect:** Block on dispatch backpressure instead of inline sync fallback [c570338](https://github.com/platformrelay/kollect/commit/c5703386668390884d85b5e9327faba3a5b24e1e)

- **controller:** Degrade scope not whole target on RBAC-forbidden namespace (#28) [7cdc108](https://github.com/platformrelay/kollect/commit/7cdc108591e6adf1efb2cd4e498c4d5d69f77637)


### Features

- **cli:** Wire collect -> runner -> store -> sink end-to-end (P-005) [4e40389](https://github.com/platformrelay/kollect/commit/4e40389b0d95a4fb5e29a6b13685a0f4d466d956)

- **cli:** Kollect-pipeline cobra entrypoint + multi-context resolution (P-001) [7a29f6e](https://github.com/platformrelay/kollect/commit/7a29f6eea99feea100bc3db8e442651836b90f61)

- **collect:** One-shot List-based collection runner (P-003) [48e779a](https://github.com/platformrelay/kollect/commit/48e779a1f53f2913f7bcd9141f224867da5e05b2)

- **sink:** Local filesystem sink backend (P-004) [e8f6cd1](https://github.com/platformrelay/kollect/commit/e8f6cd1319464dec69162eb70f3a4b9b730d1a0a)

- **pipeline:** Local config loader (P-002) [63c4eff](https://github.com/platformrelay/kollect/commit/63c4eff9f1b592ced5b4501d7a75eff6533af3cc)

- **controller,sink:** Add TTL prune to coalesce tracker and backend pool [ff18055](https://github.com/platformrelay/kollect/commit/ff180555f590acaaf2a61381130be8e0fcce9a26)

- **controller:** Surface extraction failures in target status (EC-P1-05) (#36) [3c489e4](https://github.com/platformrelay/kollect/commit/3c489e406bc7929a28e39671d83854d7cb9e237d)

- **metrics:** Bound profile label cardinality (EC-P2-09) (#32) [fea790e](https://github.com/platformrelay/kollect/commit/fea790efbd11301f72ca3c295f04e248b4adfc5f)

- **scope:** Cluster static-ref namespace allowlist (#23) [789135f](https://github.com/platformrelay/kollect/commit/789135fa0f9e0ac07ba5c509690bf8c5ef1e357a)


### Performance

- **controller:** Indexed FieldIndexer lookup for sink-watch mappers (AR-09) [64494f4](https://github.com/platformrelay/kollect/commit/64494f47213f783c692e9d6db79fb9cfc36748a7)

- **controller,collect:** Incremental namespace fingerprint cache (AR-10) (#37) [6ca0c8e](https://github.com/platformrelay/kollect/commit/6ca0c8e565c730898d95ed538954de368329a220)


### Refactoring

- **aggregate:** Remove dead ExportCoalesce abstraction (AR-12) [1c1b14e](https://github.com/platformrelay/kollect/commit/1c1b14e71e1f091d66e4d32bf033d9dd1f944a12)

- **sink/postgres:** Typed sentinel errors for Export stages (F61) [0d81aba](https://github.com/platformrelay/kollect/commit/0d81aba4a8fd9e505d0af392afc07340210733f8)

- **validation,sink:** Typed errors for family-sink and s3 empty-payload [8568765](https://github.com/platformrelay/kollect/commit/856876525949480416dc9ab936d35da3c3580d08)

- **controller,api:** Generic family-sink reconciler (AR-08/F68) [afa3e85](https://github.com/platformrelay/kollect/commit/afa3e85a7b6f8261e0b68fce8b2e4a0a415b056c)

- **sink/bigquery:** Typed sentinel errors for Export failure stages (F66) [59da86f](https://github.com/platformrelay/kollect/commit/59da86f56090d605a52bc579249ea22b39081884)

- **sink/gitlab:** Dedup inventory path parsing via pathvalidate [10a4988](https://github.com/platformrelay/kollect/commit/10a4988efa2c0241e070862b6dcda8cfc6e45056)

- **sink:** Extract shared secret-value-from-keys helper (dup audit) (#38) [dacbece](https://github.com/platformrelay/kollect/commit/dacbecea88232e784ac1392fe1e1faf825f6108d)

- **webhook:** Dedup ValidateDelete boilerplate across validators (#33) [d333c26](https://github.com/platformrelay/kollect/commit/d333c26e81c721f2e5fa07083bddc348fde8287b)

- **sink:** Extract inventoryFromObjectPath into pathvalidate (#29) [4f4c6f2](https://github.com/platformrelay/kollect/commit/4f4c6f2e8595b1816dd47eb57599baaac660f7a2)

## [0.7.0-rc.1](https://github.com/platformrelay/kollect/compare/v0.6.0-rc.2..v0.7.0-rc.1) - 2026-06-13

### Bug Fixes

- **release:** Tolerate Rekor 409 on asset re-sign [0a8f61a](https://github.com/platformrelay/kollect/commit/0a8f61aeacba831e9f5c0d0df65066c8a3f429e5)

- **sink/nats:** Reconnect when cached connection is closed [05323e7](https://github.com/platformrelay/kollect/commit/05323e705067893d282d8fcde4de3310739369d3)

- **sink/bigquery:** Snapshot emulator mode in Config [7ab6387](https://github.com/platformrelay/kollect/commit/7ab6387ae4260c81d131f60128e3d399cfb5bd24)


### Features

- **webhook:** Reject cluster kinds in tenantMode [da49cd3](https://github.com/platformrelay/kollect/commit/da49cd3b159fd973b4ae4f086aab470cc330d0a9)

- **controller:** Classify forbidden static refs for cluster kinds [0825198](https://github.com/platformrelay/kollect/commit/08251983133c63e217aa53b3f1ad991d2051a066)

- **api:** [**breaking**] Reference namespaced static config from cluster kinds [7b30097](https://github.com/platformrelay/kollect/commit/7b3009777e1af48736820c387d098ddd5b4c81fc)

## [0.6.0-rc.2](https://github.com/platformrelay/kollect/compare/v0.6.0-rc.1..v0.6.0-rc.2) - 2026-06-10

### Bug Fixes

- **test:** Drop duplicate family sink condition tests [b6bfdb7](https://github.com/platformrelay/kollect/commit/b6bfdb77c085ca8893d0921acd98ff31335cba7d)

- **build:** Fix go-arch-lint exclude paths for local dirs [95850bb](https://github.com/platformrelay/kollect/commit/95850bbee5a7cc2adba50a9088f5d629fb42ce9d)

- **validation:** Block private sink endpoint targets [6c1ed5b](https://github.com/platformrelay/kollect/commit/6c1ed5ba0fa094553837d71764d8e1924b53d18d)


### Features

- **controller:** Shard snapshot exports by max bytes [8184257](https://github.com/platformrelay/kollect/commit/81842573dc7410decf22bee507b7eefa277b642e)


### Refactoring

- **bigquery:** Inject query execution adapter [71491d6](https://github.com/platformrelay/kollect/commit/71491d69e8813392d87c6454e9ecbc2a248fe8d1)

- **s3:** Isolate head-bucket check helper [8393135](https://github.com/platformrelay/kollect/commit/839313550ef779d07000b1bf3f5e04588f9b559b)

- **postgres:** Narrow upsert tx interfaces [75bb338](https://github.com/platformrelay/kollect/commit/75bb33871c23840747e22f66a82c45040d81eb29)

- **export:** Extract envelope partition helpers [6e13c42](https://github.com/platformrelay/kollect/commit/6e13c42438de4bda7d349ec5c9e2db91a254a9da)

- **mongodb:** Isolate export scope planning [cc264c9](https://github.com/platformrelay/kollect/commit/cc264c9a6a11d0b58f7b117e02ef4b6817aec1f3)

- **postgres:** Extract export planning helpers [d858a67](https://github.com/platformrelay/kollect/commit/d858a674d30c1ef7993455b00790f9b319f4aff1)

## [0.6.0-rc.1](https://github.com/platformrelay/kollect/compare/v0.5.0..v0.6.0-rc.1) - 2026-06-09

### Bug Fixes

- **sink:** Retry BigQuery emulator readiness in L3 tests [5106a32](https://github.com/platformrelay/kollect/commit/5106a328badf17bed2449b5390e846cce5680aec)

- **controller:** Enforce namespace intersections in rollups [c4439c3](https://github.com/platformrelay/kollect/commit/c4439c3fa09aca329ae3eb124b85e0fed7feaf67)

- **sink:** Re-enable backend pool after layout integration test [bea1372](https://github.com/platformrelay/kollect/commit/bea13724b8dbe03a6a219f64ac408d32f110767b)

- **sink:** Infer resource manifest layout [e83c30f](https://github.com/platformrelay/kollect/commit/e83c30f685408d590054b9bbb92088e3984ca204)

- **build:** Compile full cmd package after pprof fold [796e744](https://github.com/platformrelay/kollect/commit/796e74473316723cc99c290cdc102e30f8974275)

- **controller:** Panic-guard family-sink, connection-test, cluster-target reconcilers [b333ce0](https://github.com/platformrelay/kollect/commit/b333ce03bc101e3c0eaece0634bffb7faebcf4c5)

- **controller:** Aggregate per-sink export errors [af341f8](https://github.com/platformrelay/kollect/commit/af341f8d54103a4bd04bc58fa9ce514e4ed8a69c)

- **controller:** Stop requeue on terminal finalizer cleanup [b3ae025](https://github.com/platformrelay/kollect/commit/b3ae025003dee70b913eb7328d627b22d13c246a)

- **api:** [**breaking**] Reject stub sink types at admission [5eee44d](https://github.com/platformrelay/kollect/commit/5eee44dadbce102a61a57101e3d5bcc8a1ee7f80)

- **sink:** Remove stub backends, make unknown sink type terminal [eaf5a15](https://github.com/platformrelay/kollect/commit/eaf5a15d848916316f12bddbd4ae227940125ca9)

- **sink/git:** Redact credentials from git CLI errors [7a8e6d0](https://github.com/platformrelay/kollect/commit/7a8e6d0ad81876bb797a0fd06d501e24b9f49431)

- **collect:** Stabilize export fingerprints for debounce [038cbec](https://github.com/platformrelay/kollect/commit/038cbec031e23c38181f38a038f0726d1c2fb6a0)

- **docs:** Purge stale hub/spoke from operator manual [367b6bf](https://github.com/platformrelay/kollect/commit/367b6bfcb2b1009dcbf301d85660d4cb8109df6d)

- **docs:** Repair broken links and stale ADR references [be7c448](https://github.com/platformrelay/kollect/commit/be7c448a0e3882dfc78d249d0ac15ae5ce479abc)

- **chart:** Sync family-sink CRDs with ADR-0416 fields [7baaa66](https://github.com/platformrelay/kollect/commit/7baaa6667157cc1ff1d396e272d68e5155132382)


### Features

- **api:** Add cluster rollup shard status [59dcdaf](https://github.com/platformrelay/kollect/commit/59dcdafdad14b816f9d988fb1b066e70fee5a200)

- **api:** Add cluster inventory namespaces list [713cdf0](https://github.com/platformrelay/kollect/commit/713cdf038a47ecbfda1aa64b8f2e6ce69da89df4)

- **helm:** Add minimal RBAC team install profile [925a8d8](https://github.com/platformrelay/kollect/commit/925a8d80804dd8f11fe95df81584047793a20061)

- **sink:** Add BigQuery database backend [ec64d81](https://github.com/platformrelay/kollect/commit/ec64d81b0165c8bf7b639489107935b82c183997)

- **sink/nats:** Version event envelope schema [58a4bf4](https://github.com/platformrelay/kollect/commit/58a4bf454abce40ad45cd8eb9bddbb7174982e85)

- **demo:** Add hero harness with in-kind Forgejo [6e6a86a](https://github.com/platformrelay/kollect/commit/6e6a86af8d55bf2e713995a32ff5225609012e5e)

- **sink:** Wire ADR-0419 git layout into export pipeline [c33baa6](https://github.com/platformrelay/kollect/commit/c33baa65eb8d08edbe2a53ae5308e8adb1648653)

- **export:** Full-resource pruning and Git layout [8502f58](https://github.com/platformrelay/kollect/commit/8502f585c4e67b61c3e43b04e389c367087f183a)

- **sink:** Render status.preview surface (ADR-0416) [f1931fc](https://github.com/platformrelay/kollect/commit/f1931fc08df4ef9230daead45df28b6200381222)

- **sink:** MongoDB database sink (ADR-0417) [3796733](https://github.com/platformrelay/kollect/commit/3796733b10b89ae712117ac5e5e975f190c310a8)


### Refactoring

- **controller:** Compose cluster rollups by namespace [374f6e6](https://github.com/platformrelay/kollect/commit/374f6e6f7ad9010e97f4673777e2644f01d074f0)

- **inventory:** Fold internal/httpauth into inventory [e570f82](https://github.com/platformrelay/kollect/commit/e570f824ea656e0080970ee9103228bb25d617e4)

- **cmd:** Fold internal/pprof into cmd [d0c7f82](https://github.com/platformrelay/kollect/commit/d0c7f827387e0815a70ae72150077cc84d24e829)

- **validation:** Decouple layout path checks from sink package [43d89c6](https://github.com/platformrelay/kollect/commit/43d89c6d8704699ee563813056720daf8e38729b)

## [0.5.0-rc.1](https://github.com/platformrelay/kollect/compare/v0.4.1..v0.5.0-rc.1) - 2026-06-07

### Bug Fixes

- **build:** Upgrade alpine packages in UI image for Trivy [3cdce54](https://github.com/platformrelay/kollect/commit/3cdce5435241add437f7205c0adaa9ba5271fa22)

- **ci:** Stop perf-report writing agent-context in CI [5efafd6](https://github.com/platformrelay/kollect/commit/5efafd691f447055aa866d2f927325b8bd020c74)

- **ui:** Disambiguate Playwright inventory row locator [878dff2](https://github.com/platformrelay/kollect/commit/878dff280b05ba10ea609616b8dcceffc79a15e5)

- **ci:** Stabilize nightly Playwright and perf-report [fcb0533](https://github.com/platformrelay/kollect/commit/fcb0533e963be476064d7b7c9ae41d06fddcd88b)


### Features

- **api:** ADR-0416 sink config layering [5f07736](https://github.com/platformrelay/kollect/commit/5f07736405d2c9f929810873d231f2123342e7b3)

## [0.4.1](https://github.com/platformrelay/kollect/compare/v0.4.0..v0.4.1) - 2026-06-07

### Bug Fixes

- **build:** Upgrade debian packages for Trivy gate [5c8f07f](https://github.com/platformrelay/kollect/commit/5c8f07f4242066cc56c0846e0ed9caab342e7ff5)


### Features

- **sink/git:** PERF-10 mirror and fingerprint skip [f8d3d23](https://github.com/platformrelay/kollect/commit/f8d3d2363b91f2db3d457fe0fb94bf01bf6a2690)

- **perf:** Scale lane PERF-08/09/15 and 10k load tier [f2444ff](https://github.com/platformrelay/kollect/commit/f2444ffc3f7eecc683db53a352cbf2e8fc404fbe)

## [0.4.0](https://github.com/platformrelay/kollect/compare/v0.3.0-rc.1..v0.4.0) - 2026-06-07

### Features

- **collect:** PERF-03 tunable dispatch pool [66426c8](https://github.com/platformrelay/kollect/commit/66426c8115a0c4fab5be4b1a3b955b8bd47ce38b)

- **ui:** Align sink families and add UI docs [5b1cc94](https://github.com/platformrelay/kollect/commit/5b1cc94280650c0c2c30ceb870c92fe47f76f3ea)


### Refactoring

- **collect:** GVR index and sharded store [0c97ee7](https://github.com/platformrelay/kollect/commit/0c97ee74f575a674d610e02b6d75b7b0d66a24c8)

## [0.3.0-rc.1](https://github.com/platformrelay/kollect/compare/v0.2.0-rc.1..v0.3.0-rc.1) - 2026-06-07

### Bug Fixes

- **controller:** Recover panics and suspend status [3bb8c55](https://github.com/platformrelay/kollect/commit/3bb8c55ce4a4b7241a1c7147e1113e4c91f6cca4)

- **git:** Resolve lint issues in sink hardening [1cd7ef7](https://github.com/platformrelay/kollect/commit/1cd7ef77e3feae46775a14b3ab0c195776dae1e7)

- **docs:** Remove extra blank line in git-sink-attribution [152cb40](https://github.com/platformrelay/kollect/commit/152cb40349985a45d58f78b1778afb07d0b4ccf6)

- **e2e:** Guard multitenant port-forward cleanup trap [fde54d4](https://github.com/platformrelay/kollect/commit/fde54d4bf52938efbd662c91931de9ab27df5284)

- **e2e:** Use object form for snapshotSinkRefs [c434c9e](https://github.com/platformrelay/kollect/commit/c434c9e5007a4400d123c75da733db50f12bb5dc)

- **e2e:** Validate git-export and multitenant via HTTP [b934eb2](https://github.com/platformrelay/kollect/commit/b934eb20a27f8299e1acfe9d2c74245b0e9e02d5)


### Features

- **sink/git:** Rich commit context and templates [96834ba](https://github.com/platformrelay/kollect/commit/96834baf767688e781c21776941c2dff09e238b4)

- [**breaking**] Remove hub/spoke tier and KollectRemoteCluster [edfb2b6](https://github.com/platformrelay/kollect/commit/edfb2b683fc3eb37a4c8089985a2d31fd30519fb)

- **git:** Harden export clone, auth, and push recovery [22921a0](https://github.com/platformrelay/kollect/commit/22921a0a9350b017409fed13fb627386a6622c6d)


### Refactoring

- **perf:** P0/P1 export path optimizations [3e12aaf](https://github.com/platformrelay/kollect/commit/3e12aaf86cf5c8968f58d2448037bcb50218eafe)

## [0.2.0-rc.1](https://github.com/platformrelay/kollect/compare/v0.1.0-rc.3..v0.2.0-rc.1) - 2026-06-07

### Bug Fixes

- **e2e:** Drop legacy sinkRefs from multitenant scope [7efd330](https://github.com/platformrelay/kollect/commit/7efd330cc3b723fd6c617818feb8228bc7cc3154)

- **e2e:** Drop removed inventory sinkRefs field [f7de7af](https://github.com/platformrelay/kollect/commit/f7de7af9fb10c20b02ca7bedb5dc8277723180c6)

- **e2e:** Validate collection via inventory HTTP [67404df](https://github.com/platformrelay/kollect/commit/67404df05408ed0a78ce45d736766c844e36fe47)

- **e2e,test:** Stabilize smoke bootstrap and debounce IT [2b7b31d](https://github.com/platformrelay/kollect/commit/2b7b31d773dcf5f1b54243e460957c9059b19be3)

- **samples:** Drop legacy sinkRefs from team-a scope [4598ce5](https://github.com/platformrelay/kollect/commit/4598ce5325c048f77dbe1b606f8bb91fb1cf1185)

- **e2e:** Wait on family sink CRDs in kind smoke [fac0a82](https://github.com/platformrelay/kollect/commit/fac0a8262bd7899fc3228dd2f9626a460e5499ea)

- **gitlab:** Basic auth for Forgejo Gitea MR API [92e2ce0](https://github.com/platformrelay/kollect/commit/92e2ce027fcae1010e14093f667ad6fe15a23e80)


### Features

- **git:** Port transport retry and SSH host keys [1486898](https://github.com/platformrelay/kollect/commit/1486898b3dde1ac26a51dad7bd11abcbf30312ed)

- **controller:** Wire family sink reconcilers and export [61d1a33](https://github.com/platformrelay/kollect/commit/61d1a33cdf0ea7b59bde6bf467d1eefc79b28d9b)

- **api:** Add sink family CRDs and remove KollectSink [efc1b1a](https://github.com/platformrelay/kollect/commit/efc1b1a6d5c59904a4659323f70af76450f542ce)

## [0.1.0-rc.3](https://github.com/platformrelay/kollect/compare/v0.1.0-rc.2..v0.1.0-rc.3) - 2026-06-06

### Bug Fixes

- **collect,controller:** Resolve race detector findings [2fc1aff](https://github.com/platformrelay/kollect/commit/2fc1affd712b0e6476770f51dec55394aeb13495)

- **api:** Keep KollectRemoteCluster status optional in codegen [578af5e](https://github.com/platformrelay/kollect/commit/578af5e53e8f9d9d095694eab4e436ec26ebe8ce)

- **api:** Drop required status on KollectRemoteCluster create [88e7bc5](https://github.com/platformrelay/kollect/commit/88e7bc573dc4b13fec222a684d61486aec5f74f1)

- **ops:** P2 hardening and chart connectionTest default [4a07ab6](https://github.com/platformrelay/kollect/commit/4a07ab6fbeb42fc2e1254028cc9b91b52b13e1d5)

- **git:** Terminal auth errors and per-repo export lock [2b596ba](https://github.com/platformrelay/kollect/commit/2b596ba23707440c86dff934a84ad87cc565a21d)

- **lint:** Gofmt webhooks and phase A envtest cleanup [f132bf9](https://github.com/platformrelay/kollect/commit/f132bf9d126d2d3a23aadcc549aed8dc2c93725a)

- **sink:** Isolate circuit breaker test from parallel pollution [d5ec865](https://github.com/platformrelay/kollect/commit/d5ec8650f1d6235002b923204b5ff4f9ba051ae0)

- **e2e:** Revert multitenant namespaceSelector [4ba734c](https://github.com/platformrelay/kollect/commit/4ba734c4621d3e24fe498cb3b42b14879020d1b5)

- **controller:** Continue multi-sink export on partial failure [6b1fa36](https://github.com/platformrelay/kollect/commit/6b1fa364bf38895c42a227f68cbfdd6a120e37f5)

- **e2e:** Apply tenant-scope after multitenant asserts [475f3f9](https://github.com/platformrelay/kollect/commit/475f3f95f8635edb2e0cbabf8165b9fe20be27a1)

- **e2e:** Stabilize multitenant matrix job waits [9ed2c7a](https://github.com/platformrelay/kollect/commit/9ed2c7a6942e05918a1ce3798be3d9147fad3f42)

- **sink:** Validate git export paths for CodeQL [4855b3d](https://github.com/platformrelay/kollect/commit/4855b3db351127ccfc4ddadcfb0aa51f78046efa)

- **ci:** Sync CHANGELOG and UI Docker npm ci [0eef82c](https://github.com/platformrelay/kollect/commit/0eef82caad5ad086467331672732eb4bfc45a1f1)

- **e2e:** Bootstrap samples for matrix git-export [7957b7b](https://github.com/platformrelay/kollect/commit/7957b7b0e4ff0c2c0b99083d59b8e0f2934b4e05)


### Features

- **controller:** Add cluster rollup finalizers [a371b8b](https://github.com/platformrelay/kollect/commit/a371b8b0a4eb0de7da3844fe726aca167617eddb)

- **controller:** Add target finalizers [240f4be](https://github.com/platformrelay/kollect/commit/240f4be405d5d55d527b9bf6ba0c180bdc49490a)

- **controller:** Extend inventory finalizer teardown [db06c39](https://github.com/platformrelay/kollect/commit/db06c39668b4947b89a42ae153192c31929a3abc)

- **collect:** Add hub cluster store cleanup [23ef632](https://github.com/platformrelay/kollect/commit/23ef632458e58f5c4c78c4a60f3a1052daff4d38)

- **collect:** Add helm: release Secret decode [a5be27a](https://github.com/platformrelay/kollect/commit/a5be27a19ad2cfd6cf655dbcc12805a13aed7b8b)

- **samples:** Add helm-release-values-redacted profile [2bb8a36](https://github.com/platformrelay/kollect/commit/2bb8a36b2e243453f37f1b066850a4f8e893be8b)

- **collect:** ScrubKeys redaction at extraction [36d3b53](https://github.com/platformrelay/kollect/commit/36d3b539fc8050b632fffcf5fab96922b5e9bc83)

- **hub:** Ingest auth cache and structured denial logs [d987846](https://github.com/platformrelay/kollect/commit/d987846e4b0e378c7133b6ddeec25de539c45a9d)

- **controller:** Parallel sinks, debounce metrics, hub coalesce [e34a482](https://github.com/platformrelay/kollect/commit/e34a482d10dea8f316774de2bc328003031fccfb)

- **sink:** Backend pool cache and envelope export path [d238c8c](https://github.com/platformrelay/kollect/commit/d238c8cca54c9c4fd03ccb2dee96c3f1fddabdc7)

- **controller:** Add inventory deletion finalizer [128f1a7](https://github.com/platformrelay/kollect/commit/128f1a7372fe484b4382af3d5e6ed5c9688fbb7e)

- **sink:** Add per-sink gobreaker circuit breaker [e136296](https://github.com/platformrelay/kollect/commit/e136296e87fdd5e67884743f4a66f7e40ff98bb1)


### Refactoring

- **collect:** Namespace-scoped store watch driver [4f1210c](https://github.com/platformrelay/kollect/commit/4f1210cc55af2591cc046a949d72c1b017a9eea3)

- **arch:** Resolve arch-04, arch-11, arch-12 [1709158](https://github.com/platformrelay/kollect/commit/17091585ea0f5e199f29c96173bd80ae180360c4)

- **docs:** Phase 1 root doc moves [fcbaca0](https://github.com/platformrelay/kollect/commit/fcbaca084f1c1ace15645f970b5842f31c303396)

## [0.1.0-rc.2](https://github.com/platformrelay/kollect/compare/v0.1.0-rc.1..v0.1.0-rc.2) - 2026-06-05

### Bug Fixes

- **sink:** Gitlab HTTP client timeout [2c1377c](https://github.com/platformrelay/kollect/commit/2c1377c06cc6cc64f637660d35096f92a85db49c)

- **sink:** Postgres connect uses request context [3590d22](https://github.com/platformrelay/kollect/commit/3590d2270cd377e8c25a99b3f4bd7f78e194277c)

- **collect:** Degrade target on SAR API error [9ada555](https://github.com/platformrelay/kollect/commit/9ada555dc19e32d12857a2b532c01b5cc1c11243)

- **hub:** Rollback merge when export fails [6065122](https://github.com/platformrelay/kollect/commit/60651222e8246cdc3352c2b529f9f9ea64005a43)

- **controller:** Requeue conflicts and log map errors [a9cd353](https://github.com/platformrelay/kollect/commit/a9cd353e67f84f8a284e0b90a21292f57d8be450)

- **sink:** Close backends and log close errors [a52d779](https://github.com/platformrelay/kollect/commit/a52d779df204b736d2d1719519b8e89a2f027ef2)

- **transport:** Commit Kafka offset on handler success [e15ef5b](https://github.com/platformrelay/kollect/commit/e15ef5bf78559521b88013b98db1c3d9debd1f37)

- **spoke:** Retain delta until publish succeeds [42fe240](https://github.com/platformrelay/kollect/commit/42fe240e3326e74274aea674833230037f77013d)

- **sink:** Validate git CLI args before exec [0625bcc](https://github.com/platformrelay/kollect/commit/0625bcc3ff04985926f06956ce95d5e7e5f86648)

- **demo:** Satisfy OpenSSF Scorecard in kind-wide-scope [78be170](https://github.com/platformrelay/kollect/commit/78be17053024e8c7d1b150f175fd1b2c6bc13853)

## [0.1.0-rc.1](https://github.com/platformrelay/kollect/compare/v0.0.4..v0.1.0-rc.1) - 2026-06-05

### Bug Fixes

- **git:** Set bare HEAD after file-remote push [e591f74](https://github.com/platformrelay/kollect/commit/e591f743faa38bbb05f56a96ff932f370f43084e)

- **inventory:** Extract degraded status goconst [9b264c2](https://github.com/platformrelay/kollect/commit/9b264c26ae6c293e53b39e18befd0460819b3b4d)

- **sink:** Use git.TypeName for goconst CI lint [1d5fd48](https://github.com/platformrelay/kollect/commit/1d5fd489c2d00d843abb10a616b685ce8a564059)

- **demo:** Survive Step 2 bootstrap failures [4561c6a](https://github.com/platformrelay/kollect/commit/4561c6a2bc16bb6be5344206e77ed6fe2e79dda8)

- **demo:** Continue past prerequisite check [b3ed1c2](https://github.com/platformrelay/kollect/commit/b3ed1c2e1efe87c2bee3d1c214f5676cf7312162)

- **chart:** Restrict PrometheusRule alerts to kollect metrics [818af18](https://github.com/platformrelay/kollect/commit/818af18b1d02910b26a1f00d70dc8ce71f858d7a)

- **ui:** Exclude Playwright specs from Vitest runner [c6f250d](https://github.com/platformrelay/kollect/commit/c6f250d73d8b57ece1c558be3a6171862bea57f9)

- **ui:** Align inventory drawer and badge props with merged API [cfad0ba](https://github.com/platformrelay/kollect/commit/cfad0ba0c4bae79550a35412ff014d2014172e41)

- **ui:** Align inventory drawer with merged status APIs [00014bc](https://github.com/platformrelay/kollect/commit/00014bca99b49da58a6e361c31f04222962399d2)

- **lint:** Extract goconst strings for CI golangci-lint [f974805](https://github.com/platformrelay/kollect/commit/f974805e9c5ca1e5bbd54b885402b8cfe3bdd280)

- **chart:** Mount writable /tmp for git export [d605752](https://github.com/platformrelay/kollect/commit/d605752ea2dc895e206d06c3b08c17979f8cae6d)

- **sink:** Harden git export paths and command args for CodeQL [0493418](https://github.com/platformrelay/kollect/commit/04934180fbdfe4a965a21b47839e41b8001fdda4)

- **ci:** Add RBAC audit and expand fuzz gates [e538490](https://github.com/platformrelay/kollect/commit/e538490889d7201e660e6e5141ffe0565293a74c)

- **supply-chain:** Address OpenSSF Scorecard findings [27cb90f](https://github.com/platformrelay/kollect/commit/27cb90fa26609bdc4dcc1988c6d15790fd5e1715)

- **docs:** Restore mkdocs nav for reference hub pages [0f01235](https://github.com/platformrelay/kollect/commit/0f01235f8808fc231cae2840015c672dc6e7bbbb)

- **docs:** Drop mkdocs nav to uncommitted pages [3e49484](https://github.com/platformrelay/kollect/commit/3e49484c9ab7479cbebf595e797a91721f64b8a6)

- **e2e:** Recreate unhealthy kind clusters [8d4b654](https://github.com/platformrelay/kollect/commit/8d4b654d050c9bb68ae8c8e28e48c8896c33b27f)

- **ci:** Harden workflows for OpenSSF Scorecard [1fe5aa4](https://github.com/platformrelay/kollect/commit/1fe5aa4208f366bc6e4394df9670f5c76a6541c0)

- **security:** Harden inventory auth and SAR caches [4ab89e3](https://github.com/platformrelay/kollect/commit/4ab89e3d5a54c60cb0eff13c26bf5c317780b54e)

- **ci:** Use codecov-action v5 tag instead of bad SHA [0c0247d](https://github.com/platformrelay/kollect/commit/0c0247dbd32720536eb99b9d11654ac23d6f67c0)

- **ci:** Restore 60% coverage floor for test job [6fe3ed7](https://github.com/platformrelay/kollect/commit/6fe3ed724c6017b858eeb8d9053af1b831121e84)

- **docs:** Repair open questions list rendering [4bd0486](https://github.com/platformrelay/kollect/commit/4bd048637b8c1a482ee3110a987197f0472e64e4)

- **ci:** Perf-report envtest gate and changelog [147a16c](https://github.com/platformrelay/kollect/commit/147a16c956e81178202df95cca95f50ffd42e03c)

- **ci:** Lll wrap, coverage floor, and changelog [04e854e](https://github.com/platformrelay/kollect/commit/04e854e4a24fcb63c4fa2cb0147b269ad092b478)

- **ci:** Pin scorecard-action to commit SHA [86e3217](https://github.com/platformrelay/kollect/commit/86e32179511c4da4c64636533a7d6074c03decf4)

- **docs:** Repair attribute extraction mermaid flowchart [abc875e](https://github.com/platformrelay/kollect/commit/abc875ec02421aaa50fe8c8a6c86196323ce33b9)

- **docs:** Restore material icon rendering [fdc6296](https://github.com/platformrelay/kollect/commit/fdc6296e8e132f8bf56ee02951c41ab337323f67)

- **ci:** Resolve goconst lint and codecov action pin [914ddd7](https://github.com/platformrelay/kollect/commit/914ddd7f313d490113ffe81fc0ffd38d18261083)

- **ci:** Seed Certificate before team-certificates target [00095b9](https://github.com/platformrelay/kollect/commit/00095b96f578468218e710a7eae79e81906e08e0)

- **ci:** Poll tenant inventory itemCount in multitenant e2e [50953e1](https://github.com/platformrelay/kollect/commit/50953e109107ed592bc2b430d3fbda6b70a3818d)

- **ci:** Seed cert-test namespace before Certificate target [7e15f6c](https://github.com/platformrelay/kollect/commit/7e15f6ceaf508728631ac12b2204a0d732e37612)

- **ci:** Skip git export clone without GIT_EXPORT_TEST_REPO [c780773](https://github.com/platformrelay/kollect/commit/c78077351dd3c1db49b2436fd462dab3dfdd3844)

- **ci:** Assert cert collection via Ready message not itemCount [ced6d28](https://github.com/platformrelay/kollect/commit/ced6d284c6684912854de450f50788f98aa0575f)

- **ci:** Create cert-test namespace before target registration [497cf32](https://github.com/platformrelay/kollect/commit/497cf3275fc64abdd391affa07138e5946967d31)

- **ci:** Poll cert-manager target itemCount in e2e [7e2fdb4](https://github.com/platformrelay/kollect/commit/7e2fdb45eeb6c4ec7e055ac6f45e3d24a734fb9b)

- **helm:** Grant cert-manager Certificate list/watch for generic CRD e2e [eb32441](https://github.com/platformrelay/kollect/commit/eb32441c53c38c1063f193bd49f9baa2fc5a46f6)

- **ci:** Poll inventory HTTP for cert-manager e2e [cfcc9ca](https://github.com/platformrelay/kollect/commit/cfcc9ca07568bf9cbf051eb05785bc2a6c3b9ef5)

- **ci:** Wait for manager controllers before e2e smoke [941f2b5](https://github.com/platformrelay/kollect/commit/941f2b5bffe58c88c656b46e140a67338b06d91a)

- **ci:** Export env before cert-manager e2e subprocess [839ac6d](https://github.com/platformrelay/kollect/commit/839ac6dfde1943af8283d1512fedb8e48037f595)

- **helm:** Sync manager ClusterRole with kubebuilder RBAC [8036e9c](https://github.com/platformrelay/kollect/commit/8036e9c132d7dfcf5a65c2ae29e455aa2d388f0e)

- **ci:** Apply e2e samples directly without kustomize parent refs [aa46947](https://github.com/platformrelay/kollect/commit/aa46947af41ae42269e9938fdf5388b21e08194d)

- **ci:** Use lean e2e samples without unreachable sinks [ce757ae](https://github.com/platformrelay/kollect/commit/ce757aee53c7f0b86a868ba1792b5384eb5e06bd)

- **api:** Drop required status on KollectConnectionTest create [4f98d27](https://github.com/platformrelay/kollect/commit/4f98d27bce8a6f31c058e839be629474c877767e)

- **ci:** Repair Go 1.26 internal coverage profile merge [8f25a28](https://github.com/platformrelay/kollect/commit/8f25a28ba459bb8df531983fc3e3d9b80412b24e)

- **ci:** Skip cmd packages in coverage pre-pass [1053e74](https://github.com/platformrelay/kollect/commit/1053e74cc4b2c9bb668945b22ae26b2a5ad13340)

- **controller:** Gate validating webhooks when chart disables TLS [fff187d](https://github.com/platformrelay/kollect/commit/fff187d9c9512450ebeb7a8ed30b0427c10e582c)

- **ci:** Merge internal coverage profile without -p 1 [707216e](https://github.com/platformrelay/kollect/commit/707216e0c8f182c21eca81fb940b8065959f7224)

- **ci:** Stabilize coverage profile merge and cmd skip [6462139](https://github.com/platformrelay/kollect/commit/6462139ea78be652b2e5cb77633e08ce90e13b21)

- **ci:** Raise kind e2e helm install wait timeouts [b197499](https://github.com/platformrelay/kollect/commit/b1974998510650151ba0482be952b7d82422df16)

- **controller:** Stabilize cluster target and inventory envtests [56e3c1d](https://github.com/platformrelay/kollect/commit/56e3c1d35c3304fa42229237916bba4ad2cadc18)

- **sink/git:** Simplify file remote push and clone tests [3737454](https://github.com/platformrelay/kollect/commit/37374544b5237ad5143099348527887b9026896e)

- **ci:** Pin git export tests to main branch [7abe06e](https://github.com/platformrelay/kollect/commit/7abe06e8ecc4722325b14c684fcfa198c32c348f)

- **sink/git:** Reset workdir after failed file clone [acf644a](https://github.com/platformrelay/kollect/commit/acf644a9d2ce2a7965f9167f5e52a5f79d57e5c1)

- **sink/git:** Use native git for file:// export path [f6c58f1](https://github.com/platformrelay/kollect/commit/f6c58f10caf6bdd3f0ed35f3fbf310feab81b0ff)

- **sink/git:** Write export payload via worktree FS [6eb020c](https://github.com/platformrelay/kollect/commit/6eb020c8d541b336d976b2c8159264c12318108e)

- **release:** Use v0.0.4 MVP anchor, reserve v0.1.0 for publish [11fc310](https://github.com/platformrelay/kollect/commit/11fc31028a7320a47c3b6785289f415909e103a5)

- **sink/git:** Native push for file remotes on CI [78be033](https://github.com/platformrelay/kollect/commit/78be0330214367d35812582c0dadce56a8318312)

- **sink/git:** Force-push when bare clone has no HEAD [53e34cc](https://github.com/platformrelay/kollect/commit/53e34cc8d6bdc1baa343360868018accfc466b62)

- Address code review P0/P1 findings [7423030](https://github.com/platformrelay/kollect/commit/7423030c3995b93da665b30fea9f24201c2eec7b)

- **validation:** Require ClusterTarget namespaceSelector [4669000](https://github.com/platformrelay/kollect/commit/46690004fa67a5bbefece8981e6ae4751b05560d)


### Features

- **api:** Per-sink export interval scheduling [83c7085](https://github.com/platformrelay/kollect/commit/83c7085d1722475757eae776587268720103bd8d)

- **demo:** Venue pitch personas, fast churn, UI reveal [674a336](https://github.com/platformrelay/kollect/commit/674a3367e26f27282285f0f3e4a75e0f1769d02a)

- **chart:** Add Prometheus Operator monitoring [92a682a](https://github.com/platformrelay/kollect/commit/92a682a2aeb43974c81810ae619d72bc0076bdf9)

- **api:** Default KollectSink connectionTest to true [528c5b2](https://github.com/platformrelay/kollect/commit/528c5b29a1b3b1d82a334f00a6468efc8be39a39)

- **ui:** Merge inventory MVP with filters and virtualization [f63f6fd](https://github.com/platformrelay/kollect/commit/f63f6fd4222e9f89bcc25f9822d726ae0d64b99a)

- **demo:** Refactor wide-scope kind showcase [c944750](https://github.com/platformrelay/kollect/commit/c9447502e508dcdb501765b088ed37da106cdeb3)

- **ui:** Wire inventory rows to detail drawer [fc99b52](https://github.com/platformrelay/kollect/commit/fc99b52b8f92b306869d13e4818ed2f36c7d94aa)

- **ui:** Add inventory MVP with filters and virtualization [d27069d](https://github.com/platformrelay/kollect/commit/d27069d12e4decc9ad23f94b5100b80d586f6da9)

- **ui:** Merge overview degraded strip and export stats [e921580](https://github.com/platformrelay/kollect/commit/e9215804894ca21a957b823bd3c22f89e4eefffd)

- **ui:** Merge detail drawers for targets and sinks [c19a410](https://github.com/platformrelay/kollect/commit/c19a410ef070b92e3d2d43cbada92c95a00ada08)

- **ui:** Add detail drawers for targets and sinks [8752432](https://github.com/platformrelay/kollect/commit/8752432c4a04e6307bdc9e58c20700091b875f5b)

- **sink:** Add git push policy branch and auth options [90319b2](https://github.com/platformrelay/kollect/commit/90319b213e3970cf8f86cf23edb6049827eb0464)

- **ui:** Implement Phase 1 mock Read API (MSW + Prism) [6b0485c](https://github.com/platformrelay/kollect/commit/6b0485c8f159f64cb01fb546e542104896e2659c)

- **inventory:** Extend Read API for UI contract [d1f0871](https://github.com/platformrelay/kollect/commit/d1f087102a6995835b6b27d4dc2d1a656e937daf)

- **api:** Add Target collection filtering (ADR-0207) [8c63fcb](https://github.com/platformrelay/kollect/commit/8c63fcb677f790c9bf98aa96184e62a81b2086f0)

- **export:** Enforce object-store spill above 1 MiB [938f63d](https://github.com/platformrelay/kollect/commit/938f63d30bce829d11d43307e7b642a80a64b502)

- **api:** Add KollectSink spec.pathTemplate [31004c5](https://github.com/platformrelay/kollect/commit/31004c504c98357174eb2c4e772032edc98c71b9)

- **sink:** Add S3/GCS Parquet snapshot export [8704e76](https://github.com/platformrelay/kollect/commit/8704e761efa47e49c8178151fa48519d101281bf)

- [**breaking**] Remove KollectHub reconciler [3c30605](https://github.com/platformrelay/kollect/commit/3c306052c1a01720f37e19ac482b48678c3e3703)

- [**breaking**] Remove KollectHub CRD surface [6470a34](https://github.com/platformrelay/kollect/commit/6470a34ee66f932e990a95e169bfd7575ec468fa)

- **webhook:** Validate KollectSink spec.type enum [5b509e7](https://github.com/platformrelay/kollect/commit/5b509e73a77c87ec8122628dd4d14578b8ac1e3e)

- **api:** Add KollectClusterInventory spec.dedupe [e8bf743](https://github.com/platformrelay/kollect/commit/e8bf74398b69b8b79d5f21ffcc14d991a3ef0ef9)

- **export:** Add schemaVersion inventory envelope [e115d27](https://github.com/platformrelay/kollect/commit/e115d27f486cdbc086a28f011101ce0fad229b5f)

- **sink:** Add NATS JetStream event sink backend [135021c](https://github.com/platformrelay/kollect/commit/135021cc8a5de549d64df2cb8aa66e350649a7bf)

- **export:** Add schemaVersion to export envelopes [0b69ae9](https://github.com/platformrelay/kollect/commit/0b69ae99830dae57035e2c46462ac2ac53d59001)

- **sink:** Add Capabilities and postgres delete reconciliation [119e861](https://github.com/platformrelay/kollect/commit/119e861f4dd502adb1dc1bb1242cc70239f712c3)

- **controller:** Wire cluster inventory aggregate dedupe [9e87710](https://github.com/platformrelay/kollect/commit/9e87710738adeb7003bb944cdd8b1c562af8f5fd)

- **aggregate:** Add cross-target dedupe spike stub [1592473](https://github.com/platformrelay/kollect/commit/15924733d6ae724690e746b7280443e779850241)

- **sink:** Push gitlab exports to feature branch in mr mode [2f1f2df](https://github.com/platformrelay/kollect/commit/2f1f2df017fab82f594380a8fe33c928d597b503)

- **collect:** Emit prometheus label values from profile metrics [033fa15](https://github.com/platformrelay/kollect/commit/033fa15e7084bdf918207ce825f26a735d35d93a)

- **collect:** Wire profile metrics paths and hub merge metric [fbdd059](https://github.com/platformrelay/kollect/commit/fbdd059f695813cf933277943af1b160832de7c2)

- **api:** Add KollectProfile.spec.metrics spike [25fc642](https://github.com/platformrelay/kollect/commit/25fc642c6fa12afdce231a55c05457d70f8e3131)

- **collect:** Wire RecordCustomResourceSeries on snapshot [fcf0c5d](https://github.com/platformrelay/kollect/commit/fcf0c5d0bedd0ac07332fcb8d982f3bf9d157386)

- **collect:** Add Phase 4 aggregation metrics stub [1806597](https://github.com/platformrelay/kollect/commit/1806597daf0973d0ecb961c93eaad2d1070384ff)

- **sink:** Add GitLab API v4 merge request client [6af3630](https://github.com/platformrelay/kollect/commit/6af3630eff335fcef7f0cd655970e471fea4f89c)

- **collect:** Complete ADR-0020 metrics catalog [9c940b1](https://github.com/platformrelay/kollect/commit/9c940b14993d7688d0518db5fc3a1c9e9bf75941)

- **sink:** Add gitlab mergeRequest CRD and transport ACL wire [304c50d](https://github.com/platformrelay/kollect/commit/304c50d23073dfc586d38dc488a83c1b016f03a2)

- **controller:** Wire cluster inventory export to sinks [18ed358](https://github.com/platformrelay/kollect/commit/18ed35802631497b5946e285e30ba1b03327431f)

- **transport:** Add queue wire ACL allowlist stub [83beaf5](https://github.com/platformrelay/kollect/commit/83beaf56d868e67ff9b8ae88024157821c6929d8)

- **controller:** Add cluster target and inventory skeletons [741d35f](https://github.com/platformrelay/kollect/commit/741d35f275db06d6c018b67e0a085bb7d0747944)

- **sink/gitlab:** Scaffold GitLab export backend [b2c1527](https://github.com/platformrelay/kollect/commit/b2c152717ef9fc4bcf75784ea74710486cd77f75)

- **hub:** Parallel Postgres+Kafka export on ingest [d5a97e4](https://github.com/platformrelay/kollect/commit/d5a97e47a5d7c215f3fbd8a8045284548db50663)

- **api:** Add KollectClusterProfile CR [ee6ff58](https://github.com/platformrelay/kollect/commit/ee6ff58bd49c87c956f58aba48827308fc089618)

- **api:** Add KollectClusterInventory CR [91465f2](https://github.com/platformrelay/kollect/commit/91465f2fdd890b3accb51d0ee353084758a41584)

- **api:** Add KollectClusterInventory CR [d7eb87a](https://github.com/platformrelay/kollect/commit/d7eb87aff77f6cde6b96c115b8410959dc919091)

- **api:** Add KollectClusterTarget CR and webhook [2c7d6a9](https://github.com/platformrelay/kollect/commit/2c7d6a98433ca9f8131131d4bda1e662ead4ade9)


### Refactoring

- **controller:** Remove --export-debounce flag [3d5b729](https://github.com/platformrelay/kollect/commit/3d5b7291f01abe6bc03cceb4c626836e4b0455c8)

- Unify bearer auth and brittle error assertions [13fdc78](https://github.com/platformrelay/kollect/commit/13fdc7888098c5ea86e3ec01489ed2d5a2a5b18c)

- **sink:** Extract shared RunExportItems pipeline [3762989](https://github.com/platformrelay/kollect/commit/3762989f27adc203853f66b4e12c142347a0734b)

- **controller:** Extract cluster inventory export path [535cca4](https://github.com/platformrelay/kollect/commit/535cca47a7f5e66d4b8d37c6fcfb4eb79a26713f)

## [0.0.4](https://github.com/platformrelay/kollect/compare/v0.0.3..v0.0.4) - 2026-06-05

### Bug Fixes

- **inventory:** V1alpha1 HTTP paths and export caps [796168e](https://github.com/platformrelay/kollect/commit/796168ebf57340d5ef89412051f16d8fc666e08e)

- **collect:** Avoid startInformer mutex deadlock [7b614f5](https://github.com/platformrelay/kollect/commit/7b614f5045b80ba4478982b93d0a3e14ab3f1ff1)

- **obs:** Standalone perf-report shell script [98d6d33](https://github.com/platformrelay/kollect/commit/98d6d33041660883ee813badb971ae6d53d6d647)


### Features

- **api:** Add KollectConnectionTest CR [b1036cd](https://github.com/platformrelay/kollect/commit/b1036cd779f4a8bd62039a9f8515836ef917de4d)

- **inventory:** Export debouncing with checksum [377b091](https://github.com/platformrelay/kollect/commit/377b0912d4aa9f0720949e8c64d0ae6958808d43)

- **api:** [**breaking**] Namespaced KollectSink and same-ns sinkRefs [687c4c7](https://github.com/platformrelay/kollect/commit/687c4c7311c416ac0f09c5f8eee2c4d84aa1a804)

- **transport:** Queue TLS and hub ACL hardening [65caa30](https://github.com/platformrelay/kollect/commit/65caa30f68b5b74e9bb1e9633096c41bb92b889e)

- **controller:** SinkReachable export and probe cleanup [92598d9](https://github.com/platformrelay/kollect/commit/92598d95cf4f5f0e1fbc61cbcb2ac8fefb536c68)

- **operator:** Add hub and spoke mode flag [d2bc4da](https://github.com/platformrelay/kollect/commit/d2bc4dacedf2339967468b41a5845e459baeb786)

- **api:** [**breaking**] Make KollectProfile namespaced [64687aa](https://github.com/platformrelay/kollect/commit/64687aaac778404a965a8582fa62a9d49b4dc60c)

- **scope:** Enforce KollectScope in reconcilers [60e79ce](https://github.com/platformrelay/kollect/commit/60e79ce65d0d90e6b88f0698329be0d51988d1a2)

- **hub:** Wire remoteClusters and credential pull [d1ca137](https://github.com/platformrelay/kollect/commit/d1ca137d1ebda0f68b5c1ece5fea655b5fb1c9f4)

- **cli:** Add create-remote-secret stub [ef6ec64](https://github.com/platformrelay/kollect/commit/ef6ec64b56687afa29ef5ede3888bb914765c1b6)

- **hub:** Add SAR on ingest auth [da8e632](https://github.com/platformrelay/kollect/commit/da8e632bef00eb3bb9c302e53b83454fe57c4bdc)

- **hub:** Remote cluster Connected and queue wire auth [203509d](https://github.com/platformrelay/kollect/commit/203509dc1f373a46da22209066520f43f7c297da)

- **collect:** Namespace and resource watch labels [156e196](https://github.com/platformrelay/kollect/commit/156e196804d35dbc144636a1aa773ca5f8ad1132)

- **hub:** Spoke push auth via TokenReview [8ef9bef](https://github.com/platformrelay/kollect/commit/8ef9befa6b46beeb118fcee7d6de0eb33f200620)

- Namespaced multi-tenant operator support [e2020df](https://github.com/platformrelay/kollect/commit/e2020df5ac9d9a9bb91effe36237cd420084ab2b)

- **spoke:** Delta publish with transport reuse [e75a5d9](https://github.com/platformrelay/kollect/commit/e75a5d9b02a7cf032954cea3244fa3444d7c5b46)

- **hub:** 100-spoke merge spike and delta removals [aeba1f3](https://github.com/platformrelay/kollect/commit/aeba1f31141acd6ac8e3585cc3a888440701cffa)

- **transport:** Add NATS JetStream backend [d981a90](https://github.com/platformrelay/kollect/commit/d981a900437fcef0a428445abec0001ffa23ca07)

- **hub:** Wire consumer mode and spoke publish stub [9a3940e](https://github.com/platformrelay/kollect/commit/9a3940e9c7650dd7d28e0d94b20649e2de114bb0)

- **hub:** Spoke report merge consumer [da9daa9](https://github.com/platformrelay/kollect/commit/da9daa9501c77c947a7291b7f7d2bd3b24a9e38e)

- **transport:** Kafka backend with redpanda tests [5ef18a7](https://github.com/platformrelay/kollect/commit/5ef18a78bcbf4cc43406805ba2bd18d76f79e807)

- **obs:** Task perf-report and metrics catalog [4de96e8](https://github.com/platformrelay/kollect/commit/4de96e810b4a9217fd1b451569a04d51e5a5eae8)

- **perf:** Parallelism, metrics, and pprof [3da5ff8](https://github.com/platformrelay/kollect/commit/3da5ff84ace7754ca143129a72b902708135dcac)

- **sink:** Postgres and kafka export backends [fa8aeb5](https://github.com/platformrelay/kollect/commit/fa8aeb50ad40ee250cf88b8d532289f516b7534f)

- **sink:** GCS backend and prometheus stub [7bde2ba](https://github.com/platformrelay/kollect/commit/7bde2ba420e8aba1f078e6235e7a8be41c4859d6)

- **api:** KollectHub and KollectScope CRDs [ae90224](https://github.com/platformrelay/kollect/commit/ae90224d7045e4f3abeb408d3df489e2922f7bce)

- **transport:** Pluggable factory with Redis Streams [d08d612](https://github.com/platformrelay/kollect/commit/d08d61206624a752b194687055736fb2c400d7ff)

- **metrics:** Complete ADR-0020 operator metric set [0e14c05](https://github.com/platformrelay/kollect/commit/0e14c05359ed185caba89641eb2072a7310e788c)

- **collect:** SAR degradation and namespaceSelector [39ae427](https://github.com/platformrelay/kollect/commit/39ae427eccd8a9faa3f2e9500c9b59b6a0518a4e)

- **inventory:** Store-backed HTTP API and K8s auth [ee03b72](https://github.com/platformrelay/kollect/commit/ee03b7204a02c1f4815aaad5beecf7c9dd912277)

- **controller:** Wire collection and inventory export [b40252f](https://github.com/platformrelay/kollect/commit/b40252ffd041371e54ab5506c93614617964c619)

- **transport:** Add in-process pub/sub bus [9154c22](https://github.com/platformrelay/kollect/commit/9154c22709d5b5265ea6fd7e8963730ad9a7949a)

- **collect:** Add dynamic informer engine and store [666bded](https://github.com/platformrelay/kollect/commit/666bded58c64bfadbfe557e550506e2d2ab28b15)

- **sink:** Add git export and s3 PutObject backends [8157739](https://github.com/platformrelay/kollect/commit/81577397c2190332b704f4eed3bbd5bd483e469c)

- **inventory:** Add toggleable HTTP endpoint and metrics [1b6f907](https://github.com/platformrelay/kollect/commit/1b6f9077518ad9e87603dbaaf54804199c8d94d3)

- **sink:** Add connection test with TLS CA support [dcc6e48](https://github.com/platformrelay/kollect/commit/dcc6e48174983fe5e58b79a41eaaa10902251939)

- **webhook:** Validate Profile CEL and JSONPath paths [6329e86](https://github.com/platformrelay/kollect/commit/6329e8645a7cf6bb7eaf2394a3fa5712ece134f3)


### Refactoring

- **hub:** Deprecate KollectHub controller [6cf1bb3](https://github.com/platformrelay/kollect/commit/6cf1bb3b32e975b57bab161b091402d83c3a94ce)

- **collect:** Store Len and reconcile metrics [09e1ca0](https://github.com/platformrelay/kollect/commit/09e1ca0c0a7a63306c72f95e4c9e51c390760d74)

- **api:** [**breaking**] Make KollectInventory namespaced [1db8ed6](https://github.com/platformrelay/kollect/commit/1db8ed68602c086c19d4d8581966d659b3770896)

## [0.0.3](https://github.com/platformrelay/kollect/compare/v0.0.2..v0.0.3) - 2026-06-05

### Bug Fixes

- **build:** Use repo-root kustomize in deploy task [6812cee](https://github.com/platformrelay/kollect/commit/6812cee8b46094b7b9391cdad94eb5a9cd484381)

- **build:** Move scrub patterns out of Taskfile [023da6f](https://github.com/platformrelay/kollect/commit/023da6f64edd4409835867203a8c431ff7740c50)

- **test:** Satisfy ginkgolinter and shorten status comments [d6cf08c](https://github.com/platformrelay/kollect/commit/d6cf08c16d09eebfcabc5d2e4480c020035baa11)


### Features

- **helm:** Add kollect operator chart [9b3e0da](https://github.com/platformrelay/kollect/commit/9b3e0da2bb35e2a864899e06da4de5d390d65190)

- **api:** Add sink TLS and inventory HTTP fields [fba40b7](https://github.com/platformrelay/kollect/commit/fba40b7fd7d284d69db22488721485c393357330)

- **controller:** Validate KollectTarget profileRef [51a3c51](https://github.com/platformrelay/kollect/commit/51a3c51225f25113d139168117f230c71869ed34)

- **sink:** Add backend registry with git stub [472800c](https://github.com/platformrelay/kollect/commit/472800c0037d328aa31a942e79185651d01a158f)

- **collect:** Add CEL and JSONPath extractor [8b079e4](https://github.com/platformrelay/kollect/commit/8b079e49cf7bdfa4248ca877ef1e0cd35b90cfe5)


### Refactoring

- **samples:** Consolidate on kollect_v1alpha1_* set [8c6263d](https://github.com/platformrelay/kollect/commit/8c6263dda34ab3cc6a182f2cb244bff00601faed)

## [0.0.2](https://github.com/platformrelay/kollect/compare/v0.0.1..v0.0.2) - 2026-06-04

### Features

- **api:** Add KollectProfile/Sink/Target/Inventory v1alpha1 types [5f5866f](https://github.com/platformrelay/kollect/commit/5f5866fa72b05e4a4b961b56a5ec9da780af7e37)

## [0.0.1] - 2026-06-04
