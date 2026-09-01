package stellarinternal

import (
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
)

func FundStellarSigner(t testing.TB, chain cldfstellar.Chain) {
	t.Helper()

	require.NotNil(t, chain.Signer)
	require.NotEmpty(t, chain.FriendbotURL)

	friendbotURL, err := url.Parse(chain.FriendbotURL)
	require.NoError(t, err)

	query := friendbotURL.Query()
	query.Set("addr", chain.Signer.Address())
	friendbotURL.RawQuery = query.Encode()

	client := &http.Client{Timeout: 10 * time.Second}

	var lastStatus int
	var lastBody string
	var lastErr error

	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			friendbotURL.String(),
			nil,
		)
		if err != nil {
			lastErr = err
			return false
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			return false
		}

		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()

		if readErr != nil {
			lastErr = readErr
			return false
		}
		if closeErr != nil {
			lastErr = closeErr
			return false
		}

		lastStatus = resp.StatusCode
		lastBody = string(body)
		lastErr = nil

		return resp.StatusCode == http.StatusOK
	}, 45*time.Second, time.Second,
		"friendbot failed to fund %s: status=%d body=%s err=%v",
		chain.Signer.Address(),
		lastStatus,
		lastBody,
		lastErr,
	)
}
