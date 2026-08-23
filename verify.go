package gcrunpresso

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"cloud.google.com/go/run/apiv2/runpb"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/fatih/color"
)

type VerifyOption struct {
	Image   bool `help:"verify container images existence" default:"true" negatable:""`
	Secrets bool `help:"verify secret manager secrets existence" default:"true" negatable:""`
	JSON    bool `help:"output verification results in JSON format" default:"false"`
}

type VerifyItemResult struct {
	Type    string `json:"type"`
	Target  string `json:"target"`
	Status  string `json:"status"` // OK, SKIP, FAIL
	Message string `json:"message,omitempty"`
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

	var results []VerifyItemResult
	var failedCount int

	if opt.Image && len(images) > 0 {
		for _, img := range images {
			err := d.verifyImage(ctx, img)
			if err != nil {
				if _, isSkip := err.(ErrSkipVerify); isSkip {
					results = append(results, VerifyItemResult{
						Type:    "image",
						Target:  img,
						Status:  "SKIP",
						Message: err.Error(),
					})
				} else {
					failedCount++
					results = append(results, VerifyItemResult{
						Type:    "image",
						Target:  img,
						Status:  "FAIL",
						Message: err.Error(),
					})
				}
			} else {
				results = append(results, VerifyItemResult{
					Type:   "image",
					Target: img,
					Status: "OK",
				})
			}
		}
	}

	if opt.Secrets && len(secrets) > 0 {
		for _, sec := range secrets {
			err := d.verifySecret(ctx, sec)
			if err != nil {
				if _, isSkip := err.(ErrSkipVerify); isSkip {
					results = append(results, VerifyItemResult{
						Type:    "secret",
						Target:  sec,
						Status:  "SKIP",
						Message: err.Error(),
					})
				} else {
					failedCount++
					results = append(results, VerifyItemResult{
						Type:    "secret",
						Target:  sec,
						Status:  "FAIL",
						Message: err.Error(),
					})
				}
			} else {
				results = append(results, VerifyItemResult{
					Type:   "secret",
					Target: sec,
					Status: "OK",
				})
			}
		}
	}

	if opt.JSON {
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	} else {
		green := color.New(color.FgGreen)
		yellow := color.New(color.FgYellow)
		red := color.New(color.FgRed)

		if opt.Image && len(images) > 0 {
			fmt.Println("Verifying Container Images:")
			for _, r := range results {
				if r.Type != "image" {
					continue
				}
				switch r.Status {
				case "OK":
					fmt.Printf("  %s %s\n", green.Sprint("[OK]"), r.Target)
				case "SKIP":
					fmt.Printf("  %s %s (%s)\n", yellow.Sprint("[SKIP]"), r.Target, r.Message)
				case "FAIL":
					fmt.Printf("  %s %s (%s)\n", red.Sprint("[FAIL]"), r.Target, r.Message)
				}
			}
		}

		if opt.Secrets && len(secrets) > 0 {
			fmt.Println("Verifying Secret Manager Secrets:")
			for _, r := range results {
				if r.Type != "secret" {
					continue
				}
				switch r.Status {
				case "OK":
					fmt.Printf("  %s %s\n", green.Sprint("[OK]"), r.Target)
				case "SKIP":
					fmt.Printf("  %s %s (%s)\n", yellow.Sprint("[SKIP]"), r.Target, r.Message)
				case "FAIL":
					fmt.Printf("  %s %s (%s)\n", red.Sprint("[FAIL]"), r.Target, r.Message)
				}
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

			if d.arClient != nil {
				repoPath := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", proj, loc, repo)

				// The repository must exist regardless of how the image is referenced.
				if _, repoErr := d.arClient.GetRepository(ctx, &artifactregistrypb.GetRepositoryRequest{
					Name: repoPath,
				}); repoErr != nil {
					if isPermissionError(repoErr) {
						return ErrSkipVerify(fmt.Sprintf("permission denied checking repository %s", repoPath))
					}
					return fmt.Errorf("artifact registry repository %s not found: %w", repoPath, repoErr)
				}

				// GetDockerImage addresses an image by digest. A digest reference is passed
				// through verbatim (its own colon is part of "sha256:..."); a tag cannot be
				// resolved by this call, so it is reported as unverified rather than as OK.
				if _, digest, ok := strings.Cut(pkgAndTag, "@"); ok && digest != "" {
					dockerImgPath := fmt.Sprintf("%s/dockerImages/%s", repoPath, pkgAndTag)
					_, err := d.arClient.GetDockerImage(ctx, &artifactregistrypb.GetDockerImageRequest{
						Name: dockerImgPath,
					})
					if err == nil {
						return nil
					}
					if isPermissionError(err) {
						return ErrSkipVerify(fmt.Sprintf("permission denied checking image %s", dockerImgPath))
					}
					if isNotFoundError(err) {
						return fmt.Errorf("container image %s not found in artifact registry", image)
					}
					return fmt.Errorf("failed to check container image %s: %w", image, err)
				}
				if idx := strings.LastIndex(pkgAndTag, ":"); idx != -1 {
					return ErrSkipVerify(fmt.Sprintf("repository %s exists; tag %q not verified (requires digest resolution)", repoPath, pkgAndTag[idx+1:]))
				}
			}
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
		if err != nil {
			if isPermissionError(err) {
				return ErrSkipVerify(fmt.Sprintf("permission denied accessing secret %s", secretPath))
			}
			return fmt.Errorf("secret %s not accessible: %w", secretPath, err)
		}
	}
	return nil
}

var _ = os.Stdout
