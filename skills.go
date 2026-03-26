package ecspresso

import (
	"context"
	"fmt"

	"github.com/Songmu/skillsmith"
)

// SkillsOption defines CLI options for the skills subcommand.
type SkillsOption struct {
	List      *SkillsListOption   `cmd:"" help:"list available skills"`
	Install   *SkillsModifyOption `cmd:"" help:"install skills"`
	Update    *SkillsModifyOption `cmd:"" help:"update installed skills"`
	Reinstall *SkillsModifyOption `cmd:"" help:"reinstall all managed skills"`
	Uninstall *SkillsModifyOption `cmd:"" help:"uninstall managed skills"`
	Status    *SkillsStatusOption `cmd:"" help:"show installation status"`
}

// SkillsListOption defines CLI options for skills list.
type SkillsListOption struct{}

// SkillsModifyOption defines CLI options for skills install/update/reinstall/uninstall.
type SkillsModifyOption struct {
	Scope  string `help:"install scope (user or repo)" default:"" enum:",user,repo"`
	Prefix string `help:"override install directory" default:""`
	DryRun bool   `help:"preview changes without applying" name:"dry-run" default:"false"`
	Force  bool   `help:"overwrite unmanaged skills or force downgrade" default:"false"`
}

func (o *SkillsModifyOption) options() skillsmith.Options {
	return skillsmith.Options{
		Scope:  o.Scope,
		Prefix: o.Prefix,
		DryRun: o.DryRun,
		Force:  o.Force,
	}
}

// SkillsStatusOption defines CLI options for skills status.
type SkillsStatusOption struct {
	Scope  string `help:"install scope (user or repo)" default:"" enum:",user,repo"`
	Prefix string `help:"override install directory" default:""`
}

func (o *SkillsStatusOption) options() skillsmith.Options {
	return skillsmith.Options{
		Scope:  o.Scope,
		Prefix: o.Prefix,
	}
}

func dispatchSkills(ctx context.Context, opts *SkillsOption) error {
	switch {
	case opts.List != nil:
		return skillsList(ctx)
	case opts.Install != nil:
		return skillsInstall(ctx, opts.Install.options())
	case opts.Update != nil:
		return skillsUpdate(ctx, opts.Update.options())
	case opts.Reinstall != nil:
		return skillsReinstall(ctx, opts.Reinstall.options())
	case opts.Uninstall != nil:
		return skillsUninstall(ctx, opts.Uninstall.options())
	case opts.Status != nil:
		return skillsStatus(ctx, opts.Status.options())
	default:
		return fmt.Errorf("unknown skills subcommand")
	}
}

func newSmith() (*skillsmith.Smith, error) {
	version := Version
	if version == "" {
		version = "v0.0.0-dev"
	}
	return skillsmith.New("ecspresso", version, skillsFS)
}

func skillsList(ctx context.Context) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	skills, err := s.List(ctx)
	if err != nil {
		return err
	}
	if len(skills) == 0 {
		_, err := WriteOutput("no skills found")
		return err
	}
	for _, sk := range skills {
		if sk.Description != "" {
			fmt.Printf("%-30s %s\n", sk.Dir, sk.Description)
		} else {
			fmt.Println(sk.Dir)
		}
	}
	return nil
}

func skillsInstall(ctx context.Context, opts skillsmith.Options) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	result, err := s.Install(ctx, opts)
	if err != nil {
		return err
	}
	printCopyResult(result, "installed", opts.DryRun)
	return nil
}

func skillsUpdate(ctx context.Context, opts skillsmith.Options) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	result, err := s.Update(ctx, opts)
	if err != nil {
		return err
	}
	printCopyResult(result, "updated", opts.DryRun)
	return nil
}

func skillsReinstall(ctx context.Context, opts skillsmith.Options) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	result, err := s.Reinstall(ctx, opts)
	if err != nil {
		return err
	}
	printCopyResult(result, "reinstalled", opts.DryRun)
	return nil
}

func skillsUninstall(ctx context.Context, opts skillsmith.Options) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	result, err := s.Uninstall(ctx, opts)
	if err != nil {
		return err
	}
	for _, a := range result.Actions {
		switch a.Action {
		case "uninstalled":
			if opts.DryRun {
				fmt.Printf("uninstalled (dry-run): %s\n", a.Dir)
			} else {
				fmt.Printf("uninstalled: %s\n", a.Dir)
			}
		case "skipped":
			fmt.Printf("skipped:     %s — %s\n", a.Dir, a.Message)
		}
	}
	if opts.DryRun {
		fmt.Println("[dry-run] no changes were made")
	}
	return nil
}

func skillsStatus(ctx context.Context, opts skillsmith.Options) error {
	s, err := newSmith()
	if err != nil {
		return err
	}
	result, err := s.Status(ctx, opts)
	if err != nil {
		return err
	}
	for _, ss := range result.Skills {
		switch {
		case !ss.Installed:
			fmt.Printf("%-30s not installed\n", ss.Dir)
		case ss.MetadataError != nil:
			fmt.Printf("%-30s installed (metadata unreadable: %v)\n", ss.Dir, ss.MetadataError)
		case ss.UpToDate:
			fmt.Printf("%-30s installed %s (up to date)\n", ss.Dir, ss.InstalledVersion)
		default:
			fmt.Printf("%-30s installed %s → available %s\n", ss.Dir, ss.InstalledVersion, ss.AvailableVersion)
		}
	}
	return nil
}

func printCopyResult(result *skillsmith.CopyResult, verb string, dryRun bool) {
	for _, a := range result.Actions {
		switch a.Action {
		case verb:
			if dryRun {
				fmt.Printf("%s (dry-run): %s\n", verb, a.Dir)
			} else {
				fmt.Printf("%s: %s\n", verb, a.Dir)
			}
		case "skipped":
			fmt.Printf("skipped:   %s — %s\n", a.Dir, a.Message)
		case "warned":
			LogWarn("warning:   %s — %s", a.Dir, a.Message)
		}
	}
	if dryRun {
		fmt.Println("[dry-run] no changes were made")
	}
}
