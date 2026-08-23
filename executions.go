package gcrunpresso

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
)

type ExecutionsOption struct {
	JSON bool `help:"output executions in JSON format" default:"false"`
}

type ExecutionItem struct {
	Name           string `json:"name"`
	SucceededCount int32  `json:"succeeded_count"`
	FailedCount    int32  `json:"failed_count"`
	RunningCount   int32  `json:"running_count"`
	StartTime      string `json:"start_time,omitempty"`
	CompletionTime string `json:"completion_time,omitempty"`
	Status         string `json:"status"`
}

func (d *App) Executions(ctx context.Context, opt ExecutionsOption) error {
	d.LogInfo("listing executions", "job", d.config.Job)

	if d.config.Job == "" {
		return fmt.Errorf("executions command is only applicable to Cloud Run Jobs, but job is not configured")
	}

	executions, err := d.executionsClient.ListExecutions(ctx, &runpb.ListExecutionsRequest{
		Parent: d.ResourceJobPath(),
	})
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	var items []ExecutionItem
	for _, exec := range executions {
		shortName := arnToName(exec.Name)
		startTime := ""
		if exec.StartTime != nil {
			startTime = exec.StartTime.AsTime().Format("2006-01-02 15:04:05")
		}
		completionTime := ""
		if exec.CompletionTime != nil {
			completionTime = exec.CompletionTime.AsTime().Format("2006-01-02 15:04:05")
		}

		statusStr := "UNKNOWN"
		if exec.Reconciling {
			statusStr = "RUNNING"
		} else {
			for _, c := range exec.Conditions {
				if c.Type == "Completed" || c.Type == "Ready" {
					if c.State == runpb.Condition_CONDITION_SUCCEEDED {
						statusStr = "SUCCEEDED"
					} else if c.State == runpb.Condition_CONDITION_FAILED {
						statusStr = "FAILED"
					}
					break
				}
			}
		}

		items = append(items, ExecutionItem{
			Name:           shortName,
			SucceededCount: exec.SucceededCount,
			FailedCount:    exec.FailedCount,
			RunningCount:   exec.RunningCount,
			StartTime:      startTime,
			CompletionTime: completionTime,
			Status:         statusStr,
		})
	}

	if opt.JSON {
		b, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "EXECUTION\tSUCCEEDED\tFAILED\tRUNNING\tSTART TIME\tCOMPLETION TIME\tSTATUS")

	for _, item := range items {
		var statusDisp string
		switch item.Status {
		case "RUNNING":
			statusDisp = yellow.Sprint("[RUNNING]")
		case "SUCCEEDED":
			statusDisp = green.Sprint("[SUCCEEDED]")
		case "FAILED":
			statusDisp = red.Sprint("[FAILED]")
		default:
			statusDisp = "[ ]"
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
			item.Name,
			item.SucceededCount,
			item.FailedCount,
			item.RunningCount,
			item.StartTime,
			item.CompletionTime,
			statusDisp,
		)
	}

	w.Flush()
	return nil
}
