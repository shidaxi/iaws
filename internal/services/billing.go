package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type BillingClient struct {
	client *costexplorer.Client
}

func NewBilling(cfg aws.Config) *BillingClient {
	ceCfg := cfg.Copy()
	ceCfg.Region = "us-east-1"
	return &BillingClient{client: costexplorer.NewFromConfig(ceCfg)}
}

type MonthlyCostItem struct {
	Month  string
	Amount string
	Unit   string
}

type ServiceCostItem struct {
	Service string
	Amount  string
	Unit    string
}

type DailyCostItem struct {
	Date   string
	Amount string
	Unit   string
}

type ResourceCostItem struct {
	ResourceID string
	Service    string
	Amount     float64
}

type RightsizingItem struct {
	InstanceID    string
	InstanceName  string
	CurrentType   string
	Action        string
	MonthlyCost   string
	SuggestedType string
	EstSavings    string
	Reasons       []string
}

func FmtDollar(amountStr string) string {
	f, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return "$" + amountStr
	}
	return fmt.Sprintf("$%.2f", f)
}

func fmtDollarF(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

// ─── Monthly Cost ──────────────────────────────────────────────────────────────

func (c *BillingClient) GetMonthlyCost(ctx context.Context, months int) ([]MonthlyCostItem, error) {
	ilog.Info("Billing: GetMonthlyCost months=%d", months)
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(now.Year(), now.Month()-time.Month(months-1), 1, 0, 0, 0, 0, time.UTC)

	out, err := c.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
	})
	if err != nil {
		return nil, err
	}

	var items []MonthlyCostItem
	for _, r := range out.ResultsByTime {
		amount := "0.00"
		unit := "USD"
		if m, ok := r.Total["UnblendedCost"]; ok {
			amount = aws.ToString(m.Amount)
			unit = aws.ToString(m.Unit)
		}
		items = append(items, MonthlyCostItem{
			Month:  aws.ToString(r.TimePeriod.Start),
			Amount: amount,
			Unit:   unit,
		})
	}
	return items, nil
}

// ─── Cost by Service ───────────────────────────────────────────────────────────

func (c *BillingClient) GetCostByService(ctx context.Context) ([]ServiceCostItem, error) {
	ilog.Info("Billing: GetCostByService")
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	out, err := c.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []cetypes.GroupDefinition{
			{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
		},
	})
	if err != nil {
		return nil, err
	}

	var items []ServiceCostItem
	for _, r := range out.ResultsByTime {
		for _, g := range r.Groups {
			svc := ""
			if len(g.Keys) > 0 {
				svc = g.Keys[0]
			}
			amount := "0.00"
			unit := "USD"
			if m, ok := g.Metrics["UnblendedCost"]; ok {
				amount = aws.ToString(m.Amount)
				unit = aws.ToString(m.Unit)
			}
			f, _ := strconv.ParseFloat(amount, 64)
			if f <= 0 {
				continue
			}
			items = append(items, ServiceCostItem{
				Service: svc,
				Amount:  amount,
				Unit:    unit,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		ai, _ := strconv.ParseFloat(items[i].Amount, 64)
		aj, _ := strconv.ParseFloat(items[j].Amount, 64)
		return ai > aj
	})
	return items, nil
}

// ─── Service Cost Detail (by usage type) ───────────────────────────────────────

func (c *BillingClient) GetServiceCostDetail(ctx context.Context, serviceName string, limit int) ([]ServiceCostItem, error) {
	ilog.Info("Billing: GetServiceCostDetail service=%q limit=%d", serviceName, limit)
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	out, err := c.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: cetypes.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		Filter: &cetypes.Expression{
			Dimensions: &cetypes.DimensionValues{
				Key:    cetypes.DimensionService,
				Values: []string{serviceName},
			},
		},
		GroupBy: []cetypes.GroupDefinition{
			{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("USAGE_TYPE")},
		},
	})
	if err != nil {
		return nil, err
	}

	var items []ServiceCostItem
	for _, r := range out.ResultsByTime {
		for _, g := range r.Groups {
			usageType := ""
			if len(g.Keys) > 0 {
				usageType = g.Keys[0]
			}
			amount := "0.00"
			unit := "USD"
			if m, ok := g.Metrics["UnblendedCost"]; ok {
				amount = aws.ToString(m.Amount)
				unit = aws.ToString(m.Unit)
			}
			f, _ := strconv.ParseFloat(amount, 64)
			if f <= 0 {
				continue
			}
			items = append(items, ServiceCostItem{
				Service: usageType,
				Amount:  amount,
				Unit:    unit,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		ai, _ := strconv.ParseFloat(items[i].Amount, 64)
		aj, _ := strconv.ParseFloat(items[j].Amount, 64)
		return ai > aj
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// ─── Daily Cost ────────────────────────────────────────────────────────────────

func (c *BillingClient) GetDailyCost(ctx context.Context, days int) ([]DailyCostItem, error) {
	ilog.Info("Billing: GetDailyCost days=%d", days)
	now := time.Now().UTC()
	end := now.AddDate(0, 0, 1)
	start := now.AddDate(0, 0, -days+1)

	out, err := c.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &cetypes.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: cetypes.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
	})
	if err != nil {
		return nil, err
	}

	var items []DailyCostItem
	for _, r := range out.ResultsByTime {
		amount := "0.00"
		unit := "USD"
		if m, ok := r.Total["UnblendedCost"]; ok {
			amount = aws.ToString(m.Amount)
			unit = aws.ToString(m.Unit)
		}
		items = append(items, DailyCostItem{
			Date:   aws.ToString(r.TimePeriod.Start),
			Amount: amount,
			Unit:   unit,
		})
	}
	return items, nil
}

// ─── Top Resources ─────────────────────────────────────────────────────────────

func (c *BillingClient) GetTopResources(ctx context.Context, limit int) ([]ResourceCostItem, error) {
	ilog.Info("Billing: GetTopResources limit=%d", limit)
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := now.Truncate(24 * time.Hour)
	if !end.After(start) {
		start = start.AddDate(0, -1, 0)
	}

	costMap := make(map[string]*ResourceCostItem)
	var nextToken *string

	for {
		input := &costexplorer.GetCostAndUsageWithResourcesInput{
			TimePeriod: &cetypes.DateInterval{
				Start: aws.String(start.Format("2006-01-02")),
				End:   aws.String(end.Format("2006-01-02")),
			},
			Granularity: cetypes.GranularityDaily,
			Metrics:     []string{"UnblendedCost"},
			Filter: &cetypes.Expression{
				Not: &cetypes.Expression{
					Dimensions: &cetypes.DimensionValues{
						Key:    cetypes.DimensionService,
						Values: []string{"Tax"},
					},
				},
			},
			GroupBy: []cetypes.GroupDefinition{
				{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
				{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("RESOURCE_ID")},
			},
		}
		if nextToken != nil {
			input.NextPageToken = nextToken
		}

		out, err := c.client.GetCostAndUsageWithResources(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, r := range out.ResultsByTime {
			for _, g := range r.Groups {
				service, resourceID := "", ""
				if len(g.Keys) > 0 {
					service = g.Keys[0]
				}
				if len(g.Keys) > 1 {
					resourceID = g.Keys[1]
				}
				if resourceID == "" {
					continue
				}
				amount := 0.0
				if m, ok := g.Metrics["UnblendedCost"]; ok {
					amount, _ = strconv.ParseFloat(aws.ToString(m.Amount), 64)
				}
				if amount <= 0 {
					continue
				}

				key := service + "|" + resourceID
				if existing, ok := costMap[key]; ok {
					existing.Amount += amount
				} else {
					costMap[key] = &ResourceCostItem{
						ResourceID: resourceID,
						Service:    service,
						Amount:     amount,
					}
				}
			}
		}

		nextToken = out.NextPageToken
		if nextToken == nil {
			break
		}
	}

	items := make([]ResourceCostItem, 0, len(costMap))
	for _, v := range costMap {
		items = append(items, *v)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Amount > items[j].Amount
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// ─── Rightsizing Recommendations ───────────────────────────────────────────────

func (c *BillingClient) GetRightsizingRecommendations(ctx context.Context) ([]RightsizingItem, error) {
	ilog.Info("Billing: GetRightsizingRecommendations")
	var items []RightsizingItem
	var nextToken *string

	for {
		input := &costexplorer.GetRightsizingRecommendationInput{
			Service: aws.String("AmazonEC2"),
		}
		if nextToken != nil {
			input.NextPageToken = nextToken
		}

		out, err := c.client.GetRightsizingRecommendation(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, rec := range out.RightsizingRecommendations {
			item := RightsizingItem{
				Action: string(rec.RightsizingType),
			}

			if rec.CurrentInstance != nil {
				item.InstanceID = aws.ToString(rec.CurrentInstance.ResourceId)
				item.InstanceName = aws.ToString(rec.CurrentInstance.InstanceName)
				item.MonthlyCost = aws.ToString(rec.CurrentInstance.MonthlyCost)
				if rec.CurrentInstance.ResourceDetails != nil &&
					rec.CurrentInstance.ResourceDetails.EC2ResourceDetails != nil {
					item.CurrentType = aws.ToString(rec.CurrentInstance.ResourceDetails.EC2ResourceDetails.InstanceType)
				}
			}

			for _, code := range rec.FindingReasonCodes {
				item.Reasons = append(item.Reasons, string(code))
			}

			if rec.RightsizingType == cetypes.RightsizingTypeModify && rec.ModifyRecommendationDetail != nil {
				for _, target := range rec.ModifyRecommendationDetail.TargetInstances {
					if target.DefaultTargetInstance || len(rec.ModifyRecommendationDetail.TargetInstances) == 1 {
						item.EstSavings = aws.ToString(target.EstimatedMonthlySavings)
						if target.ResourceDetails != nil && target.ResourceDetails.EC2ResourceDetails != nil {
							item.SuggestedType = aws.ToString(target.ResourceDetails.EC2ResourceDetails.InstanceType)
						}
						break
					}
				}
			}

			if rec.RightsizingType == cetypes.RightsizingTypeTerminate && rec.TerminateRecommendationDetail != nil {
				item.EstSavings = aws.ToString(rec.TerminateRecommendationDetail.EstimatedMonthlySavings)
				item.SuggestedType = "(terminate)"
			}

			items = append(items, item)
		}

		nextToken = out.NextPageToken
		if nextToken == nil {
			break
		}
	}

	sort.Slice(items, func(i, j int) bool {
		si, _ := strconv.ParseFloat(items[i].EstSavings, 64)
		sj, _ := strconv.ParseFloat(items[j].EstSavings, 64)
		return si > sj
	})
	return items, nil
}

// ─── Format Functions ──────────────────────────────────────────────────────────

func FormatMonthlyCostDetail(items []MonthlyCostItem) string {
	colMonth := len("Month")
	colCost := len("Cost")
	months := make([]string, len(items))
	dollars := make([]string, len(items))
	for i, item := range items {
		t, _ := time.Parse("2006-01-02", item.Month)
		months[i] = t.Format("2006-01")
		dollars[i] = FmtDollar(item.Amount)
		if len(months[i]) > colMonth {
			colMonth = len(months[i])
		}
		if len(dollars[i]) > colCost {
			colCost = len(dollars[i])
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-*s  %-*s\n", colMonth, "Month", colCost, "Cost")
	b.WriteString(strings.Repeat("─", colMonth+colCost+2) + "\n")
	for i := range items {
		fmt.Fprintf(&b, "%-*s  %-*s\n", colMonth, months[i], colCost, dollars[i])
	}
	return b.String()
}

func FormatServiceDetailTable(serviceName string, items []ServiceCostItem) string {
	colUsage := len("Usage Type")
	colCost := len("Cost")
	dollars := make([]string, len(items))
	for i, item := range items {
		dollars[i] = FmtDollar(item.Amount)
		if len(item.Service) > colUsage {
			colUsage = len(item.Service)
		}
		if len(dollars[i]) > colCost {
			colCost = len(dollars[i])
		}
	}

	var b strings.Builder
	now := time.Now().UTC()
	fmt.Fprintf(&b, "%s — %s (Top %d)\n\n", serviceName, now.Format("2006-01"), len(items))
	fmt.Fprintf(&b, "%-*s  %-*s\n", colUsage, "Usage Type", colCost, "Cost")
	b.WriteString(strings.Repeat("─", colUsage+colCost+2) + "\n")
	for i, item := range items {
		fmt.Fprintf(&b, "%-*s  %-*s\n", colUsage, item.Service, colCost, dollars[i])
	}
	return b.String()
}

func FormatDailyCostDetail(items []DailyCostItem) string {
	colDate := len("Date")
	colCost := len("Cost")
	dollars := make([]string, len(items))
	for i, item := range items {
		dollars[i] = FmtDollar(item.Amount)
		if len(item.Date) > colDate {
			colDate = len(item.Date)
		}
		if len(dollars[i]) > colCost {
			colCost = len(dollars[i])
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-*s  %-*s\n", colDate, "Date", colCost, "Cost")
	b.WriteString(strings.Repeat("─", colDate+colCost+2) + "\n")
	for i, item := range items {
		fmt.Fprintf(&b, "%-*s  %-*s\n", colDate, item.Date, colCost, dollars[i])
	}
	return b.String()
}

func FormatOptimizationReport(monthly []MonthlyCostItem, rightsizing []RightsizingItem, topRes []ResourceCostItem) string {
	var b strings.Builder

	// ── Monthly Trend ──
	b.WriteString("[Monthly Trend]\n")
	colM := len("Month")
	colC := len("Cost")
	colG := len("Change")
	ms := make([]string, len(monthly))
	ds := make([]string, len(monthly))
	gs := make([]string, len(monthly))
	prevAmt := 0.0
	for i, item := range monthly {
		t, _ := time.Parse("2006-01-02", item.Month)
		ms[i] = t.Format("2006-01")
		ds[i] = FmtDollar(item.Amount)
		cur, _ := strconv.ParseFloat(item.Amount, 64)
		if i > 0 && prevAmt > 0 {
			pct := (cur - prevAmt) / prevAmt * 100
			if pct >= 0 {
				gs[i] = fmt.Sprintf("+%.1f%%", pct)
			} else {
				gs[i] = fmt.Sprintf("%.1f%%", pct)
			}
		}
		prevAmt = cur
		if len(ms[i]) > colM {
			colM = len(ms[i])
		}
		if len(ds[i]) > colC {
			colC = len(ds[i])
		}
		if len(gs[i]) > colG {
			colG = len(gs[i])
		}
	}
	fmt.Fprintf(&b, "%-*s  %-*s  %-*s\n", colM, "Month", colC, "Cost", colG, "Change")
	b.WriteString(strings.Repeat("─", colM+colC+colG+4) + "\n")
	for i := range monthly {
		fmt.Fprintf(&b, "%-*s  %-*s  %-*s\n", colM, ms[i], colC, ds[i], colG, gs[i])
	}

	// ── Top 10 Resources ──
	if len(topRes) > 0 {
		b.WriteString("\n[Top Cost Resources]\n")
		n := len(topRes)
		if n > 10 {
			n = 10
		}
		colR := len("Resource")
		colS := len("Service")
		colA := len("Cost")
		rds := make([]string, n)
		svcs := make([]string, n)
		amts := make([]string, n)
		for i := 0; i < n; i++ {
			rds[i] = topRes[i].ResourceID
			if len(rds[i]) > 48 {
				rds[i] = rds[i][:45] + "..."
			}
			svcs[i] = ShortenService(topRes[i].Service)
			amts[i] = fmtDollarF(topRes[i].Amount)
			if len(rds[i]) > colR {
				colR = len(rds[i])
			}
			if len(svcs[i]) > colS {
				colS = len(svcs[i])
			}
			if len(amts[i]) > colA {
				colA = len(amts[i])
			}
		}
		fmt.Fprintf(&b, "%-*s  %-*s  %-*s\n", colR, "Resource", colS, "Service", colA, "Cost")
		b.WriteString(strings.Repeat("─", colR+colS+colA+4) + "\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "%-*s  %-*s  %-*s\n", colR, rds[i], colS, svcs[i], colA, amts[i])
		}

		// concentration analysis
		totalTop := 0.0
		for _, r := range topRes {
			totalTop += r.Amount
		}
		if totalTop > 0 && len(topRes) >= 3 {
			top3 := topRes[0].Amount + topRes[1].Amount + topRes[2].Amount
			pct := top3 / totalTop * 100
			if pct > 50 {
				fmt.Fprintf(&b, "\n* Top 3 resources account for %.0f%% of total resource cost — consider reviewing them.\n", pct)
			}
		}
	}

	// ── EC2 Rightsizing ──
	b.WriteString("\n[EC2 Rightsizing Recommendations]\n")
	if len(rightsizing) == 0 {
		b.WriteString("No recommendations — all instances appear right-sized.\n")
	} else {
		colID := len("Instance")
		colNm := 0
		colCur := len("Current")
		colAct := len("Action")
		colSug := len("Suggestion")
		colSav := len("Savings/mo")
		for _, r := range rightsizing {
			if len(r.InstanceID) > colID {
				colID = len(r.InstanceID)
			}
			if r.InstanceName != "" && len(r.InstanceName) > colNm {
				colNm = len(r.InstanceName)
			}
			if len(r.CurrentType) > colCur {
				colCur = len(r.CurrentType)
			}
			if len(r.Action) > colAct {
				colAct = len(r.Action)
			}
			if len(r.SuggestedType) > colSug {
				colSug = len(r.SuggestedType)
			}
			sav := FmtDollar(r.EstSavings)
			if len(sav) > colSav {
				colSav = len(sav)
			}
		}
		hasName := colNm > 0
		if hasName && colNm < len("Name") {
			colNm = len("Name")
		}

		if hasName {
			fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %-*s  %-*s  %-*s\n",
				colID, "Instance", colNm, "Name", colCur, "Current", colAct, "Action", colSug, "Suggestion", colSav, "Savings/mo")
			b.WriteString(strings.Repeat("─", colID+colNm+colCur+colAct+colSug+colSav+10) + "\n")
		} else {
			fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %-*s  %-*s\n",
				colID, "Instance", colCur, "Current", colAct, "Action", colSug, "Suggestion", colSav, "Savings/mo")
			b.WriteString(strings.Repeat("─", colID+colCur+colAct+colSug+colSav+8) + "\n")
		}
		for _, r := range rightsizing {
			sav := FmtDollar(r.EstSavings)
			if hasName {
				name := r.InstanceName
				if name == "" {
					name = "-"
				}
				fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %-*s  %-*s  %-*s\n",
					colID, r.InstanceID, colNm, name, colCur, r.CurrentType, colAct, r.Action, colSug, r.SuggestedType, colSav, sav)
			} else {
				fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %-*s  %-*s\n",
					colID, r.InstanceID, colCur, r.CurrentType, colAct, r.Action, colSug, r.SuggestedType, colSav, sav)
			}
		}
		totalSavings := 0.0
		for _, r := range rightsizing {
			s, _ := strconv.ParseFloat(r.EstSavings, 64)
			totalSavings += s
		}
		if totalSavings > 0 {
			fmt.Fprintf(&b, "\nTotal estimated savings: %s/mo\n", fmtDollarF(totalSavings))
		}
		// show reason summary
		reasonCount := make(map[string]int)
		for _, r := range rightsizing {
			for _, reason := range r.Reasons {
				reasonCount[reason]++
			}
		}
		if len(reasonCount) > 0 {
			b.WriteString("\nTop findings:\n")
			type rc struct {
				reason string
				count  int
			}
			var rcs []rc
			for r, c := range reasonCount {
				rcs = append(rcs, rc{r, c})
			}
			sort.Slice(rcs, func(i, j int) bool { return rcs[i].count > rcs[j].count })
			for _, r := range rcs {
				fmt.Fprintf(&b, "  %s (%d instances)\n", humanizeReason(r.reason), r.count)
			}
		}
	}

	// ── General Tips ──
	b.WriteString("\n[Optimization Tips]\n")
	b.WriteString("- For stable workloads, consider Reserved Instances or Savings Plans.\n")
	b.WriteString("- Review unused EBS volumes, idle ELBs, and old snapshots.\n")
	b.WriteString("- Enable S3 Lifecycle policies for infrequently accessed data.\n")
	b.WriteString("- Use S3 Intelligent-Tiering for unpredictable access patterns.\n")
	b.WriteString("- Check for idle NAT Gateways and unused Elastic IPs.\n")

	return b.String()
}

func ShortenService(name string) string {
	m := map[string]string{
		"Amazon Elastic Compute Cloud - Compute":        "EC2",
		"Amazon Elastic Compute Cloud - Other":          "EC2-Other",
		"Amazon Simple Storage Service":                 "S3",
		"Amazon Relational Database Service":            "RDS",
		"Amazon ElastiCache":                            "ElastiCache",
		"Amazon Elastic Container Service":              "ECS",
		"Amazon Elastic Container Service for Kubernetes": "EKS",
		"Amazon Elastic Load Balancing":                 "ELB",
		"AWS Lambda":                                    "Lambda",
		"Amazon DynamoDB":                               "DynamoDB",
		"Amazon CloudFront":                             "CloudFront",
		"Amazon Virtual Private Cloud":                  "VPC",
		"Amazon Elastic Block Store":                    "EBS",
		"AmazonCloudWatch":                              "CloudWatch",
		"Amazon CloudWatch":                             "CloudWatch",
		"AWS Key Management Service":                    "KMS",
		"Amazon Simple Notification Service":            "SNS",
		"Amazon Simple Queue Service":                   "SQS",
		"Amazon Elastic File System":                    "EFS",
		"Amazon Route 53":                               "Route53",
		"Amazon ECR Public":                             "ECR",
		"Amazon EC2 Container Registry (ECR)":           "ECR",
	}
	if s, ok := m[name]; ok {
		return s
	}
	if len(name) > 20 {
		return name[:17] + "..."
	}
	return name
}

func humanizeReason(code string) string {
	m := map[string]string{
		"CPU_OVER_PROVISIONED":              "CPU over-provisioned",
		"MEMORY_OVER_PROVISIONED":           "Memory over-provisioned",
		"NETWORK_BANDWIDTH_OVER_PROVISIONED": "Network over-provisioned",
		"DISK_IOPS_OVER_PROVISIONED":        "Disk IOPS over-provisioned",
		"DISK_THROUGHPUT_OVER_PROVISIONED":   "Disk throughput over-provisioned",
		"CPU_UNDER_PROVISIONED":             "CPU under-provisioned",
		"MEMORY_UNDER_PROVISIONED":          "Memory under-provisioned",
		"NETWORK_BANDWIDTH_UNDER_PROVISIONED": "Network under-provisioned",
		"DISK_IOPS_UNDER_PROVISIONED":       "Disk IOPS under-provisioned",
		"DISK_THROUGHPUT_UNDER_PROVISIONED":  "Disk throughput under-provisioned",
	}
	if s, ok := m[code]; ok {
		return s
	}
	return strings.ReplaceAll(strings.ToLower(code), "_", " ")
}
