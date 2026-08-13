# Changelog

All notable changes to themis are documented here.

## [0.0.4] - 2026-08-13

### Bug Fixes
- Use [[ ]] instead of [ ] in conditionals (#123) ([`50a8979`](https://github.com/Elysium-Labs-EU/themis/commit/50a8979c9e0f50db91aef276f03c45ada314882c))
- Return partial revertData on late Apply failure (#126) (#127) ([`82abcc3`](https://github.com/Elysium-Labs-EU/themis/commit/82abcc3cde04db235eaa43f02f0817382ff7c276))


### CI/CD
- Add typo, mod-verify, file-size, and any-convention gates (#108) ([`d0535fa`](https://github.com/Elysium-Labs-EU/themis/commit/d0535fa34e667f543403c7d3082a876123b88c0f))
- SHA-pin actions, enforce HTTPS on downloads, scope release perms (#119) ([`4d179c0`](https://github.com/Elysium-Labs-EU/themis/commit/4d179c08e0c0b7b2c7cfc35ad1e33d9d710dc905))
- Pin remaining actions and harden gitleaks curl download (#125) ([`c455653`](https://github.com/Elysium-Labs-EU/themis/commit/c45565349f896797533ed67bd775ef800170d3d4))


### Features
- Add gitleaks scanning and govulncheck to CI (#109) ([`e8634d8`](https://github.com/Elysium-Labs-EU/themis/commit/e8634d877ae411c8802531f91a07b38c0e88821d))
- Add SonarQube Cloud scanning to CI (#111)

* docs: rewrite README around config, sources, and scheduling; move release integrity to SECURITY.md

* Add SonarQube Cloud scanning to CI

Go isn't covered by Sonar's Automatic Analysis mode, so this adds its
own workflow: generate coverage.out via go test, then run the
SonarQube scan action against it.

* SHA-pin actions in sonarqube.yml to satisfy Sonar's supply-chain rule

CI-based scan run flagged the tag-pinned sonarqube-scan-action as a
new-code security vulnerability (githubactions:S7637). Pinning by
full commit SHA with the version kept as a comment.

---------

Co-authored-by: R Tuerlings <sgnilreutr@noreply.codeberg.org> ([`37622ae`](https://github.com/Elysium-Labs-EU/themis/commit/37622aeb934e968d5542e4c8e7013622af6c9f8e))


### Maintenance
- Enable Dependabot for gomod and github-actions (#99) ([`079c2e6`](https://github.com/Elysium-Labs-EU/themis/commit/079c2e64923d7a7c1b4fe05cd592e1af07c42d38))
- Bump softprops/action-gh-release from 2 to 3 (#100) ([`deebabb`](https://github.com/Elysium-Labs-EU/themis/commit/deebabb7e21a2c327948a648aa12009c6098e4a6))
- Bump actions/upload-artifact from 6 to 7 (#101) ([`09c9bcd`](https://github.com/Elysium-Labs-EU/themis/commit/09c9bcdc85eb8830e1124f4c07b8aa4ba00659ac))
- Bump actions/setup-go from 6 to 7 (#102) ([`f8250a8`](https://github.com/Elysium-Labs-EU/themis/commit/f8250a88d321da67d7ee8718488ad3980de58585))
- Bump actions/download-artifact from 7 to 8 (#104) ([`82f7f63`](https://github.com/Elysium-Labs-EU/themis/commit/82f7f63142e727379408192d222b9587a74c04b7))
- Bump github.com/mattn/go-isatty (#103) ([`91c45dc`](https://github.com/Elysium-Labs-EU/themis/commit/91c45dc591b52c97e02fa3971858d3a838d5e7c9))
- Remove dead dataValidators.ts and object.ts (#118) ([`e918f76`](https://github.com/Elysium-Labs-EU/themis/commit/e918f76ba6b503955a7398dfedcbb77e5d91b460))
- Give each worktree its own golangci-lint cache (#129) ([`7e450aa`](https://github.com/Elysium-Labs-EU/themis/commit/7e450aa733ec8c3700b352092cb968f2e6b0c7d2))


### Refactoring
- Centralize THEMIS-* identifier constants (#120) ([`ae78e2d`](https://github.com/Elysium-Labs-EU/themis/commit/ae78e2d6f7255ff1077220656d1fea7cd8f02609))
- Reduce cognitive complexity of SonarQube-flagged functions (#122) ([`fc03273`](https://github.com/Elysium-Labs-EU/themis/commit/fc032734e47adaa66b969ac8e967611b46d35bba))


### Testing
- Cover exec paths via injected command runner (#106) ([`2e53f2a`](https://github.com/Elysium-Labs-EU/themis/commit/2e53f2aa53fe5d9453ee09c5cd3ee10ce7694180))
- Raise openscap/config/lynis coverage to 80%+ (#107) ([`f590e2f`](https://github.com/Elysium-Labs-EU/themis/commit/f590e2fe2e6d7047d9dfaa59745922e2f1283631))
- Cover api_check, privilege, osquery Query, ui spinner/prompt/error (#121) ([`fd67fdf`](https://github.com/Elysium-Labs-EU/themis/commit/fd67fdf9b61ad425597329849a91376ae683125f))

## [0.0.4-rc.1] - 2026-07-26

### Bug Fixes
- Themis-fix-issue-81 (#81) (#87) ([`0b54bce`](https://github.com/Elysium-Labs-EU/themis/commit/0b54bce75c7cb75d7e5fc76517643e0c2cc1c2eb))
- Themis-fix-issue-82 (#82) (#88) ([`afbf30b`](https://github.com/Elysium-Labs-EU/themis/commit/afbf30b39f3b575f9b1810d7bcbef073ea4171c3))
- Themis-fix-issue-83 (#83) (#90) ([`755d3cc`](https://github.com/Elysium-Labs-EU/themis/commit/755d3cca6468892b86faf2db3c762efc804e2bfe))
- Themis-fix-issue-84 (#84) (#91) ([`71b0f8f`](https://github.com/Elysium-Labs-EU/themis/commit/71b0f8f307553fa5ac0d7d709fdbd525dba7790a))
- Themis-fix-issue-85 (#85) (#92) ([`e071bbb`](https://github.com/Elysium-Labs-EU/themis/commit/e071bbbb96e30b9a9c4c7a7fc04738047cbb23d8))
- Harden untrusted audit-source input (#93) ([`f256abc`](https://github.com/Elysium-Labs-EU/themis/commit/f256abc48b56d16d4699ce5b0fa06854bf3b02eb))


### Documentation
- Rewrite README around config, sources, and scheduling; move release integrity to SECURITY.md (#94) ([`cce7e5e`](https://github.com/Elysium-Labs-EU/themis/commit/cce7e5e56f70b6f68c9f1fa24b8013351ce5a8e1))

## [0.0.3] - 2026-07-25

### Bug Fixes
- Themis-fix-issue-79 (#79) (#80) ([`de02e4a`](https://github.com/Elysium-Labs-EU/themis/commit/de02e4ab1bf35e7ca8d181d132e7fa994557c70f))

## [0.0.3-rc.4] - 2026-07-25

### Bug Fixes
- Themis-fix-issue-5 (#5) (#77) ([`811327d`](https://github.com/Elysium-Labs-EU/themis/commit/811327d1a6d92e2d030122ca7330c08d7fc2e60f))
- Themis-fix-issue-73 (#73) (#75) ([`323681c`](https://github.com/Elysium-Labs-EU/themis/commit/323681c58111f3ea0625140c1922d4e3df36abaa))
- Themis-fix-issue-74 (#74) (#76) ([`2bc1728`](https://github.com/Elysium-Labs-EU/themis/commit/2bc1728ef1ee3bdbbb6851c6459bec74792f1207))
- Smarter/scoped lynis scans via skip-if-unchanged fingerprinting (#78) ([`71aa6f8`](https://github.com/Elysium-Labs-EU/themis/commit/71aa6f81ae2e116c3226ed77b5c375c3fc9d2ecf))

## [0.0.3-rc.3] - 2026-07-25

### Bug Fixes
- Fix-issue-60 (#60) (#62) ([`04ebbc5`](https://github.com/Elysium-Labs-EU/themis/commit/04ebbc5dfbb2755d41f47aaffafa1f811f81770f))
- Fix-issue-2 (#2) (#61) ([`633de09`](https://github.com/Elysium-Labs-EU/themis/commit/633de09a7e0aedbc40b5380f31b7881c13a7d74f))
- Fix-issue-54 (#54) (#63) ([`6db7c0d`](https://github.com/Elysium-Labs-EU/themis/commit/6db7c0ddbedfbf9344b77b29a47f7a1f1f521c1a))
- Themis-fix-issue-14 (#14) (#64) ([`f147a0f`](https://github.com/Elysium-Labs-EU/themis/commit/f147a0fc55c6c39b8e752c4424b189b885715558))
- Themis-fix-issue-17 (#17) (#66) ([`d5a9ba5`](https://github.com/Elysium-Labs-EU/themis/commit/d5a9ba509eb1f442c815931e94cee259cfb764f6))
- Themis-fix-issue-16 (#16) (#65) ([`afc82dc`](https://github.com/Elysium-Labs-EU/themis/commit/afc82dca55af199474ae6766bf51aa8088cd56e3))
- Themis-fix-issue-13 (#13) (#67) ([`8d21394`](https://github.com/Elysium-Labs-EU/themis/commit/8d21394cf29e19c85557a3e437c66d8c2542dc72))
- Themis-fix-issue-12 (#12) (#68) ([`0be2e81`](https://github.com/Elysium-Labs-EU/themis/commit/0be2e814a677eb14bb03f70ae4c783a329ff4219))
- Themis-fix-issue-21 (#21) (#70) ([`6b7dd78`](https://github.com/Elysium-Labs-EU/themis/commit/6b7dd784df33d31ca1b3cefb05fa9a0d22e58f50))
- Themis-fix-issue-20 (#20) (#69) ([`759c207`](https://github.com/Elysium-Labs-EU/themis/commit/759c2078739f1590caa75806d18ef4f60685b16f))
- Themis-fix-issue-9 (#9) (#71) ([`8df37fe`](https://github.com/Elysium-Labs-EU/themis/commit/8df37fea333575c995e560b092849f1dfbcdaca4))
- Themis-fix-issue-11 (#11) (#72) ([`0dfe93e`](https://github.com/Elysium-Labs-EU/themis/commit/0dfe93ede8440d3ff647d0303e186e469aa7f1ee))

## [0.0.3-rc.2] - 2026-07-25

### Bug Fixes
- Persist rollback state after every successful fix, not just at loop end ([`8d5ffcf`](https://github.com/Elysium-Labs-EU/themis/commit/8d5ffcf4ff85a0bcc34f42ca56d3fa9c89e10b95))
- Stop scanning past sshd_config Match blocks when reading/patching directives ([`bded73c`](https://github.com/Elysium-Labs-EU/themis/commit/bded73c369c056a4f1000b6392c65aab7dc54000))
- Allow sshd's configured port before enabling default-deny ([`c1f2d7e`](https://github.com/Elysium-Labs-EU/themis/commit/c1f2d7e1823fa88b7ae0782dc9a63d964eae6dcf))
- Record and persist a fix's partial revert data on Apply() error (#25) ([`2e3b007`](https://github.com/Elysium-Labs-EU/themis/commit/2e3b00755fc444b6053f53d84760346db5424847))
- Fix-issue-27 (#27) ([`8eb8f41`](https://github.com/Elysium-Labs-EU/themis/commit/8eb8f4169ed2dbe4a6b173c1ea33d7f290a5edd6))
- Sync install.sh signing key, gate drift in CI, close installer-script integrity gap (#31) ([`d4a01d5`](https://github.com/Elysium-Labs-EU/themis/commit/d4a01d574eb65be287762a1ebbe663ba9440b4ca))
- Themis-fix-issue-32 (#32) (#33) ([`57a15eb`](https://github.com/Elysium-Labs-EU/themis/commit/57a15eb9b6b2ebc76a274774fd1bf3c609b8ae52))
- Themis-fix-issue-3 (#3) (#34) ([`7ce7dc1`](https://github.com/Elysium-Labs-EU/themis/commit/7ce7dc1cdefd6f87f07ff819bef5219c966eebf9))
- Themis-fix-issue-4 (#4) (#35) ([`7b9bdbd`](https://github.com/Elysium-Labs-EU/themis/commit/7b9bdbd0a62c1035d1c06084ad4e0771ca07c987))
- Fix-issue-55 (#55) (#56) ([`5db1bf5`](https://github.com/Elysium-Labs-EU/themis/commit/5db1bf5ca9c976c09e9d6aa9dd3945dbc4c66015))
- Fix-issue-50 (#50) (#57) ([`3d3c3c7`](https://github.com/Elysium-Labs-EU/themis/commit/3d3c3c74bcceaeb7bfde69f390d8057e418357e5))
- Fix-issue-26 (#26) (#58) ([`6d7d3e0`](https://github.com/Elysium-Labs-EU/themis/commit/6d7d3e0acebad19b0ff8fce17dad43b9c711e23f))
- Untrack .claude/argus/ control files from main, gitignore them (#59) ([`c323905`](https://github.com/Elysium-Labs-EU/themis/commit/c323905308647670107872f4d0fba9e0cc6c7d00))

## [0.0.3-rc.1] - 2026-07-20

### Bug Fixes
- Git-cliff can't resolve version tag on workflow_dispatch runs ([`d6f685f`](https://github.com/Elysium-Labs-EU/themis/commit/d6f685f808152b8d171e8b5f94c233b43cac9d2f))


### Maintenance
- Rotate release-signing public key ([`240476b`](https://github.com/Elysium-Labs-EU/themis/commit/240476bef7caac9c7cd58dc14abc5b41ff639521))

## [0.0.2] - 2026-07-19

### Bug Fixes
- Resolve external commands from trusted dirs, not $PATH ([`d80b568`](https://github.com/Elysium-Labs-EU/themis/commit/d80b56824168d514200adbf3d50689b9262dea63))
- Verify state.json integrity on load ([`1ebe2c9`](https://github.com/Elysium-Labs-EU/themis/commit/1ebe2c9787228f0640fac711d130f2c564d6b397))


### CI/CD
- Make go-crap gate change-scoped to PR churn ([`cc979fb`](https://github.com/Elysium-Labs-EU/themis/commit/cc979fbeb7425acb243d25c24f1ac772153ce910))
- Run Go jobs in container with named-volume caches ([`49a1e48`](https://github.com/Elysium-Labs-EU/themis/commit/49a1e48fec933d96789c06ee144b5a89a89bae55))
- Drop dead permissions field from Forgejo workflows ([`fece221`](https://github.com/Elysium-Labs-EU/themis/commit/fece221390a25c4b2c06eb11b5bba9e1d94cae4f))
- Add Integration Test job ([`4151729`](https://github.com/Elysium-Labs-EU/themis/commit/4151729794b6dc155e7617c0d791b30ce1789635))
- Keep Integration Test job green on the shared runner ([`0c14356`](https://github.com/Elysium-Labs-EU/themis/commit/0c14356a451e2982e06b28608ce2fc114dc06b3c))
- Tune OSV scan (deps-only PRs + weekly cron, prebuilt scanner) ([`e01002e`](https://github.com/Elysium-Labs-EU/themis/commit/e01002e43e372a32d8d9ba4942a5936042791809))


### Features
- Verify update signatures with soft-fail fallback ([`1b12ab2`](https://github.com/Elysium-Labs-EU/themis/commit/1b12ab2340be93f7f26666d2c54dfd0e8c0933c4))


### Maintenance
- Remove dead GitHub-Actions-only permissions block from workflows ([`1fb70b4`](https://github.com/Elysium-Labs-EU/themis/commit/1fb70b4d4c8757acc764a1f333da4b46332ef840))
- Add go-crap as a hard-blocking CI/local gate ([`64fc517`](https://github.com/Elysium-Labs-EU/themis/commit/64fc5170048c4ea2857438bb094136398c0703f2))
- Run go-crap gate on pre-push, not pre-commit ([`876393c`](https://github.com/Elysium-Labs-EU/themis/commit/876393cd286e2c68f64bc57b8b30dfa353f0738a))
- Exclude cobra command builders from the gate ([`d39dd08`](https://github.com/Elysium-Labs-EU/themis/commit/d39dd08eae73fd4b07dbdf9ff2b1b2b16f2a8313))


### Miscellaneous
- Add integration + smoke-update make targets ([`a947f3d`](https://github.com/Elysium-Labs-EU/themis/commit/a947f3dab243e6e8952748d8eba92bf08c7b533f))
- Stage release download in a private mktemp dir ([`1db51a8`](https://github.com/Elysium-Labs-EU/themis/commit/1db51a8ec93e0a1a275da669e5a6884311d71151))
- Migrate from Codeberg to GitHub

- Rename Go module path codeberg.org/Elysium_Labs/themis -> github.com/Elysium-Labs-EU/themis (all imports, go.mod, Makefile)
- Replace .forgejo Woodpecker/Gitea workflows with GitHub Actions (ci, golangci-lint, osv-scan, release); ubuntu-latest runners
- Rewrite release.yml to publish via softprops/action-gh-release with GITHUB_TOKEN; keep the ECDSA sha256sums.txt signing step
- Repoint install.sh + cmd/update.go at github.com / api.github.com; add required User-Agent header for the GitHub API; tolerate spaced JSON; keep embedded release-signing public key
- Retarget update tests to the github.com download host
- Update README badge/links, cliff.toml, CHANGELOG commit links; add logo ([`38ae8bf`](https://github.com/Elysium-Labs-EU/themis/commit/38ae8bf15edc92cb6d456dc15103c0fe60665952))


### Refactoring
- Extract fail2ban/autoupdates/firewall Check-Apply-Revert helpers ([`734a2a9`](https://github.com/Elysium-Labs-EU/themis/commit/734a2a9035e12da2ea3e01698677e85ad99c8690))
- Split runUpdate and downloadFile into helpers ([`00429d8`](https://github.com/Elysium-Labs-EU/themis/commit/00429d8b05f28f3949f714321004181ed93715fb))
- Extract report-printing and lynis-run helpers ([`6e34d39`](https://github.com/Elysium-Labs-EU/themis/commit/6e34d39477857aec41ce27268d5da1417d2461d5))


### Testing
- Add real lynis audit integration test ([`c7d5131`](https://github.com/Elysium-Labs-EU/themis/commit/c7d51315bb877816a30e8e23944bd8aedb51c67c))
- Add apply/revert integration tests for host-mutating fixes ([`a542be3`](https://github.com/Elysium-Labs-EU/themis/commit/a542be3ad358e41ff8c2ab11af92f682737609c9))
- Add hermetic end-to-end self-update integration test ([`9aadbe7`](https://github.com/Elysium-Labs-EU/themis/commit/9aadbe754cbdeb89cc8383e24bb44c9d3f066c7d))
- Gate host-mutating fix tests behind THEMIS_INTEGRATION_MUTATE ([`6696944`](https://github.com/Elysium-Labs-EU/themis/commit/6696944ebe18a591a66c2ab468d34c9f12b44d8c))

## [0.0.1] - 2026-07-16

### Bug Fixes
- Fail fast on non-root instead of after the audit runs ([`f727163`](https://github.com/Elysium-Labs-EU/themis/commit/f7271639fc741d819b5e1bf13fddaf830aced6f8))
- Scope sshd bans to port, warn on WireGuard/CrowdSec conflicts ([`63eaa9f`](https://github.com/Elysium-Labs-EU/themis/commit/63eaa9ff89863ba1516699fd289600f836a62101))
- Skip merge commits and changelog-bump commits from changelog ([`2cbbf08`](https://github.com/Elysium-Labs-EU/themis/commit/2cbbf0831be2f700e5d48f04e4cf00d0466b9124))
- Use full GitHub URL for osv-scanner-action, not mirrored on Forgejo ([`08451c5`](https://github.com/Elysium-Labs-EU/themis/commit/08451c5164ebe7351b1efcea55b69bddaa590374))
- Allow GOTOOLCHAIN auto-upgrade for osv-scanner install ([`90c8b8c`](https://github.com/Elysium-Labs-EU/themis/commit/90c8b8cf3ce76ebbe7f8f19fe790cb52c84e6e7a))
- Atomic binary swap in install.sh ([`5cd07a2`](https://github.com/Elysium-Labs-EU/themis/commit/5cd07a2bab32db13e0252a31613041d9477c5322))


### CI/CD
- Add OSV scanner workflow for PRs to main ([`0c918d3`](https://github.com/Elysium-Labs-EU/themis/commit/0c918d3a7cd8f01b8d756732566c3fb8b6c33171))
- Run osv-scanner CLI directly instead of GitHub Action wrapper ([`7f5850e`](https://github.com/Elysium-Labs-EU/themis/commit/7f5850e9ac6f2215390ce8352607dc2d9db5c606))


### Features
- Add interactive shell completion, ported from eos ([`c21370e`](https://github.com/Elysium-Labs-EU/themis/commit/c21370e4b56d0b66059d4faf0a482a0e337a9374))
- Introduce Source interface, decouple check/api from Lynis ([`bf9c9be`](https://github.com/Elysium-Labs-EU/themis/commit/bf9c9be07b2787d3d3075056118140d3ea6367c7))
- Add --quick flag and nice/ionice priority wrapping ([`5471776`](https://github.com/Elysium-Labs-EU/themis/commit/547177661fd921f954931a289735a3fba32a2064))
- Add Apache 2.0 license, matching theia and eos ([`0f90838`](https://github.com/Elysium-Labs-EU/themis/commit/0f9083883cf9091db05b0857e9fd0107951d43a7))
- Add themis-native audit source (closes #15) ([`6a1d5cb`](https://github.com/Elysium-Labs-EU/themis/commit/6a1d5cb2b06f978cf3da6fda1800566387d95aa9))
- Add top-level --version/-v flag ([`1c1ad37`](https://github.com/Elysium-Labs-EU/themis/commit/1c1ad37b10a0fd6ceffd7ec58b5f2c3ebc436a88))


### Testing
- Cover completion.go ([`f1b9e15`](https://github.com/Elysium-Labs-EU/themis/commit/f1b9e15224ac3b09a3fe776329cf8b42a01e8ee9))

## [0.0.1-rc.7] - 2026-07-14

### Features
- Group version/update/uninstall under `themis system` ([`f669918`](https://github.com/Elysium-Labs-EU/themis/commit/f669918999da75eff419d401395d11803c05c021))

## [0.0.1-rc.6] - 2026-07-14

### Miscellaneous
- Use ui package styling in apply/rollback output ([`90736cd`](https://github.com/Elysium-Labs-EU/themis/commit/90736cd720482630b211b8147b64f2ab31451365))

## [0.0.1-rc.5] - 2026-07-14

### Bug Fixes
- Drop Lynis from themis's top-level CLI description ([`9b2175c`](https://github.com/Elysium-Labs-EU/themis/commit/9b2175c7d98b5e18446e7b290f3499c87f3ed029))
- Generate real changelog notes for releases ([`f4130b2`](https://github.com/Elysium-Labs-EU/themis/commit/f4130b213464028f72cb4ddab8836e2db8ad7106))
- Resolve lynis outside PATH, add blank line after CLI errors ([`490d0bd`](https://github.com/Elysium-Labs-EU/themis/commit/490d0bd02a71cbe5c37f6854181ce9f65c19984c))
- Detect host arch for git-cliff install ([`880db07`](https://github.com/Elysium-Labs-EU/themis/commit/880db07e629f69598b2d035a771ed71a1a5b4abe))


### CI/CD
- Add issue/PR templates, mirroring eos ([`cbd5ad7`](https://github.com/Elysium-Labs-EU/themis/commit/cbd5ad7688fa7e9fb268c4dbf916484efa765de2))


### Features
- Show version in default help output ([`b73cfa6`](https://github.com/Elysium-Labs-EU/themis/commit/b73cfa69373003ae3cd576de9a1f226223ac4c14))


### Improvements
- Update README.md ([`c2bff89`](https://github.com/Elysium-Labs-EU/themis/commit/c2bff89210c116fa62e7936f565ebd28a386a757))

## [0.0.1-rc.4] - 2026-07-14

### Features
- Add --pre flag to themis update, inline release logic into cmd ([`22b30e9`](https://github.com/Elysium-Labs-EU/themis/commit/22b30e9d707d8f6bc666459948ff067074b84fae))

## [0.0.1-rc.3] - 2026-07-14

### Features
- Guard SSH password-auth fix against lockout, fix nilaway finding ([`4475c63`](https://github.com/Elysium-Labs-EU/themis/commit/4475c6307f47ed9cdb6952f44db17350b46ccab7))

## [0.0.1-rc.2] - 2026-07-14

### Bug Fixes
- Satisfy golangci-lint (errcheck, gosec, govet shadow, misspell) ([`eccadef`](https://github.com/Elysium-Labs-EU/themis/commit/eccadefc79ff62c70f2ef31aadf41e375f91d5de))


### Features
- Human-readable errors and audit spinner ([`85c7224`](https://github.com/Elysium-Labs-EU/themis/commit/85c7224c1eadebdc41c30939060ed9f5d1a3ad4e))
- Add themis update and uninstall commands ([`bf3ac60`](https://github.com/Elysium-Labs-EU/themis/commit/bf3ac602750795c70298d8aa5843b70f31242349))


### Maintenance
- Add lefthook config, mirroring eos ([`ce84a14`](https://github.com/Elysium-Labs-EU/themis/commit/ce84a1416c5648fef6f33e452b560fff5559b44d))
- Wire up fieldalignment tool, enable it in lefthook ([`e210702`](https://github.com/Elysium-Labs-EU/themis/commit/e2107028ba02ecb5684a54cfeaa243fe927f5ccf))

## [0.0.1-rc.1] - 2026-07-13

### Features
- Add cobra CLI POC (check/plan/apply/rollback) ([`da90ed9`](https://github.com/Elysium-Labs-EU/themis/commit/da90ed9280ec98400e79b2f67ddc4fec35f2cb73))
- Merge Lynis findings with themis fixes, styled table, api check ([`5252724`](https://github.com/Elysium-Labs-EU/themis/commit/5252724df4576a9e9e05fa83009d4b980f306441))
- Replace findings table with block list, never fully hide findings ([`8144323`](https://github.com/Elysium-Labs-EU/themis/commit/81443238a66ddf1a736fb1df58f2c064feaac0e9))
- Add release infra — buildinfo, install.sh, README, release workflow ([`6a0f426`](https://github.com/Elysium-Labs-EU/themis/commit/6a0f42628808b400eeff7fd0763994c6a894e3ab))


### Miscellaneous
- Initial commit ([`873ce3d`](https://github.com/Elysium-Labs-EU/themis/commit/873ce3dc288f6fa30ca5c2ec001af3f5ea03db4b))

