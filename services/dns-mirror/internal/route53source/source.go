package route53source

import (
	"context"
	"fmt"
	"slices"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/miekg/dns"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

type api interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

// Source loads and normalizes Route 53 hosted-zone data.
type Source struct {
	client api
}

// New constructs a Source using the default AWS SDK credential chain.
func New(ctx context.Context, region string) (*Source, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return &Source{
		client: route53.NewFromConfig(cfg),
	}, nil
}

// LoadZone loads and normalizes all supported record sets in the hosted zone.
func (s *Source) LoadZone(ctx context.Context, hostedZoneID string) (mirror.Zone, error) {
	hostedZone, err := s.client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: &hostedZoneID,
	})
	if err != nil {
		return mirror.Zone{}, fmt.Errorf("get hosted zone: %w", err)
	}

	var recordSets []mirror.RecordSet
	var nextName *string
	var nextType route53types.RRType
	var nextIdentifier *string

	for {
		output, err := s.client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
			HostedZoneId:          &hostedZoneID,
			StartRecordName:       nextName,
			StartRecordType:       nextType,
			StartRecordIdentifier: nextIdentifier,
		})
		if err != nil {
			return mirror.Zone{}, fmt.Errorf("list resource record sets: %w", err)
		}

		for _, recordSet := range output.ResourceRecordSets {
			normalizedRecordSet, err := normalizeRecordSet(recordSet)
			if err != nil {
				return mirror.Zone{}, err
			}

			recordSets = append(recordSets, normalizedRecordSet)
		}

		if !output.IsTruncated {
			break
		}

		nextName = output.NextRecordName
		nextType = output.NextRecordType
		nextIdentifier = output.NextRecordIdentifier
	}

	slices.SortFunc(recordSets, func(left mirror.RecordSet, right mirror.RecordSet) int {
		if compare := strings.Compare(left.Name, right.Name); compare != 0 {
			return compare
		}

		return strings.Compare(left.Type, right.Type)
	})

	return mirror.Zone{
		Name:       dns.Fqdn(*hostedZone.HostedZone.Name),
		RecordSets: recordSets,
	}, nil
}

func normalizeRecordSet(recordSet route53types.ResourceRecordSet) (mirror.RecordSet, error) {
	if recordSet.AliasTarget != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "alias targets")
	}

	if recordSet.CidrRoutingConfig != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "CIDR routing policies")
	}

	if recordSet.GeoLocation != nil || recordSet.GeoProximityLocation != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "geo routing policies")
	}

	if recordSet.HealthCheckId != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "health checks")
	}

	if recordSet.MultiValueAnswer != nil && *recordSet.MultiValueAnswer {
		return mirror.RecordSet{}, unsupported(recordSet, "multi-value answers")
	}

	if recordSet.Region != "" {
		return mirror.RecordSet{}, unsupported(recordSet, "latency routing policies")
	}

	if recordSet.SetIdentifier != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "set identifiers")
	}

	if recordSet.TrafficPolicyInstanceId != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "traffic policy instances")
	}

	if recordSet.Weight != nil {
		return mirror.RecordSet{}, unsupported(recordSet, "weighted routing policies")
	}

	if len(recordSet.ResourceRecords) == 0 {
		return mirror.RecordSet{}, unsupported(recordSet, "empty resource record sets")
	}

	values := make([]string, 0, len(recordSet.ResourceRecords))
	for _, resourceRecord := range recordSet.ResourceRecords {
		values = append(values, *resourceRecord.Value)
	}

	slices.Sort(values)

	return mirror.RecordSet{
		Name:   dns.Fqdn(*recordSet.Name),
		Type:   string(recordSet.Type),
		TTL:    *recordSet.TTL,
		Values: values,
	}, nil
}

func unsupported(recordSet route53types.ResourceRecordSet, reason string) error {
	return fmt.Errorf("unsupported Route 53 record set %s %s: %s", dns.Fqdn(*recordSet.Name), string(recordSet.Type), reason)
}
