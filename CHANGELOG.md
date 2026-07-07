# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Release notes are generated from [Conventional Commits](https://www.conventionalcommits.org/)
on the default branch using [git-cliff](https://git-cliff.org/).

## [Unreleased]

### Bug Fixes

- **controller:** Requeue on family-sink status conflict instead of dropping it [d1a9e24](https://github.com/konih/kollect/commit/d1a9e24f29c554c4dfd51f963d8a436c85350e6a)

- **collect:** Block on dispatch backpressure instead of inline sync fallback [a3e8efb](https://github.com/konih/kollect/commit/a3e8efb4757bd469b75241e17a0633c49d58bd2d)

- **controller:** Degrade scope not whole target on RBAC-forbidden namespace (#28) ([#28](https://github.com/konih/kollect/pull/28))[0cd7e8d](https://github.com/konih/kollect/commit/0cd7e8dd3e648471aae59e2bd371d5c73f9d3941)


### Features

- **controller:** Surface extraction failures in target status (EC-P1-05) (#36) ([#36](https://github.com/konih/kollect/pull/36))[f5186ea](https://github.com/konih/kollect/commit/f5186ea519bb767b1795b1548bdb2fc9584f4b4b)

- **metrics:** Bound profile label cardinality (EC-P2-09) (#32) ([#32](https://github.com/konih/kollect/pull/32))[51e1f4b](https://github.com/konih/kollect/commit/51e1f4b26e6e821ecf46ecdb3eb9287d7b9b0266)

- **scope:** Cluster static-ref namespace allowlist (#23) ([#23](https://github.com/konih/kollect/pull/23))[c55f538](https://github.com/konih/kollect/commit/c55f5380d99ab9cf30dbd3f4122c18f6ff4fb3dc)


### Performance

- **controller,collect:** Incremental namespace fingerprint cache (AR-10) (#37) ([#37](https://github.com/konih/kollect/pull/37))[f28b93e](https://github.com/konih/kollect/commit/f28b93ee20f86e2ad2e2f008fc70535b1eaba599)


### Refactoring

- **sink/bigquery:** Typed sentinel errors for Export failure stages (F66) [6790065](https://github.com/konih/kollect/commit/679006591cbf58338397302650086af9d8333a11)

- **sink:** Extract shared secret-value-from-keys helper (dup audit) (#38) ([#38](https://github.com/konih/kollect/pull/38))[2f166a5](https://github.com/konih/kollect/commit/2f166a5bdd3fa1b035f0a6f2a9e0ba7da3c05da1)

- **webhook:** Dedup ValidateDelete boilerplate across validators (#33) ([#33](https://github.com/konih/kollect/pull/33))[f147642](https://github.com/konih/kollect/commit/f147642d87a32cd002881c70b2d34d75c672c50c)

- **sink:** Extract inventoryFromObjectPath into pathvalidate (#29) ([#29](https://github.com/konih/kollect/pull/29))[123f6a5](https://github.com/konih/kollect/commit/123f6a580c976a99a3c31074b9043db105a3eea8)

## [0.7.0-rc.1](https://github.com/konih/kollect/compare/v0.6.0-rc.2..v0.7.0-rc.1) - 2026-06-13

### Bug Fixes

- **release:** Tolerate Rekor 409 on asset re-sign [94de025](https://github.com/konih/kollect/commit/94de025138b78d4130d079d820381a38b53462f6)

- **sink/nats:** Reconnect when cached connection is closed [319a2bf](https://github.com/konih/kollect/commit/319a2bf79da165da0c74ce02b5697a614e01268f)

- **sink/bigquery:** Snapshot emulator mode in Config [4c58988](https://github.com/konih/kollect/commit/4c58988b1e931c01a7169df6320b79c1f2d3ccd6)


### Features

- **webhook:** Reject cluster kinds in tenantMode [3dbfd6b](https://github.com/konih/kollect/commit/3dbfd6bb14fbcb24d162c6e3edf4aad21593e4a6)

- **controller:** Classify forbidden static refs for cluster kinds [ae3057d](https://github.com/konih/kollect/commit/ae3057ddf3946554cd246a6475fb18bc1c0f2a31)

- **api:** [**breaking**] Reference namespaced static config from cluster kinds [9753744](https://github.com/konih/kollect/commit/97537449b7fa94b52a11b72509c3a0fbc10e5ca2)

## [0.6.0-rc.2](https://github.com/konih/kollect/compare/v0.6.0-rc.1..v0.6.0-rc.2) - 2026-06-10

### Bug Fixes

- **test:** Drop duplicate family sink condition tests [f0bfb36](https://github.com/konih/kollect/commit/f0bfb36b9dbce465daf5773741a71a2f3ea8b76d)

- **build:** Fix go-arch-lint exclude paths for local dirs [4e443f6](https://github.com/konih/kollect/commit/4e443f62ad787577a286719ec9561b111a68355e)

- **validation:** Block private sink endpoint targets [f1c0d79](https://github.com/konih/kollect/commit/f1c0d7900f873c1557faccbc1fd7635ff105289a)


### Features

- **controller:** Shard snapshot exports by max bytes [d6ee50a](https://github.com/konih/kollect/commit/d6ee50a7ff5e53e670b1e020ad968d8a7284d3fa)


### Refactoring

- **bigquery:** Inject query execution adapter [0d2208d](https://github.com/konih/kollect/commit/0d2208d2cb0f83dd5ff30fb5b3e359faad75166a)

- **s3:** Isolate head-bucket check helper [12a47ea](https://github.com/konih/kollect/commit/12a47ea867988f958320867070138a7a7bb076c6)

- **postgres:** Narrow upsert tx interfaces [82e7bb1](https://github.com/konih/kollect/commit/82e7bb1a4e2829b1f84ff496eaea0a3a3f150ecc)

- **export:** Extract envelope partition helpers [391bbb8](https://github.com/konih/kollect/commit/391bbb8d625481dc7e16c370b73ea29f176aedf7)

- **mongodb:** Isolate export scope planning [51154e7](https://github.com/konih/kollect/commit/51154e7d2dca1509562f904d74253c96486afb56)

- **postgres:** Extract export planning helpers [1e0248f](https://github.com/konih/kollect/commit/1e0248fbfcce991fa4ab5a8fac105dd1dcb19f00)

## [0.6.0-rc.1](https://github.com/konih/kollect/compare/v0.5.0..v0.6.0-rc.1) - 2026-06-09

### Bug Fixes

- **sink:** Retry BigQuery emulator readiness in L3 tests [b5a6b85](https://github.com/konih/kollect/commit/b5a6b850e9653ddfd7ea9321260c9399ae182c6b)

- **controller:** Enforce namespace intersections in rollups [8e11ab6](https://github.com/konih/kollect/commit/8e11ab6993c282a28823c71d7469dc91be184742)

- **sink:** Re-enable backend pool after layout integration test [95a417c](https://github.com/konih/kollect/commit/95a417c19ee0e4be041b7b0b2c51667b4354048a)

- **sink:** Infer resource manifest layout [90725d6](https://github.com/konih/kollect/commit/90725d67500b5d0aef3392de422e735ae94c7e87)

- **build:** Compile full cmd package after pprof fold [5045839](https://github.com/konih/kollect/commit/50458391f52d6d35df2e080c642c8e14fe89d8c7)

- **controller:** Panic-guard family-sink, connection-test, cluster-target reconcilers [95f2cae](https://github.com/konih/kollect/commit/95f2cae91e0055a269cc0ddfd3d59ee0aad583ed)

- **controller:** Aggregate per-sink export errors [61d28d4](https://github.com/konih/kollect/commit/61d28d44b671029c97f441afe00fa23750cbe16c)

- **controller:** Stop requeue on terminal finalizer cleanup [439e5dd](https://github.com/konih/kollect/commit/439e5dd0e8cd5855b89a9ac4e81011fa3306b30c)

- **api:** [**breaking**] Reject stub sink types at admission [2ebc8df](https://github.com/konih/kollect/commit/2ebc8df3a0cd904136e45dbf2d599801bea214fa)

- **sink:** Remove stub backends, make unknown sink type terminal [f39d19c](https://github.com/konih/kollect/commit/f39d19c6846bf2afe4d2b5bfe25f9cc61e48f23d)

- **sink/git:** Redact credentials from git CLI errors [f25cafb](https://github.com/konih/kollect/commit/f25cafb67bc8f9080140136e1f03764587ad7ea5)

- **collect:** Stabilize export fingerprints for debounce [76813eb](https://github.com/konih/kollect/commit/76813eb84f3d7e2cb2ef9115939c69c97d68ccbc)

- **docs:** Purge stale hub/spoke from operator manual [5135c41](https://github.com/konih/kollect/commit/5135c418f6a9e8a5338b99700425367988da2c3a)

- **docs:** Repair broken links and stale ADR references [d8b37cb](https://github.com/konih/kollect/commit/d8b37cbb706560b6d69c9b315cf0ba75db331be9)

- **chart:** Sync family-sink CRDs with ADR-0416 fields [067c5c7](https://github.com/konih/kollect/commit/067c5c7008fb6ba2693a79a11a7d658796fca0f0)


### Features

- **api:** Add cluster rollup shard status [4809cfd](https://github.com/konih/kollect/commit/4809cfd995362af22c7b2709d5359c6db24b5345)

- **api:** Add cluster inventory namespaces list [3959ba2](https://github.com/konih/kollect/commit/3959ba241cc281c21b76d2ff0e60cc0edf46dc2d)

- **helm:** Add minimal RBAC team install profile [907487e](https://github.com/konih/kollect/commit/907487ed167ac744d8093c4508c263b2c2edbd01)

- **sink:** Add BigQuery database backend [83fb2f3](https://github.com/konih/kollect/commit/83fb2f31c17a966220c922eb8c15c08ea38f9b20)

- **sink/nats:** Version event envelope schema [48fd7dd](https://github.com/konih/kollect/commit/48fd7dd989d7dd20587917af6b064ab5f591200e)

- **demo:** Add hero harness with in-kind Forgejo [c64c14c](https://github.com/konih/kollect/commit/c64c14c5e64f893d30a9a12bce56c4501c943856)

- **sink:** Wire ADR-0419 git layout into export pipeline [12d7f80](https://github.com/konih/kollect/commit/12d7f8023223c7f20c4f3badcd0b57bbc017974f)

- **export:** Full-resource pruning and Git layout [3df4027](https://github.com/konih/kollect/commit/3df4027e0fc43970df9599996241693c0886fd86)

- **sink:** Render status.preview surface (ADR-0416) [5c9c80f](https://github.com/konih/kollect/commit/5c9c80f0798e8fdbe20848525e48ff085d29b274)

- **sink:** MongoDB database sink (ADR-0417) [f49bf3e](https://github.com/konih/kollect/commit/f49bf3e21deabb89f36f7c01ef96feba40449764)


### Refactoring

- **controller:** Compose cluster rollups by namespace [b1af6b1](https://github.com/konih/kollect/commit/b1af6b1022c9d9ee57baaf6699e9ab53f19e848b)

- **inventory:** Fold internal/httpauth into inventory [f89e2cd](https://github.com/konih/kollect/commit/f89e2cdb4955c8aaa2ab307f2137aa24d29b091c)

- **cmd:** Fold internal/pprof into cmd [8a3004f](https://github.com/konih/kollect/commit/8a3004fd6d2ed38916cc27b3a8ad8801e9a16469)

- **validation:** Decouple layout path checks from sink package [a558394](https://github.com/konih/kollect/commit/a558394204f2425a1ce27c5be8a5dfa47660827f)

## [0.5.0-rc.1](https://github.com/konih/kollect/compare/v0.4.1..v0.5.0-rc.1) - 2026-06-07

### Bug Fixes

- **build:** Upgrade alpine packages in UI image for Trivy [45d08ce](https://github.com/konih/kollect/commit/45d08ce46221541fbb3b35e00dc7a202488528be)

- **ci:** Stop perf-report writing agent-context in CI [5ee2088](https://github.com/konih/kollect/commit/5ee208848043b6749362b1d0ddc83ab0c2812fa9)

- **ui:** Disambiguate Playwright inventory row locator [f422a78](https://github.com/konih/kollect/commit/f422a78335266a150280d79d0b9c86b0c10b170d)

- **ci:** Stabilize nightly Playwright and perf-report [b6ea35f](https://github.com/konih/kollect/commit/b6ea35f25b6503bb90281277db54cd347664dea6)


### Features

- **api:** ADR-0416 sink config layering [dcc5fa9](https://github.com/konih/kollect/commit/dcc5fa9b1564af39a28c5ebe30534c26f1683a4e)

## [0.4.1](https://github.com/konih/kollect/compare/v0.4.0..v0.4.1) - 2026-06-07

### Bug Fixes

- **build:** Upgrade debian packages for Trivy gate [b6515c0](https://github.com/konih/kollect/commit/b6515c04e05107e5ec97c9020435f76943f5a68f)


### Features

- **sink/git:** PERF-10 mirror and fingerprint skip [e6288c1](https://github.com/konih/kollect/commit/e6288c174c32b69c38d0caec5676338ae6217fd0)

- **perf:** Scale lane PERF-08/09/15 and 10k load tier [dbda1c2](https://github.com/konih/kollect/commit/dbda1c2ab89ea1f5041638932124d7f34748fb88)

## [0.4.0](https://github.com/konih/kollect/compare/v0.3.0-rc.1..v0.4.0) - 2026-06-07

### Features

- **collect:** PERF-03 tunable dispatch pool [d3cfc5d](https://github.com/konih/kollect/commit/d3cfc5d121859780ad5fd588835b984947595bd1)

- **ui:** Align sink families and add UI docs ([#17](https://github.com/konih/kollect/pull/17))[bcca60d](https://github.com/konih/kollect/commit/bcca60d9d1bdfedb7595cfc3b1d57d9fbb6bfc68)


### Refactoring

- **collect:** GVR index and sharded store [2391a55](https://github.com/konih/kollect/commit/2391a551b41d6c2474a3c1eb7670c2e40c744fa3)

## [0.3.0-rc.1](https://github.com/konih/kollect/compare/v0.2.0-rc.1..v0.3.0-rc.1) - 2026-06-07

### Bug Fixes

- **controller:** Recover panics and suspend status [4cf28cf](https://github.com/konih/kollect/commit/4cf28cff4b647e208a8b5e2f9ec603081ad1e58b)

- **git:** Resolve lint issues in sink hardening [452c72b](https://github.com/konih/kollect/commit/452c72b2527c28d217c6b4a150e251f668e57284)

- **docs:** Remove extra blank line in git-sink-attribution [3dcc34b](https://github.com/konih/kollect/commit/3dcc34b7cd7ed4750a5883daec78092817a69078)

- **e2e:** Guard multitenant port-forward cleanup trap [d04f292](https://github.com/konih/kollect/commit/d04f2929233efd9b072cdce85d6d47c42386ad44)

- **e2e:** Use object form for snapshotSinkRefs [468bca9](https://github.com/konih/kollect/commit/468bca94aafa745c2e24250d6914baad5eeac7e4)

- **e2e:** Validate git-export and multitenant via HTTP [85d160a](https://github.com/konih/kollect/commit/85d160ad2cfb59a967697e2791f698bd4a80b5bd)


### Features

- **sink/git:** Rich commit context and templates [981a18a](https://github.com/konih/kollect/commit/981a18a1c27d8340368b69e5657e5676fc3c09e2)

- [**breaking**] Remove hub/spoke tier and KollectRemoteCluster [8c02b47](https://github.com/konih/kollect/commit/8c02b47e1d46228a397d717f908a5f60f44d2874)

- **git:** Harden export clone, auth, and push recovery [8dc426c](https://github.com/konih/kollect/commit/8dc426c358e9085e8af294c52fff7e2ba96cf590)


### Refactoring

- **perf:** P0/P1 export path optimizations [cb16812](https://github.com/konih/kollect/commit/cb168127b0da5700047176e71b5cf6a0cc4ce9c5)

## [0.2.0-rc.1](https://github.com/konih/kollect/compare/v0.1.0-rc.3..v0.2.0-rc.1) - 2026-06-07

### Bug Fixes

- **e2e:** Drop legacy sinkRefs from multitenant scope ([#14](https://github.com/konih/kollect/pull/14))[8f7a156](https://github.com/konih/kollect/commit/8f7a1565ff3cf06ac29debe91a54a4ec5047dd60)

- **e2e:** Drop removed inventory sinkRefs field [e912dbe](https://github.com/konih/kollect/commit/e912dbe384378c9b84cef4b6646ca51851a32df4)

- **e2e:** Validate collection via inventory HTTP [ca816a0](https://github.com/konih/kollect/commit/ca816a072b89326d0bc6c6a6dae5f4d2f0ec8159)

- **e2e,test:** Stabilize smoke bootstrap and debounce IT [3677bb5](https://github.com/konih/kollect/commit/3677bb5dc2b1685def8a7aec05f066604e18985c)

- **samples:** Drop legacy sinkRefs from team-a scope [1648ab3](https://github.com/konih/kollect/commit/1648ab3ce0b67e58d8129da909487db91bf3c23b)

- **e2e:** Wait on family sink CRDs in kind smoke [933dc36](https://github.com/konih/kollect/commit/933dc366f1343efc960932c56862b5b5061612e0)

- **gitlab:** Basic auth for Forgejo Gitea MR API [c2b0f3d](https://github.com/konih/kollect/commit/c2b0f3d528365385a3e60cef3a95e791c9210abe)


### Features

- **git:** Port transport retry and SSH host keys [9f5b17f](https://github.com/konih/kollect/commit/9f5b17f40d0eda7b8e482e2c03cb69aac66f370a)

- **controller:** Wire family sink reconcilers and export [8162345](https://github.com/konih/kollect/commit/8162345b399d75ae315d05e06bb25a43c7eb4e0f)

- **api:** Add sink family CRDs and remove KollectSink [0ec8ea2](https://github.com/konih/kollect/commit/0ec8ea204c7d864d7c54bf104bfd173d8630261c)

## [0.1.0-rc.3](https://github.com/konih/kollect/compare/v0.1.0-rc.2..v0.1.0-rc.3) - 2026-06-06

### Bug Fixes

- **collect,controller:** Resolve race detector findings [5100b8b](https://github.com/konih/kollect/commit/5100b8b89eb64a1d2d016d537a28cabd74c6d702)

- **api:** Keep KollectRemoteCluster status optional in codegen [eff3347](https://github.com/konih/kollect/commit/eff3347a33966233db5a4da462b71b675e43cf62)

- **api:** Drop required status on KollectRemoteCluster create [e533d09](https://github.com/konih/kollect/commit/e533d097580a06b7941c442f141358f377e56eb1)

- **ops:** P2 hardening and chart connectionTest default [c3344cc](https://github.com/konih/kollect/commit/c3344cc3a7a8859f5a190541674efb26e737948b)

- **git:** Terminal auth errors and per-repo export lock [4353766](https://github.com/konih/kollect/commit/4353766a4ad74373d82965f441a53acc25112a72)

- **lint:** Gofmt webhooks and phase A envtest cleanup [522754b](https://github.com/konih/kollect/commit/522754bfb14884895fce07cc852652ae013dc65f)

- **sink:** Isolate circuit breaker test from parallel pollution [ff1159a](https://github.com/konih/kollect/commit/ff1159a72fac7d8162b084d3e9211ca94b13c54b)

- **e2e:** Revert multitenant namespaceSelector [926daf0](https://github.com/konih/kollect/commit/926daf0e977e7d491aa060cd5f2e3e3b69c69a7f)

- **controller:** Continue multi-sink export on partial failure [e07add5](https://github.com/konih/kollect/commit/e07add5973538de59b0ec1166ce84cb17057e10a)

- **e2e:** Apply tenant-scope after multitenant asserts [003fe99](https://github.com/konih/kollect/commit/003fe992a2b13ff7078b348351e1e65d3607e826)

- **e2e:** Stabilize multitenant matrix job waits [81df96c](https://github.com/konih/kollect/commit/81df96c354b05868ef1db0ef0f88c454a63bacb6)

- **sink:** Validate git export paths for CodeQL [b67ee61](https://github.com/konih/kollect/commit/b67ee6107897fe04dcdb6f6c3fea8e1108aa9270)

- **ci:** Sync CHANGELOG and UI Docker npm ci [e004476](https://github.com/konih/kollect/commit/e0044763813189d308d68f2a08388dd61e458e0f)

- **e2e:** Bootstrap samples for matrix git-export [084c702](https://github.com/konih/kollect/commit/084c702c6d83d063c40dc111a4f3531f2bdd852f)


### Features

- **controller:** Add cluster rollup finalizers [8dae08f](https://github.com/konih/kollect/commit/8dae08f9dac29e4b77adaa3ece15a6fd410fd2d7)

- **controller:** Add target finalizers [4f377e6](https://github.com/konih/kollect/commit/4f377e6ef6397f543db262789b5d23f94badd156)

- **controller:** Extend inventory finalizer teardown [735abb0](https://github.com/konih/kollect/commit/735abb080b3232321af867ca5d3bd5c6f2d17854)

- **collect:** Add hub cluster store cleanup [756c2c5](https://github.com/konih/kollect/commit/756c2c517a53b7e61f60603fc9b28625a9bc772d)

- **collect:** Add helm: release Secret decode [abf9a75](https://github.com/konih/kollect/commit/abf9a7528de875b2209e5953880735503b3fa6d6)

- **samples:** Add helm-release-values-redacted profile [2cb55d8](https://github.com/konih/kollect/commit/2cb55d81d9e26613e88ed2ef776940b704ebe6a2)

- **collect:** ScrubKeys redaction at extraction [b3ea87e](https://github.com/konih/kollect/commit/b3ea87ee26763c6ad19c7bfc487b8a5ad0eef083)

- **hub:** Ingest auth cache and structured denial logs [58a6cfc](https://github.com/konih/kollect/commit/58a6cfccce4f7f00eb39a6849c900fa2bc4a61f6)

- **controller:** Parallel sinks, debounce metrics, hub coalesce [b907792](https://github.com/konih/kollect/commit/b9077924663087646d132207a909dbccb1c01360)

- **sink:** Backend pool cache and envelope export path [40d77cf](https://github.com/konih/kollect/commit/40d77cf735c23b421a2ece2f1948cdae6e4bd80c)

- **controller:** Add inventory deletion finalizer [4c70371](https://github.com/konih/kollect/commit/4c70371aba2e50d50314342eebcdfd4a5c9431ea)

- **sink:** Add per-sink gobreaker circuit breaker [cae4170](https://github.com/konih/kollect/commit/cae4170780fc6a90cec306188da84affcd36383c)


### Refactoring

- **collect:** Namespace-scoped store watch driver [8a5b8ef](https://github.com/konih/kollect/commit/8a5b8ef722101dded5c79de9303aa8ed957734b7)

- **arch:** Resolve arch-04, arch-11, arch-12 [dae4f79](https://github.com/konih/kollect/commit/dae4f79d589e0d4e0121ef7dc8ac26cedb100081)

- **docs:** Phase 1 root doc moves [6fc22f3](https://github.com/konih/kollect/commit/6fc22f3ae4a830bbec1a100af4b5d4547a37e4f1)

## [0.1.0-rc.2](https://github.com/konih/kollect/compare/v0.1.0-rc.1..v0.1.0-rc.2) - 2026-06-05

### Bug Fixes

- **sink:** Gitlab HTTP client timeout [2c2564d](https://github.com/konih/kollect/commit/2c2564de6dc8ac68d6b444f22a94ea2e4e49df8a)

- **sink:** Postgres connect uses request context [fdcb4d2](https://github.com/konih/kollect/commit/fdcb4d26b14ae811aa17fc685b7657822b2bd4ab)

- **collect:** Degrade target on SAR API error [6d4ed37](https://github.com/konih/kollect/commit/6d4ed37800c78e1507f54e387eac0119b00482d3)

- **hub:** Rollback merge when export fails [b8603e6](https://github.com/konih/kollect/commit/b8603e649b8ca8e6e2fedb03392653d72cd2da82)

- **controller:** Requeue conflicts and log map errors [954d1b6](https://github.com/konih/kollect/commit/954d1b69f9737af1f2933fa08c1d445197bf7761)

- **sink:** Close backends and log close errors [bd022c3](https://github.com/konih/kollect/commit/bd022c3dbe26fbd09602b3022331ddd8dd1effd2)

- **transport:** Commit Kafka offset on handler success [86018a8](https://github.com/konih/kollect/commit/86018a83079cb0b70899481a9c07a872d3e57871)

- **spoke:** Retain delta until publish succeeds [566632b](https://github.com/konih/kollect/commit/566632b2e3e96a8200de5225f6c1542ad96519f6)

- **sink:** Validate git CLI args before exec [50fd4cf](https://github.com/konih/kollect/commit/50fd4cf2ff2d0abdb44eaf61e94d12e960681eb9)

- **demo:** Satisfy OpenSSF Scorecard in kind-wide-scope [75915df](https://github.com/konih/kollect/commit/75915df4c5a1d124aaed114af8aa3787b610066e)

## [0.1.0-rc.1](https://github.com/konih/kollect/compare/v0.0.4..v0.1.0-rc.1) - 2026-06-05

### Bug Fixes

- **git:** Set bare HEAD after file-remote push [3b6bc14](https://github.com/konih/kollect/commit/3b6bc14d9da17a088f6a2fe4d46f1e91f8f90ac2)

- **inventory:** Extract degraded status goconst [521a099](https://github.com/konih/kollect/commit/521a099914341199ced421ba534ff3b358dd0018)

- **sink:** Use git.TypeName for goconst CI lint [0d0fdfe](https://github.com/konih/kollect/commit/0d0fdfe48bac1b61e6c8745b4afb7a95dc656157)

- **demo:** Survive Step 2 bootstrap failures [e729a7b](https://github.com/konih/kollect/commit/e729a7b4ce84c3b40aa2ad08f5cbede18fb60876)

- **demo:** Continue past prerequisite check [84f742e](https://github.com/konih/kollect/commit/84f742e5eebfd7298f21b0b1b06f440e805ff360)

- **chart:** Restrict PrometheusRule alerts to kollect metrics [1c19520](https://github.com/konih/kollect/commit/1c195209c427355ff4409bf50494a1af95bb6093)

- **ui:** Exclude Playwright specs from Vitest runner [9cc1f35](https://github.com/konih/kollect/commit/9cc1f355bb0fe8cb224a4191e570ddd67bcb2167)

- **ui:** Align inventory drawer and badge props with merged API [3580e8a](https://github.com/konih/kollect/commit/3580e8a7d4920e7ed92aa9d1c0d3cecf275bff9d)

- **ui:** Align inventory drawer with merged status APIs [7659378](https://github.com/konih/kollect/commit/76593788b73ef3fde78f52044f22b32e13b0edde)

- **lint:** Extract goconst strings for CI golangci-lint [3ed8703](https://github.com/konih/kollect/commit/3ed87038efb0d509c562d4d02d4309807ee99b48)

- **chart:** Mount writable /tmp for git export [4da294f](https://github.com/konih/kollect/commit/4da294f65f73cfceb9ab3407e5776ef02f5ce612)

- **sink:** Harden git export paths and command args for CodeQL [a336a3b](https://github.com/konih/kollect/commit/a336a3bda77c19ad10b6e96e32f0f96d6fca9d1b)

- **ci:** Add RBAC audit and expand fuzz gates [e6f8b98](https://github.com/konih/kollect/commit/e6f8b983d8b3d05404d1e8151d9510c07cca7d1e)

- **supply-chain:** Address OpenSSF Scorecard findings [2026b16](https://github.com/konih/kollect/commit/2026b165d427cc4b1173b3724660050c17c18f34)

- **docs:** Restore mkdocs nav for reference hub pages [32d3ba7](https://github.com/konih/kollect/commit/32d3ba70a54e0b7388dbcb58ed7bf9c26c3665e8)

- **docs:** Drop mkdocs nav to uncommitted pages [ae6cb01](https://github.com/konih/kollect/commit/ae6cb01ee751f98396c99127568c7b4d7397c50f)

- **e2e:** Recreate unhealthy kind clusters [4dc6f35](https://github.com/konih/kollect/commit/4dc6f35106b93f714e2eb3316dd5651a7aec8370)

- **ci:** Harden workflows for OpenSSF Scorecard [bbd0815](https://github.com/konih/kollect/commit/bbd08154179cf13c6d7edddb44d653874499523e)

- **security:** Harden inventory auth and SAR caches [c934c80](https://github.com/konih/kollect/commit/c934c80f529725f8634b872d89e599df1d582ccb)

- **ci:** Use codecov-action v5 tag instead of bad SHA [f2a1d24](https://github.com/konih/kollect/commit/f2a1d240e5f31bc74ccd3db4725ec2d5b67a5bf9)

- **ci:** Restore 60% coverage floor for test job [fdcc489](https://github.com/konih/kollect/commit/fdcc48906623238f7a758fa23b439140102fbf06)

- **docs:** Repair open questions list rendering [f559c5a](https://github.com/konih/kollect/commit/f559c5a20105e8fc9e1abfe241d95de385a3cfea)

- **ci:** Perf-report envtest gate and changelog [838850c](https://github.com/konih/kollect/commit/838850c2b84787b2b03d11c3d467a39d4b494ef6)

- **ci:** Lll wrap, coverage floor, and changelog [21bfec3](https://github.com/konih/kollect/commit/21bfec3672d096877e90cb6b5501eb954af725aa)

- **ci:** Pin scorecard-action to commit SHA [8911e2f](https://github.com/konih/kollect/commit/8911e2f4d22147edd6db663997dfb6c236ee49d0)

- **docs:** Repair attribute extraction mermaid flowchart [5238169](https://github.com/konih/kollect/commit/52381691002dcbde2c0bf2ee08b5b5740a622da5)

- **docs:** Restore material icon rendering [6504754](https://github.com/konih/kollect/commit/6504754ececf2bc92f2bb28bd41d3fb2a202b78f)

- **ci:** Resolve goconst lint and codecov action pin [67519f4](https://github.com/konih/kollect/commit/67519f4af51183885d7f51d4e3713aa50d725241)

- **ci:** Seed Certificate before team-certificates target [4218369](https://github.com/konih/kollect/commit/421836930c296c09a49127bb6d4e91749052ebbe)

- **ci:** Poll tenant inventory itemCount in multitenant e2e [41e6bf0](https://github.com/konih/kollect/commit/41e6bf0ff3c227b0f235e161fce7a3454ccd638f)

- **ci:** Seed cert-test namespace before Certificate target [e5ea278](https://github.com/konih/kollect/commit/e5ea2780e7da65ce6916eb56fd4a05e8924df3d6)

- **ci:** Skip git export clone without GIT_EXPORT_TEST_REPO [b78d171](https://github.com/konih/kollect/commit/b78d1718945a2fdfe25c6e13ad7589dfa79a3728)

- **ci:** Assert cert collection via Ready message not itemCount [64c826e](https://github.com/konih/kollect/commit/64c826ea69c5ea5cc095df24e6608c7df3d15e0a)

- **ci:** Create cert-test namespace before target registration [121afcd](https://github.com/konih/kollect/commit/121afcd6ae7e5d0bad70f77fd649457b6d4e9099)

- **ci:** Poll cert-manager target itemCount in e2e [1140863](https://github.com/konih/kollect/commit/1140863edf13d536e0fc010203656b0b75d0a8cd)

- **helm:** Grant cert-manager Certificate list/watch for generic CRD e2e [91f4130](https://github.com/konih/kollect/commit/91f4130ba3b3f40fd57993a4407624544d233590)

- **ci:** Poll inventory HTTP for cert-manager e2e [58faff0](https://github.com/konih/kollect/commit/58faff0c1f249ed406068fbaa0eccc23152399c5)

- **ci:** Wait for manager controllers before e2e smoke [a500542](https://github.com/konih/kollect/commit/a500542160450ed8cbaa6cd7cd2f8bfc8d6ab0f7)

- **ci:** Export env before cert-manager e2e subprocess [04f5a45](https://github.com/konih/kollect/commit/04f5a458a1d89d98ef0b56fe8ee37800ff822e69)

- **helm:** Sync manager ClusterRole with kubebuilder RBAC [4ef1a85](https://github.com/konih/kollect/commit/4ef1a855803f159f50ed314906af6eb6b66d3724)

- **ci:** Apply e2e samples directly without kustomize parent refs [08ae477](https://github.com/konih/kollect/commit/08ae477e0787d3d0ef949475bd4a9e5cd2ca63d0)

- **ci:** Use lean e2e samples without unreachable sinks [6f0f184](https://github.com/konih/kollect/commit/6f0f1843a57936f3dd6f33e25d7ae953ace34fdd)

- **api:** Drop required status on KollectConnectionTest create [bbe1c82](https://github.com/konih/kollect/commit/bbe1c82d37105c62172059acb008b242ec1d452b)

- **ci:** Repair Go 1.26 internal coverage profile merge [c897773](https://github.com/konih/kollect/commit/c8977735bca2efd38c5afdb962e698a216b41381)

- **ci:** Skip cmd packages in coverage pre-pass [b432fbe](https://github.com/konih/kollect/commit/b432fbe399f0f943a6f4fdafef54be4747f76ee7)

- **controller:** Gate validating webhooks when chart disables TLS [06eef97](https://github.com/konih/kollect/commit/06eef970672722795338529d8acc5cf3066299f0)

- **ci:** Merge internal coverage profile without -p 1 [db8d50e](https://github.com/konih/kollect/commit/db8d50e81901ff30e11691c43cd09be1b1f48200)

- **ci:** Stabilize coverage profile merge and cmd skip [758cb29](https://github.com/konih/kollect/commit/758cb298d96e9dbd426df823726c9635818f920b)

- **ci:** Raise kind e2e helm install wait timeouts [61c61f2](https://github.com/konih/kollect/commit/61c61f25735fe7ed3b2f8e2b35c7b96452c2e429)

- **controller:** Stabilize cluster target and inventory envtests [4ce24c3](https://github.com/konih/kollect/commit/4ce24c3b1fa337db75f1a8d74bc091567ca3908d)

- **sink/git:** Simplify file remote push and clone tests [ce84b2e](https://github.com/konih/kollect/commit/ce84b2ecce27f647c6fbaa83fcd9d991735f2aea)

- **ci:** Pin git export tests to main branch [7d6dcf9](https://github.com/konih/kollect/commit/7d6dcf9ea93f1c2c3e8f34e565b799f694bdcea8)

- **sink/git:** Reset workdir after failed file clone [359093a](https://github.com/konih/kollect/commit/359093a3760f3d354ff8c2b3bc72477a0f21bf58)

- **sink/git:** Use native git for file:// export path [af9dd48](https://github.com/konih/kollect/commit/af9dd480ea9c7390c3848a2b30afe83a690c1ca3)

- **sink/git:** Write export payload via worktree FS [57d97b0](https://github.com/konih/kollect/commit/57d97b078c8a76f2bb855ae26ae684d49e830636)

- **release:** Use v0.0.4 MVP anchor, reserve v0.1.0 for publish [47706eb](https://github.com/konih/kollect/commit/47706eb217c1fd4e6002b23d746213a4a52692f5)

- **sink/git:** Native push for file remotes on CI [d25d83a](https://github.com/konih/kollect/commit/d25d83a7c209115edf52590956dc80bc35c6377c)

- **sink/git:** Force-push when bare clone has no HEAD [cc1473b](https://github.com/konih/kollect/commit/cc1473bd4ff428b62762fd56ac7257dbd5f4239b)

- Address code review P0/P1 findings [cd1642f](https://github.com/konih/kollect/commit/cd1642f0662517a2ef5fffd55ddb2b16ed5fe178)

- **validation:** Require ClusterTarget namespaceSelector [2363386](https://github.com/konih/kollect/commit/2363386f217fe96d8887c7f2c4459a6f7678538e)


### Features

- **api:** Per-sink export interval scheduling [296577b](https://github.com/konih/kollect/commit/296577bc0f2c682ea54d9cd3aea61275f93cf335)

- **demo:** Venue pitch personas, fast churn, UI reveal [29f8046](https://github.com/konih/kollect/commit/29f80464d70a206ebb48baefbd197e464178edf5)

- **chart:** Add Prometheus Operator monitoring [cfbe62f](https://github.com/konih/kollect/commit/cfbe62f6aba27de52ac01b38fcff899dd1ddfa3c)

- **api:** Default KollectSink connectionTest to true [495068e](https://github.com/konih/kollect/commit/495068e7706df3223f3c775809fca21b3dbd997f)

- **ui:** Merge inventory MVP with filters and virtualization [3971d8e](https://github.com/konih/kollect/commit/3971d8e3790f56e8aa868271b75b8c0fdee6a434)

- **demo:** Refactor wide-scope kind showcase [b1b3d91](https://github.com/konih/kollect/commit/b1b3d9177527f593c2abeef7b21ec9f83e31dbcb)

- **ui:** Wire inventory rows to detail drawer [a1d7fd7](https://github.com/konih/kollect/commit/a1d7fd7fbf0bea1f26217edd41be848715c1a8e2)

- **ui:** Add inventory MVP with filters and virtualization [9f5e04b](https://github.com/konih/kollect/commit/9f5e04b731703551d5bd28efcc5c42cd9f095404)

- **ui:** Merge overview degraded strip and export stats [c5ca56e](https://github.com/konih/kollect/commit/c5ca56e59366f9fcbd74d9405f2d9b2b45a89051)

- **ui:** Merge detail drawers for targets and sinks [5690d4d](https://github.com/konih/kollect/commit/5690d4dfecff2da35bfefeecbb415fad1c9a2bc1)

- **ui:** Add detail drawers for targets and sinks [8f3ebe3](https://github.com/konih/kollect/commit/8f3ebe3b47cbcd4e82638cbd4784b0c77aee05e0)

- **sink:** Add git push policy branch and auth options [f175929](https://github.com/konih/kollect/commit/f175929280f18d2e3a2ba0363005e18610b1e25e)

- **ui:** Implement Phase 1 mock Read API (MSW + Prism) [a2d9308](https://github.com/konih/kollect/commit/a2d9308b49a717ee2a2bd19454eaacf54567f693)

- **inventory:** Extend Read API for UI contract [0e56c19](https://github.com/konih/kollect/commit/0e56c196cf90b1cff90d113cca5074b7f3ccc0c6)

- **api:** Add Target collection filtering (ADR-0207) [34b1ebf](https://github.com/konih/kollect/commit/34b1ebf02500491f83a2727d5671a5518e089b02)

- **export:** Enforce object-store spill above 1 MiB [88e9158](https://github.com/konih/kollect/commit/88e91588486e66b0393f60aa070d637371eac743)

- **api:** Add KollectSink spec.pathTemplate [cb45ae6](https://github.com/konih/kollect/commit/cb45ae654fff5d71fecd6dda719d7392acb2c30a)

- **sink:** Add S3/GCS Parquet snapshot export [f66d3be](https://github.com/konih/kollect/commit/f66d3bee667b177c2445132bb7b7655b00635a22)

- [**breaking**] Remove KollectHub reconciler [9190ee1](https://github.com/konih/kollect/commit/9190ee16026af163083359c8f0e3ce7364689e96)

- [**breaking**] Remove KollectHub CRD surface [d4fd4f0](https://github.com/konih/kollect/commit/d4fd4f068500ae02b5839d6e719d9e0166bfb81f)

- **webhook:** Validate KollectSink spec.type enum [7fb0475](https://github.com/konih/kollect/commit/7fb0475775908d36a343b55f47994a1fae092369)

- **api:** Add KollectClusterInventory spec.dedupe [10d9345](https://github.com/konih/kollect/commit/10d934501e31b008cd9d474a0924886aa9c632d0)

- **export:** Add schemaVersion inventory envelope [eae349e](https://github.com/konih/kollect/commit/eae349ecd93ed44f1414a15f893a0ac0c4f1a4ab)

- **sink:** Add NATS JetStream event sink backend [9ea0fcf](https://github.com/konih/kollect/commit/9ea0fcfb911b39a396f8bb296acb2f97c6774723)

- **export:** Add schemaVersion to export envelopes [52ff2ac](https://github.com/konih/kollect/commit/52ff2acda30f5f5adede43ef10f8ae16feb72926)

- **sink:** Add Capabilities and postgres delete reconciliation [4f26f1c](https://github.com/konih/kollect/commit/4f26f1cc96f702a388f30755e0d03d1ed973803a)

- **controller:** Wire cluster inventory aggregate dedupe [e5ac7b5](https://github.com/konih/kollect/commit/e5ac7b5361e2369cae51f06519a14c047273531f)

- **aggregate:** Add cross-target dedupe spike stub [2327b8b](https://github.com/konih/kollect/commit/2327b8bb4e36f0a5a9334e383ce1c9a43333bb7e)

- **sink:** Push gitlab exports to feature branch in mr mode [08843d1](https://github.com/konih/kollect/commit/08843d19a74aa129afdd9b75d2994f090e5f05fc)

- **collect:** Emit prometheus label values from profile metrics [a3c72ec](https://github.com/konih/kollect/commit/a3c72ecdcc88fbf9784fe9882b684534152628b9)

- **collect:** Wire profile metrics paths and hub merge metric [4e7d01d](https://github.com/konih/kollect/commit/4e7d01dcd8dc4c7a81fe573c12d748f19a984cbf)

- **api:** Add KollectProfile.spec.metrics spike [9874d02](https://github.com/konih/kollect/commit/9874d025ef139372678dc93b3e3133b31cf2725b)

- **collect:** Wire RecordCustomResourceSeries on snapshot [1829c85](https://github.com/konih/kollect/commit/1829c858b687c588210e8e44b13f13a846a8733e)

- **collect:** Add Phase 4 aggregation metrics stub [d05fc4c](https://github.com/konih/kollect/commit/d05fc4c5b6a3288c55901734198245b55d50e6fb)

- **sink:** Add GitLab API v4 merge request client [8247f4e](https://github.com/konih/kollect/commit/8247f4ec6fa451b7837acce56a0f534193a13352)

- **collect:** Complete ADR-0020 metrics catalog [4d14925](https://github.com/konih/kollect/commit/4d14925b5aebc8665c59c72181e9950cc07ad011)

- **sink:** Add gitlab mergeRequest CRD and transport ACL wire [bd6499f](https://github.com/konih/kollect/commit/bd6499fe1756dc5c7cc24367d49fa46f96f079ca)

- **controller:** Wire cluster inventory export to sinks [b5445a2](https://github.com/konih/kollect/commit/b5445a2c37ea27f91a13b8ba6084cdf201ff4796)

- **transport:** Add queue wire ACL allowlist stub [a4c73a8](https://github.com/konih/kollect/commit/a4c73a8045772781eb845d083f11760b01ff18c7)

- **controller:** Add cluster target and inventory skeletons [737786d](https://github.com/konih/kollect/commit/737786d26ad4551b4d6b7588a34e61b2ce4eed4a)

- **sink/gitlab:** Scaffold GitLab export backend [553117c](https://github.com/konih/kollect/commit/553117cc30b5fdaaf8bcdf42406287ac403d9d81)

- **hub:** Parallel Postgres+Kafka export on ingest [68c832a](https://github.com/konih/kollect/commit/68c832a4333cacc02dff396a421df710669c6d52)

- **api:** Add KollectClusterProfile CR [c901190](https://github.com/konih/kollect/commit/c9011907df03ae50c01e78e2a0c8f7ec0849091a)

- **api:** Add KollectClusterInventory CR [47d1647](https://github.com/konih/kollect/commit/47d1647637b550984337e166d01cc34dca06cf4e)

- **api:** Add KollectClusterInventory CR [1263877](https://github.com/konih/kollect/commit/12638770dbf0a24f15ec420bc35e1b39fec293ab)

- **api:** Add KollectClusterTarget CR and webhook [4ed55f2](https://github.com/konih/kollect/commit/4ed55f2d31778465ec844ea60f61b3915a01eea3)


### Refactoring

- **controller:** Remove --export-debounce flag [d5a01fa](https://github.com/konih/kollect/commit/d5a01fa5cf631431df16536de9bc34b3b563b552)

- Unify bearer auth and brittle error assertions [d382a28](https://github.com/konih/kollect/commit/d382a28ff5e9188e1547f2f176857bb0c8752047)

- **sink:** Extract shared RunExportItems pipeline [d9494ae](https://github.com/konih/kollect/commit/d9494ae79a68065a9122f8cf538a13f877045930)

- **controller:** Extract cluster inventory export path [5214893](https://github.com/konih/kollect/commit/5214893dda65d7a5957b010b16d6a253140e9f8d)

## [0.0.4](https://github.com/konih/kollect/compare/v0.0.3..v0.0.4) - 2026-06-05

### Bug Fixes

- **inventory:** V1alpha1 HTTP paths and export caps [aa9f9c6](https://github.com/konih/kollect/commit/aa9f9c6d991aadda47a61e764bf4525f83ff84b3)

- **collect:** Avoid startInformer mutex deadlock [50fb789](https://github.com/konih/kollect/commit/50fb7893316d71fa6275c74270919a7e7bb9b5a4)

- **obs:** Standalone perf-report shell script [f51e0be](https://github.com/konih/kollect/commit/f51e0be5df4fcdf3d4f05ebca771316a8964e877)


### Features

- **api:** Add KollectConnectionTest CR [176a945](https://github.com/konih/kollect/commit/176a945ac03b093ac35fb8d394b5b423b73e82e7)

- **inventory:** Export debouncing with checksum [23c575d](https://github.com/konih/kollect/commit/23c575d58e83b84e998c7c5f668bd142fcfebaa0)

- **api:** [**breaking**] Namespaced KollectSink and same-ns sinkRefs [9c0e361](https://github.com/konih/kollect/commit/9c0e3619c88c314137f9f27f493ecf2cf8484779)

- **transport:** Queue TLS and hub ACL hardening [bf0139a](https://github.com/konih/kollect/commit/bf0139a0b70b74b88bb3bacdbc440e2ef6fbe0f9)

- **controller:** SinkReachable export and probe cleanup [4940cd0](https://github.com/konih/kollect/commit/4940cd08f285b64ae2e3994f726cc73efcb6af88)

- **operator:** Add hub and spoke mode flag [c8da5b3](https://github.com/konih/kollect/commit/c8da5b3243bf628a6ff9e7f7b888b29aaff75105)

- **api:** [**breaking**] Make KollectProfile namespaced [92240c3](https://github.com/konih/kollect/commit/92240c330f8122837230f8c5ec66d363cb984d6b)

- **scope:** Enforce KollectScope in reconcilers [2e74b41](https://github.com/konih/kollect/commit/2e74b415d9b24c71efa5bb5d83eaa9c0041e6643)

- **hub:** Wire remoteClusters and credential pull [bd919cd](https://github.com/konih/kollect/commit/bd919cd9e4de2d8ae01a8986390d5dc48d998f15)

- **cli:** Add create-remote-secret stub [df19fd6](https://github.com/konih/kollect/commit/df19fd6569d16733fe46ec061a977874b99f95e3)

- **hub:** Add SAR on ingest auth [d0f06b3](https://github.com/konih/kollect/commit/d0f06b3bab5417188d41f093a5cf016671aac252)

- **hub:** Remote cluster Connected and queue wire auth [32ef269](https://github.com/konih/kollect/commit/32ef269c6747041b4bcfc03635d281b7f2e0c87e)

- **collect:** Namespace and resource watch labels [d1c059a](https://github.com/konih/kollect/commit/d1c059a83066116140089f01074ba9dcf01afec7)

- **hub:** Spoke push auth via TokenReview [080435f](https://github.com/konih/kollect/commit/080435f56fd0a7d6e1d168e0a68ac00d6fa19e2c)

- Namespaced multi-tenant operator support [f19cc4e](https://github.com/konih/kollect/commit/f19cc4ec437c35daabc640579bfa6832247a82da)

- **spoke:** Delta publish with transport reuse [2e4b217](https://github.com/konih/kollect/commit/2e4b2171ac50826d440676ccc9545a23fdbe9dca)

- **hub:** 100-spoke merge spike and delta removals [cdc79b2](https://github.com/konih/kollect/commit/cdc79b246b541dcbcf15c26afc0e6008a0e47123)

- **transport:** Add NATS JetStream backend [9f185e4](https://github.com/konih/kollect/commit/9f185e4c3c74c912dcb751a3cecddf71774ea30d)

- **hub:** Wire consumer mode and spoke publish stub [b16321a](https://github.com/konih/kollect/commit/b16321ac7d597a6270a09e7043fcd95292e585a0)

- **hub:** Spoke report merge consumer [b06659e](https://github.com/konih/kollect/commit/b06659e2339d2442e52db341948adb7d96026322)

- **transport:** Kafka backend with redpanda tests [b795ceb](https://github.com/konih/kollect/commit/b795cebe9d35f446c8c6c41b86ccd650a67cd582)

- **obs:** Task perf-report and metrics catalog [e5ce1a9](https://github.com/konih/kollect/commit/e5ce1a9b092f6d7a74e188d2be3334c285ba0215)

- **perf:** Parallelism, metrics, and pprof [53efb00](https://github.com/konih/kollect/commit/53efb00c0c9945048c12e795cbc77c6b03e00c9d)

- **sink:** Postgres and kafka export backends [5eb2b71](https://github.com/konih/kollect/commit/5eb2b7101a45746487e47975a97aa8ef24e5b411)

- **sink:** GCS backend and prometheus stub [0d6ab00](https://github.com/konih/kollect/commit/0d6ab0024c5ced84999463fc47df50ef6db4398d)

- **api:** KollectHub and KollectScope CRDs [c61ef78](https://github.com/konih/kollect/commit/c61ef781052c23febd8589de5bd94de7a334801f)

- **transport:** Pluggable factory with Redis Streams [36a8193](https://github.com/konih/kollect/commit/36a819337cba8e799cb6d928563fae1035f35cf1)

- **metrics:** Complete ADR-0020 operator metric set [ec56d86](https://github.com/konih/kollect/commit/ec56d862a5b91942b93805705b2c5cc762865433)

- **collect:** SAR degradation and namespaceSelector [e127736](https://github.com/konih/kollect/commit/e1277363aba2a389448ba941af7508ede3550fdc)

- **inventory:** Store-backed HTTP API and K8s auth [3c84ec9](https://github.com/konih/kollect/commit/3c84ec957cb53a74b5f8cbf70a96c9016a963553)

- **controller:** Wire collection and inventory export [dd31026](https://github.com/konih/kollect/commit/dd310269acdf34e0d2dc9005e9f49d2f5eb463d5)

- **transport:** Add in-process pub/sub bus [33344c2](https://github.com/konih/kollect/commit/33344c26729ffdee6d9da3b76617203bc636c425)

- **collect:** Add dynamic informer engine and store [db23db0](https://github.com/konih/kollect/commit/db23db001e7bbb4bb5413d492d5726b84f2ec1e6)

- **sink:** Add git export and s3 PutObject backends [4b5f7a8](https://github.com/konih/kollect/commit/4b5f7a8a69aeb8fe29783ac4191259ec65617b01)

- **inventory:** Add toggleable HTTP endpoint and metrics [3f5c194](https://github.com/konih/kollect/commit/3f5c1941dd888021810ec4c4f7c4b5ff9314b624)

- **sink:** Add connection test with TLS CA support [a907332](https://github.com/konih/kollect/commit/a907332ff869fb65139e60550c2bb4fd7af78a54)

- **webhook:** Validate Profile CEL and JSONPath paths [ab044e5](https://github.com/konih/kollect/commit/ab044e5cb41717c459fb84a86510c85a553608ab)


### Refactoring

- **hub:** Deprecate KollectHub controller [a0789ba](https://github.com/konih/kollect/commit/a0789ba2fcb3b4f2e945a4e056f3968a97bb1ded)

- **collect:** Store Len and reconcile metrics [b30dd3d](https://github.com/konih/kollect/commit/b30dd3de07d6d3ab61a7fcdaeef64afe63ee9dea)

- **api:** [**breaking**] Make KollectInventory namespaced [cdd06d2](https://github.com/konih/kollect/commit/cdd06d265ac37fef3b0750999ffcf5822c5598f0)

## [0.0.3](https://github.com/konih/kollect/compare/v0.0.2..v0.0.3) - 2026-06-05

### Bug Fixes

- **build:** Use repo-root kustomize in deploy task [ab0f434](https://github.com/konih/kollect/commit/ab0f434336684a5d43e06129521d9ea28d3a7a79)

- **build:** Move scrub patterns out of Taskfile [8a435c4](https://github.com/konih/kollect/commit/8a435c43ff5c2e674e6c93eae359caa29386df9c)

- **test:** Satisfy ginkgolinter and shorten status comments [15eef6d](https://github.com/konih/kollect/commit/15eef6df77ec514caa4ca727b91b5f6b9bc5d006)


### Features

- **helm:** Add kollect operator chart [6642133](https://github.com/konih/kollect/commit/66421337fb48ecadbae4856d51e7dc2433470eee)

- **api:** Add sink TLS and inventory HTTP fields [9fc70ee](https://github.com/konih/kollect/commit/9fc70ee230d3c6a05a095bf24c7622653b9393f1)

- **controller:** Validate KollectTarget profileRef [1fccc36](https://github.com/konih/kollect/commit/1fccc3657e5bcbc71938eeb27a2e4017d3a7874f)

- **sink:** Add backend registry with git stub [d7f0e1c](https://github.com/konih/kollect/commit/d7f0e1c4aeb7eeb58b974de689efe2020db38bfe)

- **collect:** Add CEL and JSONPath extractor [91ab137](https://github.com/konih/kollect/commit/91ab1379f9a7357baef656d53afd044964223f73)


### Refactoring

- **samples:** Consolidate on kollect_v1alpha1_* set [237b805](https://github.com/konih/kollect/commit/237b8051278150ad58d45ff9233b45bbb7e12d90)

## [0.0.2](https://github.com/konih/kollect/compare/v0.0.1..v0.0.2) - 2026-06-04

### Features

- **api:** Add KollectProfile/Sink/Target/Inventory v1alpha1 types [1e6f671](https://github.com/konih/kollect/commit/1e6f6719bcab81d3c18eb17d066bb29946a9f70e)

## [0.0.1] - 2026-06-04
