# Changelog

## [1.1.3](https://github.com/devsy-org/devsy-provider-ssh/compare/v1.1.2...v1.1.3) (2026-06-28)


### Bug Fixes

* **deps:** update module github.com/onsi/ginkgo/v2 to v2.32.0 ([#22](https://github.com/devsy-org/devsy-provider-ssh/issues/22)) ([1d0f9fa](https://github.com/devsy-org/devsy-provider-ssh/commit/1d0f9fa57ba9b87c608603bac8321c945fd76196))

## [1.1.2](https://github.com/devsy-org/devsy-provider-ssh/compare/v1.1.1...v1.1.2) (2026-06-27)


### Bug Fixes

* **e2e:** update devsy to v1.0.0 and fix CLI command forms ([#13](https://github.com/devsy-org/devsy-provider-ssh/issues/13)) ([ca21bea](https://github.com/devsy-org/devsy-provider-ssh/commit/ca21bead7195153940b2c01988b2751bfd985bff))

## [1.1.1](https://github.com/devsy-org/devsy-provider-ssh/compare/v1.1.0...v1.1.1) (2026-04-19)


### Bug Fixes

* replace broken badge SVG with shields.io badge ([#3](https://github.com/devsy-org/devsy-provider-ssh/issues/3)) ([a73d0c4](https://github.com/devsy-org/devsy-provider-ssh/commit/a73d0c43783bd7499553f050983b86c598e1b2e9))

## [1.1.0](https://github.com/devsy-org/devsy-provider-ssh/compare/v1.0.0...v1.1.0) (2026-04-19)


### Features

* add golangci-lint rules ([#5](https://github.com/devsy-org/devsy-provider-ssh/issues/5)) ([a67ac5b](https://github.com/devsy-org/devsy-provider-ssh/commit/a67ac5b60db4a9b1777e50cb1c7f27a084f5eca4))
* add release-please for automated releases ([#60](https://github.com/devsy-org/devsy-provider-ssh/issues/60)) ([9161490](https://github.com/devsy-org/devsy-provider-ssh/commit/9161490265719479590f094c9f3d6d9230957212))
* add Taskfile and workflow-approval ([#63](https://github.com/devsy-org/devsy-provider-ssh/issues/63)) ([df31157](https://github.com/devsy-org/devsy-provider-ssh/commit/df31157f7c42f9cae103d99bb34ccde99e185796))
* rebrand from DevPod/skevetter to Devsy/devsy-org ([#1](https://github.com/devsy-org/devsy-provider-ssh/issues/1)) ([ad2014c](https://github.com/devsy-org/devsy-provider-ssh/commit/ad2014c64b8a91c04602ebf2e10c047ef7ec1fa9))
* rewrite ssh provider in golang ([68ea43a](https://github.com/devsy-org/devsy-provider-ssh/commit/68ea43a77893ebbdd0fb8316c6bc700739b6c3c7))
* rewrite ssh provider in golang ([145b674](https://github.com/devsy-org/devsy-provider-ssh/commit/145b674c9e4cf82e5e928e4876606fa49c3776c4))
* setup repository ([#1](https://github.com/devsy-org/devsy-provider-ssh/issues/1)) ([c3ff7ea](https://github.com/devsy-org/devsy-provider-ssh/commit/c3ff7ea853604e074c3ffe3536baec74c88cb041))


### Bug Fixes

* add custom port to windows command execution ([bd240fb](https://github.com/devsy-org/devsy-provider-ssh/commit/bd240fb8f77f3df595c0624de709702780e0a0a0))
* add custom port to windows command execution ([cfc9dc2](https://github.com/devsy-org/devsy-provider-ssh/commit/cfc9dc28783db4dac3569edaa4ada5f00cf60471))
* add echo to init command ([021fcdb](https://github.com/devsy-org/devsy-provider-ssh/commit/021fcdb5dc1ac0c76c4409594e58fc4d6223be2e))
* add option to use builtin SSH client, not enabled by default ([414cf94](https://github.com/devsy-org/devsy-provider-ssh/commit/414cf940f953844a14da1ae3eaa4cc970f135098))
* add option to use builtin SSH client, not enabled by default ([c6efcc3](https://github.com/devsy-org/devsy-provider-ssh/commit/c6efcc35004b590af456e8421c9d9922a39af462))
* add port flag only if non-standard port is specified ([2cc7172](https://github.com/devsy-org/devsy-provider-ssh/commit/2cc7172faa6895ccbd8bc4310de322355764e0a2))
* add port flag only if non-standard port is specified ([cc4438d](https://github.com/devsy-org/devsy-provider-ssh/commit/cc4438d5c0e36aed0b28ef75d52c7eeebf6ca788))
* add proper init checks to ssh provider ([9c40cbe](https://github.com/devsy-org/devsy-provider-ssh/commit/9c40cbe16836cc1c05931801da3fbb2a916cc328))
* add proper init checks to ssh provider ([5e67368](https://github.com/devsy-org/devsy-provider-ssh/commit/5e67368559177327f3c61348c69bea0b0153e25b))
* add proper init checks to ssh provider ([061db9e](https://github.com/devsy-org/devsy-provider-ssh/commit/061db9e3a9f02ccefb924c6e51b96e66ab0b1fa7))
* better defaults to work without sudo ([10395fe](https://github.com/devsy-org/devsy-provider-ssh/commit/10395fe675f56024b0020531615e5d266722cca6))
* better highlight Linux compatibility ([708772a](https://github.com/devsy-org/devsy-provider-ssh/commit/708772aa9678e54e94406a4c2f10caf24cfb590e))
* better highlight Linux compatibility ([cff6ef0](https://github.com/devsy-org/devsy-provider-ssh/commit/cff6ef081f1c563a9a5e297c9d19b281d6358083))
* check for exit code in init command ([f16d5e3](https://github.com/devsy-org/devsy-provider-ssh/commit/f16d5e3ea8b21646e8e4ef3aec1255c80ad1d4ed))
* check for exit code in init command ([82b520a](https://github.com/devsy-org/devsy-provider-ssh/commit/82b520ac078921c225ef2fae67d6e49fafd82c6a))
* clean out bytes between commands ([0ca0adc](https://github.com/devsy-org/devsy-provider-ssh/commit/0ca0adcab095290b7bc815b91e303da3d8e5cd98))
* correct spacing in logs ([39d942d](https://github.com/devsy-org/devsy-provider-ssh/commit/39d942dcaa8523411ab21e07d6d0df3bb9d759e6))
* **deps:** update github.com/skevetter/log digest to bfd26ab ([#6](https://github.com/devsy-org/devsy-provider-ssh/issues/6)) ([432ff24](https://github.com/devsy-org/devsy-provider-ssh/commit/432ff24ad6fca67e347a122e50940a4695ad0614))
* **deps:** update module github.com/goccy/go-yaml to v1.19.2 ([#24](https://github.com/devsy-org/devsy-provider-ssh/issues/24)) ([31e82aa](https://github.com/devsy-org/devsy-provider-ssh/commit/31e82aaa550b4d758c31ad6d85dc0ec91b939051))
* **deps:** update module github.com/onsi/ginkgo/v2 to v2.28.1 ([#9](https://github.com/devsy-org/devsy-provider-ssh/issues/9)) ([7c46163](https://github.com/devsy-org/devsy-provider-ssh/commit/7c461636f78a4122148afffb720b89dd2e13975b))
* **deps:** update module github.com/onsi/gomega to v1.39.1 ([#14](https://github.com/devsy-org/devsy-provider-ssh/issues/14)) ([01bf342](https://github.com/devsy-org/devsy-provider-ssh/commit/01bf342c62f3cb259ccc84f7e31cb71656bf3856))
* **deps:** update module github.com/skevetter/devpod to v0.13.0 ([#15](https://github.com/devsy-org/devsy-provider-ssh/issues/15)) ([d58e952](https://github.com/devsy-org/devsy-provider-ssh/commit/d58e952dbba847dc5d517607a339a708491a2ba7))
* **deps:** update module github.com/skevetter/devpod to v0.14.0 ([#17](https://github.com/devsy-org/devsy-provider-ssh/issues/17)) ([9da3a89](https://github.com/devsy-org/devsy-provider-ssh/commit/9da3a890aa94cd303b0167be08880dbbf83fd206))
* **deps:** update module github.com/skevetter/devpod to v0.14.1 ([#19](https://github.com/devsy-org/devsy-provider-ssh/issues/19)) ([30be52a](https://github.com/devsy-org/devsy-provider-ssh/commit/30be52affd319d1b06b8c8af1b65dfa8f17bfabf))
* **deps:** update module github.com/skevetter/devpod to v0.14.2 ([#20](https://github.com/devsy-org/devsy-provider-ssh/issues/20)) ([4ca0578](https://github.com/devsy-org/devsy-provider-ssh/commit/4ca0578734e94c6fdcb9ae11e475d821748a2d51))
* **deps:** update module github.com/skevetter/devpod to v0.14.3 ([#21](https://github.com/devsy-org/devsy-provider-ssh/issues/21)) ([435a95e](https://github.com/devsy-org/devsy-provider-ssh/commit/435a95e15dfeaeb199a5c79758e13849e6312449))
* **deps:** update module github.com/skevetter/devpod to v0.14.4 ([#22](https://github.com/devsy-org/devsy-provider-ssh/issues/22)) ([2e5b09a](https://github.com/devsy-org/devsy-provider-ssh/commit/2e5b09abc53a94087c9b9765552fe4c132a70fe0))
* **deps:** update module github.com/skevetter/devpod to v0.14.5 ([#25](https://github.com/devsy-org/devsy-provider-ssh/issues/25)) ([ee21685](https://github.com/devsy-org/devsy-provider-ssh/commit/ee21685fa9a37c27e2e94e2f83d348d5b3e317b8))
* **deps:** update module github.com/skevetter/devpod to v0.14.6 ([#26](https://github.com/devsy-org/devsy-provider-ssh/issues/26)) ([5145209](https://github.com/devsy-org/devsy-provider-ssh/commit/514520927dfb3eaae19df463bfeb7268dbd4c12c))
* **deps:** update module github.com/skevetter/devpod to v0.14.7 ([#27](https://github.com/devsy-org/devsy-provider-ssh/issues/27)) ([3b1248e](https://github.com/devsy-org/devsy-provider-ssh/commit/3b1248e5937707f81fd7ae76f3313a43caa8c774))
* **deps:** update module github.com/skevetter/devpod to v0.15.0 ([#28](https://github.com/devsy-org/devsy-provider-ssh/issues/28)) ([7b40769](https://github.com/devsy-org/devsy-provider-ssh/commit/7b40769d2cb5c15e392cdb25021035ad9dd59751))
* **deps:** update module github.com/skevetter/devpod to v0.15.1 ([#29](https://github.com/devsy-org/devsy-provider-ssh/issues/29)) ([c08d6f0](https://github.com/devsy-org/devsy-provider-ssh/commit/c08d6f071f509b5535db3bc5e65199a9248da041))
* **deps:** update module github.com/skevetter/devpod to v0.16.1 ([#30](https://github.com/devsy-org/devsy-provider-ssh/issues/30)) ([c7d4d16](https://github.com/devsy-org/devsy-provider-ssh/commit/c7d4d16fb4ce0a88d26b77cc33c9bea6a8c9ac6d))
* **deps:** update module github.com/skevetter/devpod to v0.16.2 ([#31](https://github.com/devsy-org/devsy-provider-ssh/issues/31)) ([b8a42ef](https://github.com/devsy-org/devsy-provider-ssh/commit/b8a42ef885f7ee63787210ab7d10873cea3b84a5))
* **deps:** update module github.com/skevetter/devpod to v0.16.3 ([#37](https://github.com/devsy-org/devsy-provider-ssh/issues/37)) ([73ad961](https://github.com/devsy-org/devsy-provider-ssh/commit/73ad9615e2997284fcd8da81b6ca51922b083646))
* **deps:** update module github.com/skevetter/devpod to v0.16.4 ([#39](https://github.com/devsy-org/devsy-provider-ssh/issues/39)) ([b910173](https://github.com/devsy-org/devsy-provider-ssh/commit/b9101739db01b433f93990895f183721ad42bf23))
* **deps:** update module github.com/skevetter/devpod to v0.16.6 ([#41](https://github.com/devsy-org/devsy-provider-ssh/issues/41)) ([20c5c46](https://github.com/devsy-org/devsy-provider-ssh/commit/20c5c460cdf5c3dd0e001725072d7a53f7fa2bbe))
* **deps:** update module github.com/skevetter/devpod to v0.17.0 ([#42](https://github.com/devsy-org/devsy-provider-ssh/issues/42)) ([d658a40](https://github.com/devsy-org/devsy-provider-ssh/commit/d658a40f7b6694e93bbfa33cbff95bb0e37ae3bd))
* **deps:** update module github.com/skevetter/devpod to v0.17.1 ([#43](https://github.com/devsy-org/devsy-provider-ssh/issues/43)) ([13e8f7f](https://github.com/devsy-org/devsy-provider-ssh/commit/13e8f7ff894c35279fcb193929026fd5c6fea3b5))
* **deps:** update module github.com/skevetter/devpod to v0.17.2 ([#44](https://github.com/devsy-org/devsy-provider-ssh/issues/44)) ([afe7e90](https://github.com/devsy-org/devsy-provider-ssh/commit/afe7e9059bf0a92e8ff1335d3b3f2d442e7f4745))
* **deps:** update module github.com/skevetter/devpod to v0.17.4 ([#45](https://github.com/devsy-org/devsy-provider-ssh/issues/45)) ([c14b07e](https://github.com/devsy-org/devsy-provider-ssh/commit/c14b07edcb4aa0f5b147e39b5a2f44c98123087c))
* **deps:** update module github.com/skevetter/devpod to v0.18.0 ([#47](https://github.com/devsy-org/devsy-provider-ssh/issues/47)) ([04ddde2](https://github.com/devsy-org/devsy-provider-ssh/commit/04ddde2fa24cc204c6c36f09121d45803af266a2))
* **deps:** update module github.com/skevetter/devpod to v0.18.1 ([#48](https://github.com/devsy-org/devsy-provider-ssh/issues/48)) ([ff2c92a](https://github.com/devsy-org/devsy-provider-ssh/commit/ff2c92af038e709df36fbe75f94f5834652d78e5))
* **deps:** update module github.com/skevetter/devpod to v0.18.2 ([#49](https://github.com/devsy-org/devsy-provider-ssh/issues/49)) ([c653194](https://github.com/devsy-org/devsy-provider-ssh/commit/c65319493345bd291ddcc807353daa1cf36e327c))
* **deps:** update module github.com/skevetter/devpod to v0.18.3 ([#50](https://github.com/devsy-org/devsy-provider-ssh/issues/50)) ([19a118f](https://github.com/devsy-org/devsy-provider-ssh/commit/19a118f80aa423197fa9f9bbbad9a03befa3be03))
* **deps:** update module github.com/skevetter/devpod to v0.18.5 ([#51](https://github.com/devsy-org/devsy-provider-ssh/issues/51)) ([403f707](https://github.com/devsy-org/devsy-provider-ssh/commit/403f70714456313b33b749707da5e1e300f12195))
* **deps:** update module github.com/skevetter/devpod to v0.18.6 ([#52](https://github.com/devsy-org/devsy-provider-ssh/issues/52)) ([75fd69c](https://github.com/devsy-org/devsy-provider-ssh/commit/75fd69c4035125570cec3f50ece8c16228017fc3))
* **deps:** update module github.com/skevetter/devpod to v0.19.0 ([#53](https://github.com/devsy-org/devsy-provider-ssh/issues/53)) ([986f8a0](https://github.com/devsy-org/devsy-provider-ssh/commit/986f8a0656136ed98a99203ef03205742278b067))
* **deps:** update module github.com/skevetter/devpod to v0.19.1 ([#54](https://github.com/devsy-org/devsy-provider-ssh/issues/54)) ([52d58a1](https://github.com/devsy-org/devsy-provider-ssh/commit/52d58a1d2e41442ed98989383746fc70133aa902))
* **deps:** update module github.com/skevetter/devpod to v0.19.2 ([#56](https://github.com/devsy-org/devsy-provider-ssh/issues/56)) ([8f840ef](https://github.com/devsy-org/devsy-provider-ssh/commit/8f840effe2802877718c85aa42cf3d79445db5d9))
* **deps:** update module github.com/skevetter/devpod to v0.19.3 ([#57](https://github.com/devsy-org/devsy-provider-ssh/issues/57)) ([4ea71e2](https://github.com/devsy-org/devsy-provider-ssh/commit/4ea71e21290a168d850bfa7b76d37503745c5352))
* **deps:** update module github.com/skevetter/devpod to v0.19.4 ([#59](https://github.com/devsy-org/devsy-provider-ssh/issues/59)) ([d415337](https://github.com/devsy-org/devsy-provider-ssh/commit/d4153378744179381e7d719d15e1f031f3240891))
* **deps:** update module golang.org/x/crypto to v0.49.0 ([#32](https://github.com/devsy-org/devsy-provider-ssh/issues/32)) ([d2f5b3f](https://github.com/devsy-org/devsy-provider-ssh/commit/d2f5b3f16346f0874d4727f264b4d23332d95e08))
* **deps:** update module golang.org/x/crypto to v0.50.0 ([#55](https://github.com/devsy-org/devsy-provider-ssh/issues/55)) ([385af8e](https://github.com/devsy-org/devsy-provider-ssh/commit/385af8e2f719ba17b158116ba885caef197a21ee))
* **e2e:** remove eventual spaces from expected outputs ([725e40f](https://github.com/devsy-org/devsy-provider-ssh/commit/725e40fc48e8d31d2c24798d8ab93b99208e4b25))
* error variable ([b7b5469](https://github.com/devsy-org/devsy-provider-ssh/commit/b7b5469e81d2101870f827c3e052435da97d5c89))
* **exec:** fallback logic for non-posix shell incompatibilities ([24c7f5e](https://github.com/devsy-org/devsy-provider-ssh/commit/24c7f5ee5e77a0a1cdead5f034b417c91013fd21))
* **exec:** fallback logic for non-posix shell incompatibilities ([b6212bf](https://github.com/devsy-org/devsy-provider-ssh/commit/b6212bf94fefc6886c19da1ca5ecdf4c4808d1d4))
* **exec:** force use of /tmp for destination ([2da52ee](https://github.com/devsy-org/devsy-provider-ssh/commit/2da52eea24ab17091ba8b08d0662cd27613ff9a0))
* **exec:** let it use OS's temp dir ([06a589a](https://github.com/devsy-org/devsy-provider-ssh/commit/06a589a09a86a52fc8b85e63ce26c89dfc82d89f))
* EXTRA_FLAGS is not mandatory ([bbc1600](https://github.com/devsy-org/devsy-provider-ssh/commit/bbc1600bcf2f6b8066ac0afedd9d98e8499719fc))
* handle errors in shell quoting split ([6f706e2](https://github.com/devsy-org/devsy-provider-ssh/commit/6f706e2ec7f527aefcd71490913d63dad2327a35))
* linting and formatting ([a8cd912](https://github.com/devsy-org/devsy-provider-ssh/commit/a8cd912bbd0a1344bf986d12a29f8a173be4d047))
* **manifest:** use per-user agent path ([d1580d3](https://github.com/devsy-org/devsy-provider-ssh/commit/d1580d3b7e8564f2025cb19eaea79ac83a58ae29))
* **manifest:** use per-user agent path ([b9445bb](https://github.com/devsy-org/devsy-provider-ssh/commit/b9445bbf3177b8e31e2945ed7fe994d9cd288250))
* openssh on windows doesn't play nicely with piping data through Stdin ([a9c6447](https://github.com/devsy-org/devsy-provider-ssh/commit/a9c6447f09a69a60943e4c299a0d9f34de9d9387))
* openssh on windows doesn't play nicely with piping data through Stdin. ([d7585cb](https://github.com/devsy-org/devsy-provider-ssh/commit/d7585cb11ae1bbe8d27ef1457d1eaff1fa111e19))
* properly set EXTRA_FLAGS when passed ([a475913](https://github.com/devsy-org/devsy-provider-ssh/commit/a475913047564848e156c104d5afed41a018b46c))
* remove dead code ([cf6b8c8](https://github.com/devsy-org/devsy-provider-ssh/commit/cf6b8c880a882a1a76454d95fcafe47428eeb2cf))
* remove private key config, it can be used with extra_flags now ([4874089](https://github.com/devsy-org/devsy-provider-ssh/commit/48740892a727e503cc0debc2d7968652bf26b3be))
* use proper shell splitting for EXTRA_FLAGS ([e142b19](https://github.com/devsy-org/devsy-provider-ssh/commit/e142b194ee5688fc3e769275902cd47cdaedf5b5))
* use ssh binary ([c53775c](https://github.com/devsy-org/devsy-provider-ssh/commit/c53775c3c5a28006688b79717898891526b05870))

## 1.0.0 (2026-04-19)

Initial release as Devsy SSH Provider, rebranded from DevPod.
