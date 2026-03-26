package ecspresso

import (
	"context"

	"github.com/Songmu/skillsmith"
	"github.com/kayac/ecspresso/v2/skillscmd"
)

func newSmith() (*skillsmith.Smith, error) {
	version := Version
	if version == "" {
		version = "v0.0.0-dev"
	}
	return skillsmith.New("ecspresso", version, skillsFS)
}

func dispatchSkills(ctx context.Context, opts *skillscmd.Commands) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	return opts.Run(ctx, s)
}
