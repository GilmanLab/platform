package zonefile

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/miekg/dns"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

// Renderer converts normalized zones into deterministic zonefiles.
type Renderer struct{}

// NewRenderer constructs a Renderer.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Render renders the supplied zone into a deterministic zonefile.
func (r *Renderer) Render(zone mirror.Zone) ([]byte, error) {
	recordSets := append([]mirror.RecordSet(nil), zone.RecordSets...)
	slices.SortFunc(recordSets, func(left mirror.RecordSet, right mirror.RecordSet) int {
		if compare := strings.Compare(left.Name, right.Name); compare != 0 {
			return compare
		}

		if compare := strings.Compare(left.Type, right.Type); compare != 0 {
			return compare
		}

		return compareValues(left.Values, right.Values)
	})

	var buffer bytes.Buffer
	for _, recordSet := range recordSets {
		values := append([]string(nil), recordSet.Values...)
		slices.Sort(values)

		for _, value := range values {
			line := fmt.Sprintf("%s\t%d\tIN\t%s\t%s", dns.Fqdn(recordSet.Name), recordSet.TTL, recordSet.Type, value)
			record, err := dns.NewRR(line)
			if err != nil {
				return nil, fmt.Errorf("parse record %s %s: %w", recordSet.Name, recordSet.Type, err)
			}

			buffer.WriteString(record.String())
			buffer.WriteByte('\n')
		}
	}

	return buffer.Bytes(), nil
}

func compareValues(left []string, right []string) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}

	for index := range limit {
		if compare := strings.Compare(left[index], right[index]); compare != 0 {
			return compare
		}
	}

	switch {
	case len(left) < len(right):
		return -1
	case len(left) > len(right):
		return 1
	default:
		return 0
	}
}
