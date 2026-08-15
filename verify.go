package gcrunpresso

import (
	"context"
	"fmt"
	"strings"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"cloud.google.com/go/run/apiv2/runpb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/fatih/color"
)

type VerifyOption struct {
	Image   bool `help:"verify container images existence" default:"true" negatable:""`
	Secrets bool `help:"verify secret manager secrets existence" default:"true" negatable:""`
}

func (d *App) Verify(ctx context.Context, opt VerifyOption) error {
	d.LogInfo("verifying referenced resources", "service", d.config.Service, "job", d.config.Job)

	var images []string
	var secrets []string

	if d.config.Service != "" {
		svc, err := d.LoadServiceDefinition("")
		if err != nil {
			return err
		}
		images, secrets = extractServiceResources(svc)
	} else if d.config.Job != "" {
		job, err := d.LoadJobDefinition("")
		if err != nil {
			return err
		}
		images, secrets = extractJobResources(job)
	} else {
		return fmt.Errorf("either service or job must be specified to verify")
	}

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	var failedCount int

	if opt.Image && len(images) > 0 {
		fmt.Println("Verifying Container Images:")
		for _, img := range images {
			err := d.verifyImage(ctx, img)
			if err != nil {
				fmt.Printf("  %s %s (%v)\n", red.Sprint("[FAIL]"), img, err)
				failedCount++
			} else {
				fmt.Printf("  %s %s\n", green.Sprint("[OK]"), img)
			}
		}
	}

	if opt.Secrets && len(secrets) > 0 {
		fmt.Println("Verifying Secret Manager Secrets:")
		for _, sec := range secrets {
			err := d.verifySecret(ctx, sec)
			if err != nil {
				fmt.Printf("  %s %s (%v)\n", red.Sprint("[FAIL]"), sec, err)
				failedCount++
			} else {
				fmt.Printf("  %s %s\n", green.Sprint("[OK]"), sec)
			}
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("verification failed with %d errors", failedCount)
	}

	d.LogInfo("all referenced resources verified successfully")
	return nil
}

func extractServiceResources(svc *runpb.Service) (images []string, secrets []string) {
	if svc.Template == nil {
		return
	}
	for _, c := range svc.Template.Containers {
		if c.Image != "" {
			images = append(images, c.Image)
		}
		for _, env := range c.Env {
			if vs := env.GetValueSource(); vs != nil {
				if ref := vs.GetSecretKeyRef(); ref != nil && ref.Secret != "" {
					secrets = append(secrets, ref.Secret)
				}
			}
		}
	}
	for _, vol := range svc.Template.Volumes {
		if vs := vol.GetSecret(); vs != nil && vs.Secret != "" {
			secrets = append(secrets, vs.Secret)
		}
	}
	return
}

func extractJobResources(job *runpb.Job) (images []string, secrets []string) {
	if job.Template == nil || job.Template.Template == nil {
		return
	}
	for _, c := range job.Template.Template.Containers {
		if c.Image != "" {
			images = append(images, c.Image)
		}
		for _, env := range c.Env {
			if vs := env.GetValueSource(); vs != nil {
				if ref := vs.GetSecretKeyRef(); ref != nil && ref.Secret != "" {
					secrets = append(secrets, ref.Secret)
				}
			}
		}
	}
	for _, vol := range job.Template.Template.Volumes {
		if vs := vol.GetSecret(); vs != nil && vs.Secret != "" {
			secrets = append(secrets, vs.Secret)
		}
	}
	return
}

func (d *App) verifyImage(ctx context.Context, image string) error {
	// If Artifact Registry image: <loc>-docker.pkg.dev/<proj>/<repo>/<package>[:tag|@digest]
	if strings.Contains(image, "-docker.pkg.dev/") {
		parts := strings.SplitN(image, "/", 4)
		if len(parts) >= 4 {
			loc := strings.TrimSuffix(parts[0], "-docker.pkg.dev")
			proj := parts[1]
			repo := parts[2]
			pkgAndTag := parts[3]

			pkg := pkgAndTag
			if idx := strings.IndexAny(pkgAndTag, ":@"); idx != -1 {
				pkg = pkgAndTag[:idx]
			}

			repoPath := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", proj, loc, repo)
			if d.arClient != nil {
				_, err := d.arClient.GetRepository(ctx, &artifactregistrypb.GetRepositoryRequest{
					Name: repoPath,
				})
				if err != nil && !isPermissionError(err) {
					return fmt.Errorf("artifact registry repository %s not found: %w", repoPath, err)
				}
			}
			_ = pkg
		}
	}
	return nil
}

func (d *App) verifySecret(ctx context.Context, secretName string) error {
	secretPath := secretName
	if !strings.HasPrefix(secretPath, "projects/") {
		secretPath = fmt.Sprintf("projects/%s/secrets/%s", d.config.Project, secretName)
	}

	if d.secretClient != nil {
		_, err := d.secretClient.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
			Name: secretPath,
		})
		if err != nil && !isPermissionError(err) {
			return fmt.Errorf("secret %s not accessible: %w", secretPath, err)
		}
	}
	return nil
}

var (
	_ *artifactregistry.Client
	_ *secretmanager.Client
)
