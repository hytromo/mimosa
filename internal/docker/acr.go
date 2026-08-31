package docker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/docker/docker-credential-helpers/credentials"
)

// acrRE matches Azure Container Registry hostnames. Anchored at both ends so
// that attacker-controlled hostnames such as "evil.azurecr.io.attacker.com"
// are never mistaken for a real ACR registry (see GO-2026-6225).
var acrRE = regexp.MustCompile(`^.*\.azurecr\.io$|^.*\.azurecr\.cn$|^.*\.azurecr\.de$|^.*\.azurecr\.us$`)

const (
	mcrHostname   = "mcr.microsoft.com"
	tokenUsername = "<token>"
	// acrAudience is the first-party AAD audience for Azure Container Registry.
	// Matches azcontainerregistry's defaultAudience.
	acrAudience = "https://containerregistry.azure.net"
	acrTimeout  = 30 * time.Second
)

// ACRCredHelper exchanges an Azure AD token for an Azure Container Registry
// refresh token, enabling docker-style pushes/pulls to ACR registries.
type ACRCredHelper struct{}

func (ACRCredHelper) Add(*credentials.Credentials) error {
	return errors.New("list is unimplemented")
}

func (ACRCredHelper) Delete(string) error {
	return errors.New("list is unimplemented")
}

func (ACRCredHelper) List() (map[string]string, error) {
	return nil, errors.New("list is unimplemented")
}

func (ACRCredHelper) Get(serverURL string) (string, string, error) {
	if !isACRRegistry(serverURL) {
		return "", "", errors.New("serverURL does not refer to Azure Container Registry")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to obtain Azure credential: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), acrTimeout)
	defer cancel()

	accessToken, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{acrAudience + "/.default"},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire Azure AD access token: %w", err)
	}

	client, err := azcontainerregistry.NewAuthenticationClient("https://"+serverURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create ACR authentication client: %w", err)
	}

	resp, err := client.ExchangeAADAccessTokenForACRRefreshToken(ctx,
		azcontainerregistry.PostContentSchemaGrantTypeAccessToken,
		serverURL,
		&azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions{
			AccessToken: &accessToken.Token,
		})
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire ACR refresh token: %w", err)
	}
	if resp.RefreshToken == nil {
		return "", "", fmt.Errorf("no ACR refresh token returned for %s", serverURL)
	}

	return tokenUsername, *resp.RefreshToken, nil
}

func isACRRegistry(input string) bool {
	serverURL, err := url.Parse("https://" + input)
	if err != nil {
		return false
	}
	if serverURL.Hostname() == mcrHostname {
		return true
	}
	return acrRE.MatchString(serverURL.Hostname())
}
