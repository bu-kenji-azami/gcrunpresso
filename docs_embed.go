package ecspresso

import (
	"embed"
	_ "embed"
)

//go:embed README.md
var readmeContent string

//go:embed skills
var skillsFS embed.FS
