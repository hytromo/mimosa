package docker

import (
	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	ecrapi "github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	acr "github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	cntauthn "github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
)

var Keychain = cntauthn.NewMultiKeychain(
	cntauthn.DefaultKeychain,
	google.Keychain,
	cntauthn.NewKeychainFromHelper(ecr.NewECRHelper(
		ecr.WithClientFactory(ecrapi.DefaultClientFactory{}),
	)),
	cntauthn.NewKeychainFromHelper(acr.ACRCredHelper{}),
)
