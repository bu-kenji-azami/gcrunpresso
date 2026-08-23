package gcrunpresso

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
)

type RevisionsOption struct {
	JSON bool `help:"output revisions in JSON format" default:"false"`
}

type RevisionItem struct {
	Name      string `json:"name"`
	Traffic   int32  `json:"traffic_percent"`
	Tag       string `json:"tag,omitempty"`
	Image     string `json:"image,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Status    string `json:"status"`
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
	for _, ts := range svc.TrafficStatuses {
		if ts != nil && ts.Revision != "" {
			trafficMap[ts.Revision] += ts.Percent
			if ts.Tag != "" {
				tagMap[ts.Revision] = ts.Tag
			}
		}
	}
	if len(trafficMap) == 0 {
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
	}

	revisions, err := d.revisionsClient.ListRevisions(ctx, &runpb.ListRevisionsRequest{
		Parent: d.ResourceServicePath(),
	})
	if err != nil {
		return fmt.Errorf("failed to list revisions: %w", err)
	}

	var items []RevisionItem
	for _, rev := range revisions {
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

		statusStr := "UNKNOWN"
		for _, c := range rev.Conditions {
			if c.Type == "Ready" {
				if c.State == runpb.Condition_CONDITION_SUCCEEDED {
					statusStr = "READY"
				} else if c.State == runpb.Condition_CONDITION_FAILED {
					statusStr = "FAILED"
				}
				break
			}
		}

		items = append(items, RevisionItem{
			Name:      shortName,
			Traffic:   pct,
			Tag:       tag,
			Image:     image,
			CreatedAt: createdAt,
			Status:    statusStr,
		})
	}

	if opt.JSON {
		return printJSON(items)
	}

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "REVISION\tTRAFFIC\tTAG\tIMAGE\tCREATED\tSTATUS")

	for _, item := range items {
		var statusDisp string
		switch item.Status {
		case "READY":
			statusDisp = green.Sprint("[READY]")
		case "FAILED":
			statusDisp = red.Sprint("[FAILED]")
		default:
			statusDisp = "[ ]"
		}

		trafficStr := fmt.Sprintf("%d%%", item.Traffic)
		if item.Traffic > 0 {
			trafficStr = green.Sprintf("%d%%", item.Traffic)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", item.Name, trafficStr, item.Tag, item.Image, item.CreatedAt, statusDisp)
	}

	w.Flush()
	return nil
}
