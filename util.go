package gcrunpresso

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
)

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
