# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Release notes are generated from [Conventional Commits](https://www.conventionalcommits.org/)
on the default branch using [git-cliff](https://git-cliff.org/).

## [Unreleased]

### Bug Fixes

- **security:** Verify CI scanner archive checksums ([#115](https://github.com/platformrelay/kollect/pull/115))[491e6e1](https://github.com/platformrelay/kollect/commit/491e6e120941392cb375f5d04451a0405db8607a)

- **collect:** Drop stale row on extract failure ([#113](https://github.com/platformrelay/kollect/pull/113))[705fe31](https://github.com/platformrelay/kollect/commit/705fe31f15ef606e015811cb4c15d63732f8c20a)

- **netguard:** Allow localhost under integration allowPrivate ([#111](https://github.com/platformrelay/kollect/pull/111))[ed6326c](https://github.com/platformrelay/kollect/commit/ed6326cae865d142b6dfd6e6992b8233963bfde1)

- **security:** Close BigQuery and git SSRF gaps [d648824](https://github.com/platformrelay/kollect/commit/d64882483df9b0507041366c6462062f693512af)

- **security:** Guard resolved sink addresses [747aff8](https://github.com/platformrelay/kollect/commit/747aff85b2b2fa2a753a3575fc1b53b0d46d1f24)

- **pipeline:** Degrade exit on extraction failures [ca7fa80](https://github.com/platformrelay/kollect/commit/ca7fa80d5a8b2c51e4767249a2997ea7e0450c30)

- **collect:** Surface per-object extraction failures [d3123e9](https://github.com/platformrelay/kollect/commit/d3123e94945b113b88be7c631bf850914532f9f8)

- **collect:** Harden informer replacement [f253e39](https://github.com/platformrelay/kollect/commit/f253e39187f4f0742a7d1e7a14066e81bc1eda51)

- **collect:** Widen shared informer scope [60f0ade](https://github.com/platformrelay/kollect/commit/60f0adebbe468b24b07d40499d3ba7aef44eb3f2)

- **ci:** Correct Renovate scheduling [96b370b](https://github.com/platformrelay/kollect/commit/96b370b9448bc9e4020b3c66b9943e7a36396372)

- **docs:** Describe CI filters exactly [e449712](https://github.com/platformrelay/kollect/commit/e4497122881726c71308e8eb675bea908e251019)


### Refactoring

- **operator:** Simplify mode normalization [c03042e](https://github.com/platformrelay/kollect/commit/c03042e13834f92d58b1ee2518f3790eb213f1da)

