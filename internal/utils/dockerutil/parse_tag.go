package dockerutil

import (
	"errors"

	"github.com/google/go-containerregistry/pkg/name"
)

type ParsedTag struct {
	Ref       name.Reference
	Registry  string
	Tag       string
	ImageName string
}

func ParseTag(fromTag string) (ParsedTag, error) {
	if t, err := name.NewTag(fromTag); err == nil {
		return ParsedTag{Ref: t, Registry: t.Context().Registry.Name(), Tag: t.TagStr(), ImageName: t.Context().RepositoryStr()}, nil
	}
	if d, err := name.NewDigest(fromTag); err == nil {
		return ParsedTag{Ref: d, Registry: d.Context().Registry.Name(), Tag: d.DigestStr(), ImageName: d.Context().RepositoryStr()}, nil
	}

	return ParsedTag{}, errors.New("invalid image reference")
}
