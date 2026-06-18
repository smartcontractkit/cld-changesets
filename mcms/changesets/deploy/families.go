package deploy

// RegisterFamilies registers one or more chain-family deploy implementations.
// Each family may only be registered once; duplicate registration panics.
//
// External teams call this once at startup from their own module:
//
//	deploy.RegisterFamilies(aptosimpl.Registration())
//
// Built-in families (EVM) register themselves automatically via their package
// init function when imported:
//
//	import _ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
func RegisterFamilies(regs ...Registration) {
	registerAll(regs...)
}
