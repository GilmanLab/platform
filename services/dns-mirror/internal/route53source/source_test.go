package route53source

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRecordSet(t *testing.T) {
	recordSet, err := normalizeRecordSet(types.ResourceRecordSet{
		Name: strPtr("api.glab.lol."),
		Type: types.RRTypeA,
		TTL:  int64Ptr(300),
		ResourceRecords: []types.ResourceRecord{
			{Value: strPtr("192.0.2.20")},
			{Value: strPtr("192.0.2.10")},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "api.glab.lol.", recordSet.Name)
	assert.Equal(t, "A", recordSet.Type)
	assert.Equal(t, int64(300), recordSet.TTL)
	assert.Equal(t, []string{"192.0.2.10", "192.0.2.20"}, recordSet.Values)
}

func TestNormalizeRecordSetRejectsAliasTargets(t *testing.T) {
	_, err := normalizeRecordSet(types.ResourceRecordSet{
		Name:        strPtr("glab.lol."),
		Type:        types.RRTypeA,
		AliasTarget: &types.AliasTarget{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alias targets")
}

func int64Ptr(value int64) *int64 {
	return &value
}

func strPtr(value string) *string {
	return &value
}
