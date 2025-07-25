package docker

import (
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	log "github.com/sirupsen/logrus"
)

func Get(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error) {
	return remote.Get(ref, remote.WithAuthFromKeychain(Keychain))
}

func SimpleRetag(source, target string) error {
	srcRef, err := name.ParseReference(source)
	if err != nil {
		log.Debugln("Failed to parse source reference:", err)
		return err
	}

	dstRef, err := name.ParseReference(target)
	if err != nil {
		log.Debugln("Failed to parse destination reference:", err)
		return err
	}

	// Get the image from the source tag
	img, err := remote.Image(srcRef, remote.WithAuthFromKeychain(Keychain))
	if err != nil {
		log.Debugln("Failed to get image from source reference:", err)
		return err
	}

	// Write the same image to the new tag
	if err := remote.Write(dstRef, img, remote.WithAuthFromKeychain(Keychain)); err != nil {
		log.Debugln("Failed to write image to new tag:", err)
		return err
	}

	return nil
}

func WriteIndex(ref name.Reference, ii v1.ImageIndex, options ...remote.Option) (rerr error) {
	return remote.WriteIndex(ref, ii, remote.WithAuthFromKeychain(Keychain))
}
