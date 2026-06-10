# Changelog

## [0.7.1](https://github.com/smartcontractkit/cld-changesets/compare/v0.7.0...v0.7.1) (2026-06-10)


### Bug Fixes

* use specific type to avoid requiring the address in datastore delete ([#84](https://github.com/smartcontractkit/cld-changesets/issues/84)) ([846077c](https://github.com/smartcontractkit/cld-changesets/commit/846077c6291a1f2ecf0f37a1dabb66cb305778f6))

## [0.7.0](https://github.com/smartcontractkit/cld-changesets/compare/v0.6.0...v0.7.0) (2026-06-10)


### ⚠ BREAKING CHANGES

* Delete `pkg/contract/mcms/view/v1_0` and `GenerateMCMSWithTimelockView`.

### Features

* remove EVM MCMS view generation ([#77](https://github.com/smartcontractkit/cld-changesets/issues/77)) ([dcfe322](https://github.com/smartcontractkit/cld-changesets/commit/dcfe322adee2dd8ff8cf89d0d3fbad9204631be9))


### Bug Fixes

* change type from address ref key to address ref ([#83](https://github.com/smartcontractkit/cld-changesets/issues/83)) ([23a126d](https://github.com/smartcontractkit/cld-changesets/commit/23a126d5b1bbf17f44f79fa729140113986c7adf))

## [0.6.0](https://github.com/smartcontractkit/cld-changesets/compare/v0.5.0...v0.6.0) (2026-06-02)


### Features

* add datastore chain metadata changeset ([#75](https://github.com/smartcontractkit/cld-changesets/issues/75)) ([7d0c300](https://github.com/smartcontractkit/cld-changesets/commit/7d0c30044951bca48cbed55eb369751cf27350fd))
* add datastore contract metadata changeset ([#74](https://github.com/smartcontractkit/cld-changesets/issues/74)) ([4b75d9b](https://github.com/smartcontractkit/cld-changesets/commit/4b75d9bfb2dec9b4f148161810d11adda43d74d3))
* add datastore delete address ref changeset ([#72](https://github.com/smartcontractkit/cld-changesets/issues/72)) ([11308c8](https://github.com/smartcontractkit/cld-changesets/commit/11308c8d8b19280d4c2e45cdb86eea79318f5cff))

## [0.5.0](https://github.com/smartcontractkit/cld-changesets/compare/v0.4.0...v0.5.0) (2026-05-29)


### ⚠ BREAKING CHANGES

* Deletes the ApproveToken helper and related tests.
* Removes the Solana MCMS state views from the public API.
* Removes the LinkToken and StaticLinkToken state views from the public API.
* Moves version constants out of the public API and into internal.
* Remove `ValidateSelectorsInEnvironment` method from `pkg/cldfutil/selectors.go`.

### Features

* delete ApproveToken helper ([#70](https://github.com/smartcontractkit/cld-changesets/issues/70)) ([83f59e7](https://github.com/smartcontractkit/cld-changesets/commit/83f59e7f7fce11d1cfe02c22f090a9f316e5b27d))
* delete Link state views ([#68](https://github.com/smartcontractkit/cld-changesets/issues/68)) ([d5f643b](https://github.com/smartcontractkit/cld-changesets/commit/d5f643babc970be9869004efbcc1a450163e7f7c))
* link refactor ([#71](https://github.com/smartcontractkit/cld-changesets/issues/71)) ([177dbc5](https://github.com/smartcontractkit/cld-changesets/commit/177dbc5ace4fa2435dc9e837c6e893ecccd2a5ca))
* remove Solana MCMS state views ([#69](https://github.com/smartcontractkit/cld-changesets/issues/69)) ([eb948dd](https://github.com/smartcontractkit/cld-changesets/commit/eb948dd2e436125b5c909c4519970268c783f476))
* remove ValidateSelectorsInEnvironment method ([#61](https://github.com/smartcontractkit/cld-changesets/issues/61)) ([72f431f](https://github.com/smartcontractkit/cld-changesets/commit/72f431f09aaecfaeb5d2368b8706087fe01902c9))


### Bug Fixes

* move contract version to internal package ([#64](https://github.com/smartcontractkit/cld-changesets/issues/64)) ([936a00b](https://github.com/smartcontractkit/cld-changesets/commit/936a00b387f20684064f01a7dd451820b7fb6fb5))

## [0.4.0](https://github.com/smartcontractkit/cld-changesets/compare/v0.3.0...v0.4.0) (2026-05-18)


### Features

* add changeset and operation to delete CRE workflow ([#54](https://github.com/smartcontractkit/cld-changesets/issues/54)) ([f0e341a](https://github.com/smartcontractkit/cld-changesets/commit/f0e341a654355183db5ff9d9c9e1697b5825282f))
* add support for multiple api key to deploy workflow ([#55](https://github.com/smartcontractkit/cld-changesets/issues/55)) ([a68f156](https://github.com/smartcontractkit/cld-changesets/commit/a68f156c62d19888e5a50897a7a2164bc1368251))
* transfer native ([#56](https://github.com/smartcontractkit/cld-changesets/issues/56)) ([4368e49](https://github.com/smartcontractkit/cld-changesets/commit/4368e4947a322e411790dbb61e5d6ad3aa1d3b3a))


### Bug Fixes

* use cache dir for sol programs loading ([#52](https://github.com/smartcontractkit/cld-changesets/issues/52)) ([b04f4d9](https://github.com/smartcontractkit/cld-changesets/commit/b04f4d9908495e6048a59a1f0892ea5d4ddddc37))

## [0.3.0](https://github.com/smartcontractkit/cld-changesets/compare/v0.2.0...v0.3.0) (2026-05-08)


### Features

* add catalog create address refs changeset ([#38](https://github.com/smartcontractkit/cld-changesets/issues/38)) ([c22ad31](https://github.com/smartcontractkit/cld-changesets/commit/c22ad3193d241da813c99df771c813790b2d6334))
* add catalog update address refs changeset ([#43](https://github.com/smartcontractkit/cld-changesets/issues/43)) ([a9de479](https://github.com/smartcontractkit/cld-changesets/commit/a9de4799ccb8056c8c24a15dee2ba9e2a5c562e0))
* add catalog update env metadata changeset ([#46](https://github.com/smartcontractkit/cld-changesets/issues/46)) ([5a793ef](https://github.com/smartcontractkit/cld-changesets/commit/5a793ef3149b2e05c0179762838b0025d89a610f))
* add sonar properties file ([#44](https://github.com/smartcontractkit/cld-changesets/issues/44)) ([cbc2fa5](https://github.com/smartcontractkit/cld-changesets/commit/cbc2fa5874651ff99b07400f9f947d05adafa32f))
* port deploy mcms with timelock ([#28](https://github.com/smartcontractkit/cld-changesets/issues/28)) ([39a62f2](https://github.com/smartcontractkit/cld-changesets/commit/39a62f2ae67efa2ec8aead9a2d88ea50e723842a))

## [0.2.0](https://github.com/smartcontractkit/cld-changesets/compare/v0.1.0...v0.2.0) (2026-05-06)


### Features

* add catalog update chain metadata changeset ([#37](https://github.com/smartcontractkit/cld-changesets/issues/37)) ([9ec54b5](https://github.com/smartcontractkit/cld-changesets/commit/9ec54b5a5b1e1f791fd4c87cb9aa7cbf3afdb676))
* add catalog update contract metadata changeset ([#29](https://github.com/smartcontractkit/cld-changesets/issues/29)) ([1687d14](https://github.com/smartcontractkit/cld-changesets/commit/1687d144f8f1313735fe469fab3c64fc5a7409fb))

## [0.1.0](https://github.com/smartcontractkit/cld-changesets/compare/cld-changesets-v0.0.1...cld-changesets-v0.1.0) (2026-05-06)


### Features

* add "catalog create chain metadata" changeset ([#36](https://github.com/smartcontractkit/cld-changesets/issues/36)) ([c12bb51](https://github.com/smartcontractkit/cld-changesets/commit/c12bb51b95e9928cd8e66cbb828e22e1a6e5c663))
* add catalog create contract metadata changeset ([#27](https://github.com/smartcontractkit/cld-changesets/issues/27)) ([8ec1cbc](https://github.com/smartcontractkit/cld-changesets/commit/8ec1cbcfabcf49e61fa9a59aaf5a4d203045062b))
* add CRE workflow deploy changeset and operation ([#1](https://github.com/smartcontractkit/cld-changesets/issues/1)) ([960fde1](https://github.com/smartcontractkit/cld-changesets/commit/960fde156a3e4d7e2cf30b5b63b46bce01b7aa0b))
* add target name param to cre deploy changeset ([#3](https://github.com/smartcontractkit/cld-changesets/issues/3)) ([8b25662](https://github.com/smartcontractkit/cld-changesets/commit/8b256625d481986e3a70c1d979813834455a2195))
* fund mcms pdas ([#22](https://github.com/smartcontractkit/cld-changesets/issues/22)) ([821e542](https://github.com/smartcontractkit/cld-changesets/commit/821e54277c0d7fb2be52f7ea535da91a4cf74fcd))
* **jobspec:** port jobspec changesets from chainlink ([#21](https://github.com/smartcontractkit/cld-changesets/issues/21)) ([0ce3f74](https://github.com/smartcontractkit/cld-changesets/commit/0ce3f7465dd7e85de86e0cefee1c2a74b5eaf82b))
* link token ([#30](https://github.com/smartcontractkit/cld-changesets/issues/30)) ([07345c1](https://github.com/smartcontractkit/cld-changesets/commit/07345c1b6d299bf02aa528daa14d315b0456215b))
* **pkg:** add contract constants and Solana MCMS state loading ([#5](https://github.com/smartcontractkit/cld-changesets/issues/5)) ([6a2bbee](https://github.com/smartcontractkit/cld-changesets/commit/6a2bbee2252207e64d248852b8865314954722bd))
* port BuildProposalFromBatchesV2 ([#24](https://github.com/smartcontractkit/cld-changesets/issues/24)) ([28d53d7](https://github.com/smartcontractkit/cld-changesets/commit/28d53d7600f997d8fa6a93b42947d4ce87af5080))
* port run changeset from chainlink ([#32](https://github.com/smartcontractkit/cld-changesets/issues/32)) ([693922f](https://github.com/smartcontractkit/cld-changesets/commit/693922ff4100b96c103b743f1d2a73a638a3db59))
* port solana grant role ([#33](https://github.com/smartcontractkit/cld-changesets/issues/33)) ([7eca5be](https://github.com/smartcontractkit/cld-changesets/commit/7eca5be53c91b172fad206da2e0c72368ed42754))
* port token approve ([#40](https://github.com/smartcontractkit/cld-changesets/issues/40)) ([04130de](https://github.com/smartcontractkit/cld-changesets/commit/04130dea1cef245363491be955fcbd9c7e88c880))
* **port:** firedrill mcms with operations api refactor ([#25](https://github.com/smartcontractkit/cld-changesets/issues/25)) ([6d9010c](https://github.com/smartcontractkit/cld-changesets/commit/6d9010c9fc9c19d636bf97fb855e3054f4c6a2b3))
* **solana:** add SOL funding helpers for deployer transfers ([#19](https://github.com/smartcontractkit/cld-changesets/issues/19)) ([63bebf8](https://github.com/smartcontractkit/cld-changesets/commit/63bebf8efcd6ac5eadffe223b0bd374eee25c996))
* **solana:** port over mcms pda loader ([#11](https://github.com/smartcontractkit/cld-changesets/issues/11)) ([7170ddc](https://github.com/smartcontractkit/cld-changesets/commit/7170ddc27f6189d08ecbb7ff7cd89ff43fb04049))


### Bug Fixes

* **aptos:** move state load ([#8](https://github.com/smartcontractkit/cld-changesets/issues/8)) ([de58102](https://github.com/smartcontractkit/cld-changesets/commit/de58102750753e89f31898a8ca0d9d7d2e1c9634))
* **evm:** port state load evm funcs ([#9](https://github.com/smartcontractkit/cld-changesets/issues/9)) ([99279f1](https://github.com/smartcontractkit/cld-changesets/commit/99279f19acc9f876322a6f98f235b01bf0370784))
* **state:** restore addressbook usage ([#17](https://github.com/smartcontractkit/cld-changesets/issues/17)) ([017cd5e](https://github.com/smartcontractkit/cld-changesets/commit/017cd5e63c12104cf6e54ad531e3782ecff8f38f))
