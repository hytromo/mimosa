package docker

import (
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func Get(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error) {
	return remote.Get(ref, remote.WithAuthFromKeychain(Keychain))
}

func WriteIndex(ref name.Reference, ii v1.ImageIndex, options ...remote.Option) (rerr error) {
	return remote.WriteIndex(ref, ii, remote.WithAuthFromKeychain(Keychain))
}
