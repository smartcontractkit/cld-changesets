package operations

import (
	"context"
	"fmt"

	nodev1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/node"
	"github.com/smartcontractkit/chainlink-protos/job-distributor/v1/shared/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	foffchain "github.com/smartcontractkit/chainlink-deployments-framework/offchain"
)

// JDOpDeps is the shared dependency surface for all JD operations and sequences.
type JDOpDeps struct {
	Offchain foffchain.Client
	EnvName  string
}

func labelsFromMap(m map[string]string) []*ptypes.Label {
	labels := make([]*ptypes.Label, 0, len(m))
	for k, v := range m {
		labels = append(labels, &ptypes.Label{Key: k, Value: &v})
	}

	return labels
}

func isJDNotFound(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.NotFound
}

// ListNodeByPublicKey returns the node whose public key matches csaKey,
// or (nil, nil).
func ListNodeByPublicKey(ctx context.Context, svc nodev1.NodeServiceClient, csaKey string) (*nodev1.Node, error) {
	resp, err := svc.ListNodes(ctx, &nodev1.ListNodesRequest{
		Filter: &nodev1.ListNodesRequest_Filter{PublicKeys: []string{csaKey}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	for _, n := range resp.Nodes {
		if n.GetPublicKey() == csaKey {
			return n, nil
		}
	}

	return nil, nil //nolint:nilnil // nil means "not found".
}
