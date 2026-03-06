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

func FmtDollar(amountStr string) string {
	f, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return "$" + amountStr
	}
	return fmt.Sprintf("$%.2f", f)
}

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
