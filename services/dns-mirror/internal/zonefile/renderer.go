package zonefile

import (
	"bytes"
	"fmt"
	"hash/crc32"
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
	recordSets := sortedRecordSets(zone.RecordSets)
	serial, err := contentSerial(recordSets)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	for _, recordSet := range recordSets {
		values := append([]string(nil), recordSet.Values...)
		slices.Sort(values)

		for _, value := range values {
			record, err := parseRecord(recordSet, value)
			if err != nil {
				return nil, err
			}

			if soa, ok := record.(*dns.SOA); ok {
				soa.Serial = serial
			}

			buffer.WriteString(record.String())
			buffer.WriteByte('\n')
		}
	}

	return buffer.Bytes(), nil
}

func sortedRecordSets(recordSets []mirror.RecordSet) []mirror.RecordSet {
	sorted := append([]mirror.RecordSet(nil), recordSets...)
	slices.SortFunc(sorted, func(left mirror.RecordSet, right mirror.RecordSet) int {
		if compare := strings.Compare(left.Name, right.Name); compare != 0 {
			return compare
		}

		if compare := strings.Compare(left.Type, right.Type); compare != 0 {
			return compare
		}

		return compareValues(left.Values, right.Values)
	})

	return sorted
}

// contentSerial gives CoreDNS' file plugin a stable reload signal when Route 53
// leaves the source SOA serial unchanged across record edits.
func contentSerial(recordSets []mirror.RecordSet) (uint32, error) {
	var buffer bytes.Buffer
	for _, recordSet := range recordSets {
		values := append([]string(nil), recordSet.Values...)
		slices.Sort(values)

		for _, value := range values {
			record, err := parseRecord(recordSet, value)
			if err != nil {
				return 0, err
			}

			if soa, ok := record.(*dns.SOA); ok {
				soa.Serial = 0
			}

			buffer.WriteString(record.String())
			buffer.WriteByte('\n')
		}
	}

	serial := crc32.ChecksumIEEE(buffer.Bytes())
	if serial == 0 {
		return 1, nil
	}

	return serial, nil
}

func parseRecord(recordSet mirror.RecordSet, value string) (dns.RR, error) {
	line := fmt.Sprintf("%s\t%d\tIN\t%s\t%s", dns.Fqdn(recordSet.Name), recordSet.TTL, recordSet.Type, value)
	record, err := dns.NewRR(line)
	if err != nil {
		return nil, fmt.Errorf("parse record %s %s: %w", recordSet.Name, recordSet.Type, err)
	}

	return record, nil
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
