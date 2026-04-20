package zonefile_test

import (
	"testing"

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

	assert.Equal(t, ""+
		"api.glab.lol.\t300\tIN\tA\t192.0.2.10\n"+
		"api.glab.lol.\t300\tIN\tA\t192.0.2.20\n"+
		"glab.lol.\t172800\tIN\tNS\tns-1.example.\n"+
		"glab.lol.\t172800\tIN\tNS\tns-2.example.\n"+
		"glab.lol.\t900\tIN\tSOA\tns-1.example. hostmaster.example. 1 7200 900 1209600 86400\n",
		string(content),
	)
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
