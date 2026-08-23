package gcrunpresso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/samber/lo"
)

// FindServingRevisions extracts all revisions currently receiving traffic (Percent > 0)
// from rawSvc.TrafficStatuses. If TrafficStatuses is empty, falls back to rawSvc.LatestReadyRevision.
func FindServingRevisions(rawSvc *runpb.Service) []string {
	if rawSvc == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var revs []string

	for _, ts := range rawSvc.TrafficStatuses {
		if ts != nil && ts.Percent > 0 && ts.Revision != "" {
			if _, ok := seen[ts.Revision]; !ok {
				seen[ts.Revision] = struct{}{}
				revs = append(revs, ts.Revision)
			}
		}
	}

	if len(revs) == 0 && rawSvc.LatestReadyRevision != "" {
		revs = append(revs, rawSvc.LatestReadyRevision)
	}

	return revs
}

func parseLabels(s string) (map[string]string, error) {
	labels := make(map[string]string)
	if s == "" {
		return labels, nil
	}

	for pairStr := range strings.SplitSeq(s, ",") {
		if pairStr == "" {
			continue
		}
		pair := strings.SplitN(pairStr, "=", 2)
		if len(pair) != 2 {
			return labels, fmt.Errorf("invalid label format. Key=Value is required: %s", pairStr)
		}
		if len(pair[0]) == 0 {
			return labels, fmt.Errorf("label Key is required")
		}
		labels[pair[0]] = pair[1]
	}
	return labels, nil
}

func map2str(m map[string]string) string {
	var p []string
	keys := lo.Keys(m)
	sort.Strings(keys)
	for _, k := range keys {
		p = append(p, fmt.Sprintf("%s=%s", k, m[k]))
	}
	return strings.Join(p, ",")
}

func sleepContext(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

var defaultJSONWriter io.Writer = os.Stdout

func printJSON(v any) error {
	return printJSONTo(defaultJSONWriter, v)
}

func printJSONTo(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(b)); err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}
	return nil
}
