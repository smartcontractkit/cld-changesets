package stellarsetconfig

import (
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
)

func TestSetStellarMCMConfig_Validation(t *testing.T) {
	t.Parallel()

	validConfig := cldftesthelpers.SingleGroupMCMS(t)
	invalidConfig := validConfig
	invalidConfig.Quorum = 0

	tests := []struct {
		name    string
		input   OpStellarSetConfigInput
		wantErr string
	}{
		{
			name: "empty target address",
			input: OpStellarSetConfigInput{
				Target: MCMSetConfigTarget{
					Config:       validConfig,
					ContractType: "ProposerManyChainMultiSig",
				},
			},
			wantErr: "stellar set-config: target address is empty",
		},
		{
			name: "empty contract type",
			input: OpStellarSetConfigInput{
				Target: MCMSetConfigTarget{
					Address: testStellarContractID(t, 1),
					Config:  validConfig,
				},
			},
			wantErr: "stellar set-config: contract type is empty",
		},
		{
			name: "invalid config",
			input: OpStellarSetConfigInput{
				Target: MCMSetConfigTarget{
					Address:      testStellarContractID(t, 2),
					Config:       invalidConfig,
					ContractType: "ProposerManyChainMultiSig",
				},
			},
			wantErr: "stellar set-config: invalid config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := setStellarMCMConfig(
				optest.NewBundle(t),
				cldfstellar.Chain{
					ChainMetadata: cldfstellar.ChainMetadata{Selector: chainselectors.STELLAR_LOCALNET.Selector},
				},
				tt.input,
			)

			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
