package cmd

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/client"
	"github.com/intsig-textin/xparse-skills/cli/internal/config"
)

// Type aliases — re-export from internal/client so existing parse code compiles unchanged.
type (
	APIMode       = client.APIMode
	XParserClient = client.Client
	ParseOptions  = client.ParseOptions
	ParseResponse = client.ParseResponse
	ParseData     = client.ParseData
	ParseMetadata = client.ParseMetadata
	Summary       = client.Summary
)

const (
	APIModeAuto APIMode = client.APIModeAuto
	APIModeFree APIMode = client.APIModeFree
	APIModePaid APIMode = client.APIModePaid

	paidParseAPIPath = client.PaidParseAPIPath
	freeParseAPIPath = client.FreeParseAPIPath
)

// resolveAPIMode determines whether to use free or paid API.
func resolveAPIMode(mode APIMode, cred *config.CredentialSource) bool {
	return client.ResolveAPIMode(mode, cred)
}

// newXParserClient creates a client configured for free or paid API.
func newXParserClient(cmd *cobra.Command, cred *config.CredentialSource, bearerToken string, isFree bool) *XParserClient {
	var httpClient *http.Client
	if verboseFlag {
		httpClient = newVerboseHTTPClient()
	}
	return client.NewClientWithBearer(cmd, cred, bearerToken, isFree, httpClient)
}
