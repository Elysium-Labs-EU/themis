# Changelog

All notable changes to themis are documented here.

## [0.0.1] - 2026-07-15

### Bug Fixes
- Fail fast on non-root instead of after the audit runs ([`f727163`](https://codeberg.org/Elysium_Labs/themis/commit/f7271639fc741d819b5e1bf13fddaf830aced6f8))
- Scope sshd bans to port, warn on WireGuard/CrowdSec conflicts ([`63eaa9f`](https://codeberg.org/Elysium_Labs/themis/commit/63eaa9ff89863ba1516699fd289600f836a62101))


### Features
- Add interactive shell completion, ported from eos ([`c21370e`](https://codeberg.org/Elysium_Labs/themis/commit/c21370e4b56d0b66059d4faf0a482a0e337a9374))
- Introduce Source interface, decouple check/api from Lynis ([`bf9c9be`](https://codeberg.org/Elysium_Labs/themis/commit/bf9c9be07b2787d3d3075056118140d3ea6367c7))
- Add --quick flag and nice/ionice priority wrapping ([`5471776`](https://codeberg.org/Elysium_Labs/themis/commit/547177661fd921f954931a289735a3fba32a2064))
- Add Apache 2.0 license, matching theia and eos ([`0f90838`](https://codeberg.org/Elysium_Labs/themis/commit/0f9083883cf9091db05b0857e9fd0107951d43a7))


### Maintenance
- Update changelog for v0.0.1 ([`68cbe20`](https://codeberg.org/Elysium_Labs/themis/commit/68cbe203791a5a32deb57283842cb66f9a6192b5))


### Miscellaneous
- Merge pull request 'feat(cmd): group version/update/uninstall under `themis system`' (#12) from feat/system-command-group into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/12 ([`ddfd48a`](https://codeberg.org/Elysium_Labs/themis/commit/ddfd48a44db512b2868e9dc479cb2072f1363476))
- Merge pull request 'feat/completion-command' (#13) from feat/completion-command into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/13 ([`01891be`](https://codeberg.org/Elysium_Labs/themis/commit/01891bef3d7b0936f46940e120c63d3120202dc7))
- Merge pull request 'feat(audit): introduce Source interface, decouple check/api from Lynis' (#17) from feat/source-interface into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/17 ([`0173c8e`](https://codeberg.org/Elysium_Labs/themis/commit/0173c8e40e89cbe306d70a1bbe37aea24a52a0c8))
- Merge pull request 'fix(lynis): fail fast on non-root instead of after the audit runs' (#18) from feat/native-checks-source into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/18 ([`21a0326`](https://codeberg.org/Elysium_Labs/themis/commit/21a0326b25324d818a6f5a1231b92dec15c2377f))
- Merge pull request 'feat(lynis): add --quick flag and nice/ionice priority wrapping' (#23) from feat/lynis-quick-nice into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/23 ([`8952f87`](https://codeberg.org/Elysium_Labs/themis/commit/8952f878d8cafb9056809b07f227140d806bb70d))
- Merge pull request 'fix(fail2ban): scope sshd bans to port, warn on WireGuard/CrowdSec conflicts' (#24) from fix/fail2ban-scope-bans-to-port into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/24 ([`cc3fd09`](https://codeberg.org/Elysium_Labs/themis/commit/cc3fd09b500dbaa38c46b8b8b75f2593cc69aa16))
- Merge pull request 'Add Apache 2.0 license, matching theia and eos' (#25) from chore/apache-license into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/25 ([`20c9bf7`](https://codeberg.org/Elysium_Labs/themis/commit/20c9bf7a37eac804436fb415b81d32764b087a6c))
- Merge pull request 'chore: update changelog for v0.0.1' (#26) from chore/changelog-v0.0.1 into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/26 ([`297cc38`](https://codeberg.org/Elysium_Labs/themis/commit/297cc38760be19d0e2271c93a5f4eb470980dd30))


### Testing
- Cover completion.go ([`f1b9e15`](https://codeberg.org/Elysium_Labs/themis/commit/f1b9e15224ac3b09a3fe776329cf8b42a01e8ee9))

## [0.0.1-rc.7] - 2026-07-14

### Features
- Group version/update/uninstall under `themis system` ([`f669918`](https://codeberg.org/Elysium_Labs/themis/commit/f669918999da75eff419d401395d11803c05c021))

## [0.0.1-rc.6] - 2026-07-14

### Miscellaneous
- Use ui package styling in apply/rollback output ([`90736cd`](https://codeberg.org/Elysium_Labs/themis/commit/90736cd720482630b211b8147b64f2ab31451365))
- Merge pull request 'style(cmd): use ui package styling in apply/rollback output' (#11) from fix/apply-rollback-ui-styling into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/11 ([`3b30907`](https://codeberg.org/Elysium_Labs/themis/commit/3b3090752b7202e2a00dc84f4a04d054d5cb7062))

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


### Miscellaneous
- Merge pull request 'feat: show version in default help output' (#6) from feat/show-version-in-help into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/6 ([`74366b5`](https://codeberg.org/Elysium_Labs/themis/commit/74366b5e342fc6d3d94d820f87f56a5305cbd659))
- Merge pull request 'fix(ci): generate real changelog notes for releases' (#7) from fix/release-notes-changelog into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/7 ([`aac4ba2`](https://codeberg.org/Elysium_Labs/themis/commit/aac4ba223eaadbf066b5145b86a38783488ae244))
- Merge pull request 'fix: resolve lynis outside PATH, add blank line after CLI errors' (#8) from fix/lynis-path-fallback into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/8 ([`2598f94`](https://codeberg.org/Elysium_Labs/themis/commit/2598f943a09e849e3b2e47e1fd7b04c070c53507))
- Merge pull request 'ci: add issue/PR templates, mirroring eos' (#9) from codeberg/issue-templates into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/9 ([`c44311f`](https://codeberg.org/Elysium_Labs/themis/commit/c44311faceed8a94aac74c6679cef53ce692177a))
- Merge pull request 'fix(ci): detect host arch for git-cliff install' (#10) from fix/git-cliff-arch-detect into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/10 ([`a07f389`](https://codeberg.org/Elysium_Labs/themis/commit/a07f389b957107816510cdc314df40df237347b5))

## [0.0.1-rc.4] - 2026-07-14

### Features
- Add --pre flag to themis update, inline release logic into cmd ([`22b30e9`](https://codeberg.org/Elysium_Labs/themis/commit/22b30e9d707d8f6bc666459948ff067074b84fae))


### Miscellaneous
- Merge pull request 'feat: add --pre flag to themis update, inline release logic into cmd' (#5) from feat/update-prerelease-support into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/5 ([`cf5ac3a`](https://codeberg.org/Elysium_Labs/themis/commit/cf5ac3ac854e9c5247f21e90373553d76d1954b2))

## [0.0.1-rc.3] - 2026-07-14

### Features
- Guard SSH password-auth fix against lockout, fix nilaway finding ([`4475c63`](https://codeberg.org/Elysium_Labs/themis/commit/4475c6307f47ed9cdb6952f44db17350b46ccab7))


### Miscellaneous
- Merge pull request 'feat: guard SSH password-auth fix against lockout, fix nilaway finding' (#4) from feat/ssh-lockout-guard into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/4 ([`ce89380`](https://codeberg.org/Elysium_Labs/themis/commit/ce89380b30f4cba018c922c9e55fcd4b52d6563f))

## [0.0.1-rc.2] - 2026-07-14

### Bug Fixes
- Satisfy golangci-lint (errcheck, gosec, govet shadow, misspell) ([`eccadef`](https://codeberg.org/Elysium_Labs/themis/commit/eccadefc79ff62c70f2ef31aadf41e375f91d5de))


### Features
- Human-readable errors and audit spinner ([`85c7224`](https://codeberg.org/Elysium_Labs/themis/commit/85c7224c1eadebdc41c30939060ed9f5d1a3ad4e))
- Add themis update and uninstall commands ([`bf3ac60`](https://codeberg.org/Elysium_Labs/themis/commit/bf3ac602750795c70298d8aa5843b70f31242349))


### Maintenance
- Add lefthook config, mirroring eos ([`ce84a14`](https://codeberg.org/Elysium_Labs/themis/commit/ce84a1416c5648fef6f33e452b560fff5559b44d))
- Wire up fieldalignment tool, enable it in lefthook ([`e210702`](https://codeberg.org/Elysium_Labs/themis/commit/e2107028ba02ecb5684a54cfeaa243fe927f5ccf))


### Miscellaneous
- Merge pull request 'feat: human-readable errors and audit spinner' (#1) from feat/friendly-errors-and-spinner into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/1 ([`f2ca60f`](https://codeberg.org/Elysium_Labs/themis/commit/f2ca60fd1d602b95b6c6e087018472b88469461d))
- Merge pull request 'feat: add themis update and uninstall commands' (#2) from feat/update-uninstall into main

Reviewed-on: https://codeberg.org/Elysium_Labs/themis/pulls/2 ([`fe4c425`](https://codeberg.org/Elysium_Labs/themis/commit/fe4c4253fb6de6e1081d5e02b521f1da4a9b6c75))

## [0.0.1-rc.1] - 2026-07-13

### Features
- Add cobra CLI POC (check/plan/apply/rollback) ([`da90ed9`](https://codeberg.org/Elysium_Labs/themis/commit/da90ed9280ec98400e79b2f67ddc4fec35f2cb73))
- Merge Lynis findings with themis fixes, styled table, api check ([`5252724`](https://codeberg.org/Elysium_Labs/themis/commit/5252724df4576a9e9e05fa83009d4b980f306441))
- Replace findings table with block list, never fully hide findings ([`8144323`](https://codeberg.org/Elysium_Labs/themis/commit/81443238a66ddf1a736fb1df58f2c064feaac0e9))
- Add release infra — buildinfo, install.sh, README, release workflow ([`6a0f426`](https://codeberg.org/Elysium_Labs/themis/commit/6a0f42628808b400eeff7fd0763994c6a894e3ab))


### Miscellaneous
- Initial commit ([`873ce3d`](https://codeberg.org/Elysium_Labs/themis/commit/873ce3dc288f6fa30ca5c2ec001af3f5ea03db4b))

