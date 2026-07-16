# Changelog

All notable changes to themis are documented here.

## [0.0.1] - 2026-07-16

### Bug Fixes
- Fail fast on non-root instead of after the audit runs ([`f727163`](https://codeberg.org/Elysium_Labs/themis/commit/f7271639fc741d819b5e1bf13fddaf830aced6f8))
- Scope sshd bans to port, warn on WireGuard/CrowdSec conflicts ([`63eaa9f`](https://codeberg.org/Elysium_Labs/themis/commit/63eaa9ff89863ba1516699fd289600f836a62101))
- Skip merge commits and changelog-bump commits from changelog ([`2cbbf08`](https://codeberg.org/Elysium_Labs/themis/commit/2cbbf0831be2f700e5d48f04e4cf00d0466b9124))
- Use full GitHub URL for osv-scanner-action, not mirrored on Forgejo ([`08451c5`](https://codeberg.org/Elysium_Labs/themis/commit/08451c5164ebe7351b1efcea55b69bddaa590374))
- Allow GOTOOLCHAIN auto-upgrade for osv-scanner install ([`90c8b8c`](https://codeberg.org/Elysium_Labs/themis/commit/90c8b8cf3ce76ebbe7f8f19fe790cb52c84e6e7a))
- Atomic binary swap in install.sh ([`5cd07a2`](https://codeberg.org/Elysium_Labs/themis/commit/5cd07a2bab32db13e0252a31613041d9477c5322))


### CI/CD
- Add OSV scanner workflow for PRs to main ([`0c918d3`](https://codeberg.org/Elysium_Labs/themis/commit/0c918d3a7cd8f01b8d756732566c3fb8b6c33171))
- Run osv-scanner CLI directly instead of GitHub Action wrapper ([`7f5850e`](https://codeberg.org/Elysium_Labs/themis/commit/7f5850e9ac6f2215390ce8352607dc2d9db5c606))


### Features
- Add interactive shell completion, ported from eos ([`c21370e`](https://codeberg.org/Elysium_Labs/themis/commit/c21370e4b56d0b66059d4faf0a482a0e337a9374))
- Introduce Source interface, decouple check/api from Lynis ([`bf9c9be`](https://codeberg.org/Elysium_Labs/themis/commit/bf9c9be07b2787d3d3075056118140d3ea6367c7))
- Add --quick flag and nice/ionice priority wrapping ([`5471776`](https://codeberg.org/Elysium_Labs/themis/commit/547177661fd921f954931a289735a3fba32a2064))
- Add Apache 2.0 license, matching theia and eos ([`0f90838`](https://codeberg.org/Elysium_Labs/themis/commit/0f9083883cf9091db05b0857e9fd0107951d43a7))
- Add themis-native audit source (closes #15) ([`6a1d5cb`](https://codeberg.org/Elysium_Labs/themis/commit/6a1d5cb2b06f978cf3da6fda1800566387d95aa9))
- Add top-level --version/-v flag ([`1c1ad37`](https://codeberg.org/Elysium_Labs/themis/commit/1c1ad37b10a0fd6ceffd7ec58b5f2c3ebc436a88))


### Testing
- Cover completion.go ([`f1b9e15`](https://codeberg.org/Elysium_Labs/themis/commit/f1b9e15224ac3b09a3fe776329cf8b42a01e8ee9))

## [0.0.1-rc.7] - 2026-07-14

### Features
- Group version/update/uninstall under `themis system` ([`f669918`](https://codeberg.org/Elysium_Labs/themis/commit/f669918999da75eff419d401395d11803c05c021))

## [0.0.1-rc.6] - 2026-07-14

### Miscellaneous
- Use ui package styling in apply/rollback output ([`90736cd`](https://codeberg.org/Elysium_Labs/themis/commit/90736cd720482630b211b8147b64f2ab31451365))

## [0.0.1-rc.5] - 2026-07-14

### Bug Fixes
- Drop Lynis from themis's top-level CLI description ([`9b2175c`](https://codeberg.org/Elysium_Labs/themis/commit/9b2175c7d98b5e18446e7b290f3499c87f3ed029))
- Generate real changelog notes for releases ([`f4130b2`](https://codeberg.org/Elysium_Labs/themis/commit/f4130b213464028f72cb4ddab8836e2db8ad7106))
- Resolve lynis outside PATH, add blank line after CLI errors ([`490d0bd`](https://codeberg.org/Elysium_Labs/themis/commit/490d0bd02a71cbe5c37f6854181ce9f65c19984c))
- Detect host arch for git-cliff install ([`880db07`](https://codeberg.org/Elysium_Labs/themis/commit/880db07e629f69598b2d035a771ed71a1a5b4abe))


### CI/CD
- Add issue/PR templates, mirroring eos ([`cbd5ad7`](https://codeberg.org/Elysium_Labs/themis/commit/cbd5ad7688fa7e9fb268c4dbf916484efa765de2))


### Features
- Show version in default help output ([`b73cfa6`](https://codeberg.org/Elysium_Labs/themis/commit/b73cfa69373003ae3cd576de9a1f226223ac4c14))


### Improvements
- Update README.md ([`c2bff89`](https://codeberg.org/Elysium_Labs/themis/commit/c2bff89210c116fa62e7936f565ebd28a386a757))

## [0.0.1-rc.4] - 2026-07-14

### Features
- Add --pre flag to themis update, inline release logic into cmd ([`22b30e9`](https://codeberg.org/Elysium_Labs/themis/commit/22b30e9d707d8f6bc666459948ff067074b84fae))

## [0.0.1-rc.3] - 2026-07-14

### Features
- Guard SSH password-auth fix against lockout, fix nilaway finding ([`4475c63`](https://codeberg.org/Elysium_Labs/themis/commit/4475c6307f47ed9cdb6952f44db17350b46ccab7))

## [0.0.1-rc.2] - 2026-07-14

### Bug Fixes
- Satisfy golangci-lint (errcheck, gosec, govet shadow, misspell) ([`eccadef`](https://codeberg.org/Elysium_Labs/themis/commit/eccadefc79ff62c70f2ef31aadf41e375f91d5de))


### Features
- Human-readable errors and audit spinner ([`85c7224`](https://codeberg.org/Elysium_Labs/themis/commit/85c7224c1eadebdc41c30939060ed9f5d1a3ad4e))
- Add themis update and uninstall commands ([`bf3ac60`](https://codeberg.org/Elysium_Labs/themis/commit/bf3ac602750795c70298d8aa5843b70f31242349))


### Maintenance
- Add lefthook config, mirroring eos ([`ce84a14`](https://codeberg.org/Elysium_Labs/themis/commit/ce84a1416c5648fef6f33e452b560fff5559b44d))
- Wire up fieldalignment tool, enable it in lefthook ([`e210702`](https://codeberg.org/Elysium_Labs/themis/commit/e2107028ba02ecb5684a54cfeaa243fe927f5ccf))

## [0.0.1-rc.1] - 2026-07-13

### Features
- Add cobra CLI POC (check/plan/apply/rollback) ([`da90ed9`](https://codeberg.org/Elysium_Labs/themis/commit/da90ed9280ec98400e79b2f67ddc4fec35f2cb73))
- Merge Lynis findings with themis fixes, styled table, api check ([`5252724`](https://codeberg.org/Elysium_Labs/themis/commit/5252724df4576a9e9e05fa83009d4b980f306441))
- Replace findings table with block list, never fully hide findings ([`8144323`](https://codeberg.org/Elysium_Labs/themis/commit/81443238a66ddf1a736fb1df58f2c064feaac0e9))
- Add release infra — buildinfo, install.sh, README, release workflow ([`6a0f426`](https://codeberg.org/Elysium_Labs/themis/commit/6a0f42628808b400eeff7fd0763994c6a894e3ab))


### Miscellaneous
- Initial commit ([`873ce3d`](https://codeberg.org/Elysium_Labs/themis/commit/873ce3dc288f6fa30ca5c2ec001af3f5ea03db4b))

