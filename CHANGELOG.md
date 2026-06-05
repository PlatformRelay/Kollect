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
| `v0.0.4` | `4234960b` | ADR-0703 platform pivot MVP (namespaced API, export pipeline) |
| `v0.1.0` | `main` HEAD | First published release (includes hub/cluster APIs since MVP) |

Reserve **`v0.1.0` for the first tag pushed at `main` HEAD** — do not use it as a changelog-only
anchor on an older commit.

## [Unreleased]

### Bug Fixes

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
