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

type ExecutionsOption struct {
}

func (d *App) Executions(ctx context.Context, opt ExecutionsOption) error {
	d.LogInfo("listing executions", "job", d.config.Job)

	if d.config.Job == "" {
		return fmt.Errorf("executions command is only applicable to Cloud Run Jobs, but job is not configured")
	}

	it := d.executionsClient.ListExecutions(ctx, &runpb.ListExecutionsRequest{
		Parent: d.ResourceJobPath(),
	})

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "EXECUTION\tSUCCEEDED\tFAILED\tRUNNING\tSTART TIME\tCOMPLETION TIME\tSTATUS")

	for {
		exec, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to iterate executions: %w", err)
		}

		shortName := arnToName(exec.Name)
		startTime := ""
		if exec.StartTime != nil {
			startTime = exec.StartTime.AsTime().Format("2006-01-02 15:04:05")
		}
		completionTime := ""
		if exec.CompletionTime != nil {
			completionTime = exec.CompletionTime.AsTime().Format("2006-01-02 15:04:05")
		}

		statusStr := "[ ]"
		if exec.Reconciling {
			statusStr = yellow.Sprint("[RUNNING]")
		} else {
			for _, c := range exec.Conditions {
				if c.Type == "Completed" || c.Type == "Ready" {
					if c.State == runpb.Condition_CONDITION_SUCCEEDED {
						statusStr = green.Sprint("[SUCCEEDED]")
					} else if c.State == runpb.Condition_CONDITION_FAILED {
						statusStr = red.Sprint("[FAILED]")
					}
					break
				}
			}
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
			shortName,
			exec.SucceededCount,
			exec.FailedCount,
			exec.RunningCount,
			startTime,
			completionTime,
			statusStr,
		)
	}

	w.Flush()
	return nil
}
