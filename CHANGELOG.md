# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Release notes are generated from [Conventional Commits](https://www.conventionalcommits.org/)
on the default branch using [git-cliff](https://git-cliff.org/).

## Version mapping (retroactive anchors)

Retroactive git tags `v0.0.1`–`v0.0.4` segment history before the first published GitHub
release. See [docs/RELEASE.md](docs/RELEASE.md) for the maintainer runbook.

| Tag | Anchor commit | Milestone |
| --- | --- | --- |
| `v0.0.1` | `13546aff` | Kubebuilder scaffold, MIT license |
| `v0.0.2` | `1e6f6719` | Core `v1alpha1` CRDs (Profile, Sink, Target, Inventory) |
| `v0.0.3` | `66421337` | Helm chart, CEL/JSONPath extraction, inventory HTTP |
| `v0.0.4` | `4234960b` | ADR-0032 platform pivot MVP (namespaced API, export pipeline) |
| `v0.1.0` | `main` HEAD | First published release (includes hub/cluster APIs since MVP) |

Reserve **`v0.1.0` for the first tag pushed at `main` HEAD** — do not use it as a changelog-only
anchor on an older commit.

## [Unreleased]

### Bug Fixes

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

- **transport:** Add queue wire ACL allowlist stub [a4c73a8](https://github.com/konih/kollect/commit/a4c73a8045772781eb845d083f11760b01ff18c7)

- **controller:** Add cluster target and inventory skeletons [737786d](https://github.com/konih/kollect/commit/737786d26ad4551b4d6b7588a34e61b2ce4eed4a)

- **sink/gitlab:** Scaffold GitLab export backend [553117c](https://github.com/konih/kollect/commit/553117cc30b5fdaaf8bcdf42406287ac403d9d81)

- **hub:** Parallel Postgres+Kafka export on ingest [68c832a](https://github.com/konih/kollect/commit/68c832a4333cacc02dff396a421df710669c6d52)

- **api:** Add KollectClusterProfile CR [c901190](https://github.com/konih/kollect/commit/c9011907df03ae50c01e78e2a0c8f7ec0849091a)

- **api:** Add KollectClusterInventory CR [47d1647](https://github.com/konih/kollect/commit/47d1647637b550984337e166d01cc34dca06cf4e)

- **api:** Add KollectClusterInventory CR [1263877](https://github.com/konih/kollect/commit/12638770dbf0a24f15ec420bc35e1b39fec293ab)

- **api:** Add KollectClusterTarget CR and webhook [4ed55f2](https://github.com/konih/kollect/commit/4ed55f2d31778465ec844ea60f61b3915a01eea3)

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
