package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type ACMClient struct {
	client *acm.Client
}

func NewACM(cfg aws.Config) *ACMClient {
	return &ACMClient{client: acm.NewFromConfig(cfg)}
}

type CertSummary struct {
	ARN        string
	DomainName string
	Status     string
	Type       string
}

type PageResult struct {
	NextToken *string
}

func (c *ACMClient) ListCertificates(ctx context.Context, nextToken *string, filter string) ([]CertSummary, *string, error) {
	ilog.Info("ACM: ListCertificates (token=%v filter=%q)", nextToken != nil, filter)
	if filter != "" {
		var items []CertSummary
		paginator := acm.NewListCertificatesPaginator(c.client, &acm.ListCertificatesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				ilog.Error("ACM: ListCertificates search failed: %v", err)
				return nil, nil, err
			}
			for _, cs := range page.CertificateSummaryList {
				domain := aws.ToString(cs.DomainName)
				if strings.Contains(strings.ToLower(domain), strings.ToLower(filter)) {
					items = append(items, CertSummary{
						ARN:        aws.ToString(cs.CertificateArn),
						DomainName: domain,
						Status:     string(cs.Status),
						Type:       string(cs.Type),
					})
				}
			}
		}
		ilog.Info("ACM: search returned %d items", len(items))
		return items, nil, nil
	}
	input := &acm.ListCertificatesInput{MaxItems: aws.Int32(50)}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	out, err := c.client.ListCertificates(ctx, input)
	if err != nil {
		ilog.Error("ACM: ListCertificates failed: %v", err)
		return nil, nil, err
	}
	var items []CertSummary
	for _, cs := range out.CertificateSummaryList {
		items = append(items, CertSummary{
			ARN:        aws.ToString(cs.CertificateArn),
			DomainName: aws.ToString(cs.DomainName),
			Status:     string(cs.Status),
			Type:       string(cs.Type),
		})
	}
	ilog.Info("ACM: ListCertificates returned %d items", len(items))
	return items, out.NextToken, nil
}

type CertDetail struct {
	ARN              string
	DomainName       string
	Status           string
	Type             string
	Issuer           string
	NotBefore        string
	NotAfter         string
	SubjectAltNames  []string
	InUseBy          []string
	Serial           string
	KeyAlgorithm     string
}

func (c *ACMClient) DescribeCertificate(ctx context.Context, arn string) (*CertDetail, error) {
	ilog.Info("ACM: DescribeCertificate %s", arn)
	out, err := c.client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		ilog.Error("ACM: DescribeCertificate failed: %v", err)
		return nil, err
	}
	cert := out.Certificate
	d := &CertDetail{
		ARN:             aws.ToString(cert.CertificateArn),
		DomainName:      aws.ToString(cert.DomainName),
		Status:          string(cert.Status),
		Type:            string(cert.Type),
		Issuer:          aws.ToString(cert.Issuer),
		Serial:          aws.ToString(cert.Serial),
		KeyAlgorithm:    string(cert.KeyAlgorithm),
		SubjectAltNames: cert.SubjectAlternativeNames,
	}
	if cert.NotBefore != nil {
		d.NotBefore = cert.NotBefore.Format("2006-01-02 15:04:05")
	}
	if cert.NotAfter != nil {
		d.NotAfter = cert.NotAfter.Format("2006-01-02 15:04:05")
	}
	for _, r := range cert.InUseBy {
		d.InUseBy = append(d.InUseBy, r)
	}
	return d, nil
}

func (d *CertDetail) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "ARN:          %s\n", d.ARN)
	fmt.Fprintf(&b, "Domain:       %s\n", d.DomainName)
	fmt.Fprintf(&b, "Status:       %s\n", d.Status)
	fmt.Fprintf(&b, "Type:         %s\n", d.Type)
	fmt.Fprintf(&b, "Issuer:       %s\n", d.Issuer)
	fmt.Fprintf(&b, "Algorithm:    %s\n", d.KeyAlgorithm)
	fmt.Fprintf(&b, "Serial:       %s\n", d.Serial)
	fmt.Fprintf(&b, "Not Before:   %s\n", d.NotBefore)
	fmt.Fprintf(&b, "Not After:    %s\n", d.NotAfter)
	if len(d.SubjectAltNames) > 0 {
		fmt.Fprintf(&b, "SANs:         %s\n", strings.Join(d.SubjectAltNames, ", "))
	}
	if len(d.InUseBy) > 0 {
		fmt.Fprintf(&b, "In Use By:\n")
		for _, r := range d.InUseBy {
			fmt.Fprintf(&b, "  - %s\n", r)
		}
	}
	return b.String()
}
