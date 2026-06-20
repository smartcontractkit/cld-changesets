package evmdeploytopology

import (
	"encoding/json"
	"fmt"
)

// EVMExtraArgs carries EVM-specific topology input that the chain-agnostic
// changeset config cannot express. It is passed via ChainTopologyConfig.ExtraArgs
// and parsed by the EVM sequence.
type EVMExtraArgs struct {
	// DeployCallProxyByTimelockRef maps a timelock Ref to whether a CallProxy
	// should be deployed and granted the executor role for it. A timelock not
	// present in the map (or a nil map) defaults to deploying a call proxy.
	DeployCallProxyByTimelockRef map[string]bool `json:"deployCallProxyByTimelockRef,omitempty"`
}

// deployCallProxy reports whether a call proxy should be deployed for the given
// timelock ref. The default (no entry / nil map) is true.
func (e EVMExtraArgs) deployCallProxy(timelockRef string) bool {
	if e.DeployCallProxyByTimelockRef == nil {
		return true
	}
	if v, ok := e.DeployCallProxyByTimelockRef[timelockRef]; ok {
		return v
	}

	return true
}

// parseEVMExtraArgs extracts EVMExtraArgs from the chain-agnostic ExtraArgs field.
// It accepts a typed EVMExtraArgs (or pointer) directly, and otherwise falls back
// to a JSON round-trip so deserialized configs (map[string]any) are supported.
// A nil value yields the zero EVMExtraArgs (call proxies default on).
func parseEVMExtraArgs(v any) (EVMExtraArgs, error) {
	switch ea := v.(type) {
	case nil:
		return EVMExtraArgs{}, nil
	case EVMExtraArgs:
		return ea, nil
	case *EVMExtraArgs:
		if ea == nil {
			return EVMExtraArgs{}, nil
		}

		return *ea, nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return EVMExtraArgs{}, fmt.Errorf("failed to marshal evm extra args: %w", err)
	}
	var ea EVMExtraArgs
	if err := json.Unmarshal(data, &ea); err != nil {
		return EVMExtraArgs{}, fmt.Errorf("failed unmarshal evm extra args: %w", err)
	}

	return ea, nil
}
