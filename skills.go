package ecspresso

import (
	"context"

	"github.com/Songmu/skillsmith"
)

// Skills runs the skillsmith subcommand for managing agent skills.
func Skills(ctx context.Context, args []string) error {
	version := Version
	if version == "" {
		version = "v0.0.0-dev"
	}
	s, err := skillsmith.New("ecspresso", version, skillsFS)
	if err != nil {
		return err
	}
	return s.Run(ctx, args)
}
