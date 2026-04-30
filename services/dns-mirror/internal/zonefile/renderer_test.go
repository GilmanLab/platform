package zonefile_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/zonefile"
)

func TestRenderDeterministicOrder(t *testing.T) {
	renderer := zonefile.NewRenderer()

	content, err := renderer.Render(mirror.Zone{
		Name: "glab.lol.",
		RecordSets: []mirror.RecordSet{
			{Name: "glab.lol.", Type: "SOA", TTL: 900, Values: []string{"ns-1.example. hostmaster.example. 1 7200 900 1209600 86400"}},
			{Name: "api.glab.lol.", Type: "A", TTL: 300, Values: []string{"192.0.2.20", "192.0.2.10"}},
			{Name: "glab.lol.", Type: "NS", TTL: 172800, Values: []string{"ns-2.example.", "ns-1.example."}},
		},
	})
	require.NoError(t, err)

	serial := requireSOASerial(t, content)
	assert.NotEqual(t, uint32(1), serial)
	assert.Equal(t, ""+
		"api.glab.lol.\t300\tIN\tA\t192.0.2.10\n"+
		"api.glab.lol.\t300\tIN\tA\t192.0.2.20\n"+
		"glab.lol.\t172800\tIN\tNS\tns-1.example.\n"+
		"glab.lol.\t172800\tIN\tNS\tns-2.example.\n"+
		fmt.Sprintf("glab.lol.\t900\tIN\tSOA\tns-1.example. hostmaster.example. %d 7200 900 1209600 86400\n", serial),
		string(content),
	)
}

func TestRenderDerivesSOASerialFromZoneContent(t *testing.T) {
	renderer := zonefile.NewRenderer()

	first, err := renderer.Render(testZone("1", "192.0.2.10"))
	require.NoError(t, err)
	second, err := renderer.Render(testZone("99", "192.0.2.10"))
	require.NoError(t, err)
	changed, err := renderer.Render(testZone("1", "192.0.2.11"))
	require.NoError(t, err)

	assert.Equal(t, requireSOASerial(t, first), requireSOASerial(t, second), "Route 53 SOA serial changes should not affect the rendered content serial")
	assert.NotEqual(t, requireSOASerial(t, first), requireSOASerial(t, changed), "record content changes should change the rendered SOA serial")
}

func TestRenderRejectsInvalidRecordValue(t *testing.T) {
	renderer := zonefile.NewRenderer()

	_, err := renderer.Render(mirror.Zone{
		Name: "glab.lol.",
		RecordSets: []mirror.RecordSet{
			{Name: "api.glab.lol.", Type: "A", TTL: 300, Values: []string{"not-an-ip"}},
		},
	})
	require.Error(t, err)
}

func testZone(route53Serial string, address string) mirror.Zone {
	return mirror.Zone{
		Name: "glab.lol.",
		RecordSets: []mirror.RecordSet{
			{Name: "glab.lol.", Type: "SOA", TTL: 900, Values: []string{fmt.Sprintf("ns-1.example. hostmaster.example. %s 7200 900 1209600 86400", route53Serial)}},
			{Name: "api.glab.lol.", Type: "A", TTL: 300, Values: []string{address}},
			{Name: "glab.lol.", Type: "NS", TTL: 172800, Values: []string{"ns-1.example."}},
		},
	}
}

func requireSOASerial(t *testing.T, content []byte) uint32 {
	t.Helper()

	parser := dns.NewZoneParser(strings.NewReader(string(content)), "glab.lol.", "test")
	for rr, ok := parser.Next(); ok; rr, ok = parser.Next() {
		if soa, ok := rr.(*dns.SOA); ok {
			return soa.Serial
		}
	}
	require.NoError(t, parser.Err())

	require.FailNow(t, "rendered zone did not contain an SOA record")
	return 0
}
