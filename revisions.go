package gcrunpresso

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
	"google.golang.org/api/iterator"
)

type RevisionsOption struct {
}

func (d *App) Revisions(ctx context.Context, opt RevisionsOption) error {
	d.LogInfo("listing revisions", "service", d.config.Service)

	if d.config.Service == "" {
		return fmt.Errorf("revisions command is only applicable to Cloud Run Services, but service is not configured")
	}

	// Fetch current service to know traffic splits
	svc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", d.ResourceServicePath(), err)
	}

	trafficMap := make(map[string]int32)
	tagMap := make(map[string]string)
	for _, t := range svc.Traffic {
		rev := t.Revision
		if rev == "" && t.Type == runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST {
			rev = svc.LatestReadyRevision
		}
		trafficMap[rev] += t.Percent
		if t.Tag != "" {
			tagMap[rev] = t.Tag
		}
	}

	it := d.revisionsClient.ListRevisions(ctx, &runpb.ListRevisionsRequest{
		Parent: d.ResourceServicePath(),
	})

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "REVISION\tTRAFFIC\tTAG\tIMAGE\tCREATED\tSTATUS")

	for {
		rev, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to iterate revisions: %w", err)
		}

		shortName := arnToName(rev.Name)
		pct := trafficMap[shortName]
		tag := tagMap[shortName]

		image := ""
		if len(rev.Containers) > 0 {
			image = rev.Containers[0].Image
		}

		createdAt := ""
		if rev.CreateTime != nil {
			createdAt = rev.CreateTime.AsTime().Format("2006-01-02 15:04:05")
		}

		statusStr := "[ ]"
		for _, c := range rev.Conditions {
			if c.Type == "Ready" {
				if c.State == runpb.Condition_CONDITION_SUCCEEDED {
					statusStr = green.Sprint("[READY]")
				} else if c.State == runpb.Condition_CONDITION_FAILED {
					statusStr = red.Sprint("[FAILED]")
				}
				break
			}
		}

		trafficStr := fmt.Sprintf("%d%%", pct)
		if pct > 0 {
			trafficStr = green.Sprintf("%d%%", pct)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", shortName, trafficStr, tag, image, createdAt, statusStr)
	}

	w.Flush()
	return nil
}
