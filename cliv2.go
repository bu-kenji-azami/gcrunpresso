package gcrunpresso

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/hashicorp/go-envparse"
)

func ParseCLIv2(args []string) (string, *CLIOptions, func(), error) {
	if len(args) == 0 || (len(args) > 0 && args[0] == "help") {
		args = []string{"--help"}
	}

	var opts CLIOptions
	parser, err := kong.New(&opts, kong.Vars{"version": Version})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to new kong: %w", err)
	}
	c, err := parser.Parse(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}
	sub := strings.Fields(c.Command())[0]

	for _, envFile := range opts.Envfile {
		if err := exportEnvFile(envFile); err != nil {
			return sub, &opts, nil, fmt.Errorf("failed to load envfile: %w", err)
		}
	}

	if opts.ExtStr == nil {
		opts.ExtStr = map[string]string{}
	}
	if opts.ExtCode == nil {
		opts.ExtCode = map[string]string{}
	}
	color.NoColor = !opts.Color
	return sub, &opts, func() { c.PrintUsage(true) }, nil
}

func exportEnvFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	envs, err := envparse.Parse(f)
	if err != nil {
		return err
	}
	for k, v := range envs {
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}
