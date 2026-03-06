package app

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/charmbracelet/bubbletea"
	"github.com/shidaxi/iaws/internal/config"
	ilog "github.com/shidaxi/iaws/internal/log"
	"github.com/shidaxi/iaws/internal/services"
)

func loadProfilesCmd() tea.Cmd {
	return func() tea.Msg {
		profiles, err := config.ProfilesFromConfig()
		if err != nil {
			profiles = []string{"default"}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

func loadAWSCmd(ctx context.Context, profile, region string) tea.Cmd {
	return func() tea.Msg {
		ilog.Info("cmd: loading AWS config profile=%q region=%q", profile, region)
		aws, err := config.Load(ctx, config.LoadOptions{Profile: profile, Region: region})
		if err != nil {
			ilog.Error("cmd: load AWS config failed: %v", err)
			return errMsg{err: err}
		}
		ilog.Info("cmd: AWS config loaded successfully")
		return awsLoadedMsg{aws: aws, profile: profile, region: region}
	}
}

type errMsg struct{ err error }

// --- EC2 ---

func ec2InstancesCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.ec2.ListInstances(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		return pagedInstancesMsg{items: items, nextToken: next, nextState: stateEC2InstanceList, append: more}
	}
}

func ec2VPCsCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.ec2.ListVPCs(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		for _, v := range items {
			title := fmt.Sprintf("%-24s %-20s %s", v.ID, v.CIDR, v.State)
			if filter != "" && !containsCI(title, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: title, ID: v.ID})
		}
		return listLoadedMsg{items: entries, nextState: stateEC2VPCList}
	}
}

func ec2SubnetsCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.ec2.ListSubnets(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		for _, s := range items {
			title := fmt.Sprintf("%-26s %-20s %s", s.ID, s.CIDR, s.AZ)
			if filter != "" && !containsCI(title, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: title, ID: s.ID})
		}
		return listLoadedMsg{items: entries, nextState: stateEC2SubnetList}
	}
}

func ec2SGsCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.ec2.ListSecurityGroups(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		for _, g := range items {
			title := fmt.Sprintf("%-24s %s", g.ID, g.Name)
			if filter != "" && !containsCI(title, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: title, ID: g.ID})
		}
		return listLoadedMsg{items: entries, nextState: stateEC2SGList}
	}
}

func ec2KeyPairsCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.ec2.ListKeyPairs(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		for _, k := range items {
			if filter != "" && !containsCI(k.Name, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: k.Name, ID: k.Name})
		}
		return listLoadedMsg{items: entries, nextState: stateEC2KeyList}
	}
}

func ec2VolumesCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.ec2.ListVolumes(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		var vols []services.VolumeItem
		for _, v := range items {
			title := fmt.Sprintf("%-24s %-10s %s", v.ID, fmt.Sprintf("%dGiB", v.Size), v.State)
			if filter != "" && !containsCI(title, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: title, ID: v.ID})
			vols = append(vols, v)
		}
		return volumeListLoadedMsg{items: entries, volumes: vols}
	}
}

func ec2AMIsCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.ec2.ListAMIs(m.ctx, false)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		for _, a := range items {
			title := fmt.Sprintf("%-24s %s", a.ID, a.Name)
			if filter != "" && !containsCI(title, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: title, ID: a.ID})
		}
		return listLoadedMsg{items: entries, nextState: stateEC2AMIList}
	}
}

func (m *model) runEC2Menu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.resetPage()
		return m, ec2InstancesCmd(m, nil, false, "")
	case 1:
		return m, ec2VPCsCmd(m, "")
	case 2:
		return m, ec2SubnetsCmd(m, "")
	case 3:
		return m, ec2SGsCmd(m, "")
	case 4:
		return m, ec2KeyPairsCmd(m, "")
	case 5:
		return m, ec2VolumesCmd(m, "")
	case 6:
		return m, ec2AMIsCmd(m, "")
	}
	return m, nil
}

// --- SSM ---

func ssmParamsCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.ssm.ListParameters(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, p := range items {
			entries[i] = listEntry{Title: fmt.Sprintf("%-50s %s", p.Name, p.Type), ID: p.Name}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateSSMParamList, append: more}
	}
}

func getSSMParamCmd(m *model, name string) tea.Cmd {
	return func() tea.Msg {
		value, err := m.ssm.GetParameter(m.ctx, name)
		if err != nil {
			return errMsg{err: err}
		}
		return ssmParamValueLoadedMsg{value: value}
	}
}

func startSSMSessionCmd(m *model, instanceID string) tea.Cmd {
	ilog.Info("cmd: starting SSM session for instance=%s", instanceID)
	var credsPtr *aws.Credentials
	if m.aws != nil {
		creds, err := m.aws.Config.Credentials.Retrieve(m.ctx)
		if err != nil {
			ilog.Error("cmd: retrieve credentials failed: %v", err)
		} else if creds.SessionToken != "" {
			ilog.Info("cmd: using cached credentials (has SessionToken)")
			credsPtr = &creds
		} else {
			ilog.Info("cmd: credentials have no SessionToken, using profile/region flags")
		}
	}
	cmd, err := m.ssm.BuildSessionCmd(credsPtr, instanceID)
	if err != nil {
		ilog.Error("cmd: build session cmd failed: %v", err)
		return func() tea.Msg { return errMsg{err: err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			ilog.Error("cmd: SSM session ended with error: %v", err)
			return errMsg{err: err}
		}
		ilog.Info("cmd: SSM session ended normally")
		return sessionDoneMsg{}
	})
}

type sessionDoneMsg struct{}

func ssmLoginInstancesCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		if m.ec2 == nil {
			return errMsg{err: fmt.Errorf("EC2 client not initialized")}
		}
		items, next, err := m.ec2.ListInstances(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		return pagedInstancesMsg{items: items, nextToken: next, nextState: stateSSMLoginInstanceList, append: more}
	}
}

func (m *model) runSSMMenu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.resetPage()
		return m, ssmLoginInstancesCmd(m, nil, false, "")
	case 1:
		m.resetPage()
		return m, ssmParamsCmd(m, nil, false, "")
	case 2:
		m.prevSecretAction = "get"
		m.resetPage()
		return m, secretsListCmd(m, nil, false, "")
	}
	return m, nil
}

// --- Secrets ---

func secretsListCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.secrets.ListSecrets(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, s := range items {
			entries[i] = listEntry{Title: s.Name, ID: s.Name}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateSecretsList, append: more}
	}
}

func getSecretValueCmd(m *model, name string) tea.Cmd {
	return func() tea.Msg {
		value, err := m.secrets.GetSecretValue(m.ctx, name)
		if err != nil {
			return errMsg{err: err}
		}
		return secretValueLoadedMsg{value: value}
	}
}

func (m *model) runSecretsMenu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.resetPage()
		return m, secretsListCmd(m, nil, false, "")
	case 1:
		m.prevSecretAction = "get"
		m.resetPage()
		return m, secretsListCmd(m, nil, false, "")
	case 2:
		m.prevSecretAction = "put"
		m.resetPage()
		return m, secretsListCmd(m, nil, false, "")
	}
	return m, nil
}

// --- S3 ---

func listS3ObjectsCmd(m *model, bucket, prefix string, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.s3.ListObjects(m.ctx, bucket, prefix, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		if !more && prefix != "" {
			entries = append(entries, listEntry{Title: "..", ID: ".."})
		}
		for _, o := range items {
			if o.IsDir {
				entries = append(entries, listEntry{Title: o.Key, ID: o.Key, IsDir: true})
			} else {
				entries = append(entries, listEntry{
					Title: fmt.Sprintf("%-50s %d", o.Key, o.Size),
					ID:    o.Key,
					IsDir: false,
				})
			}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateS3ObjectList, append: more}
	}
}

func s3BucketsCmd(m *model, filter string) tea.Cmd {
	return func() tea.Msg {
		names, err := m.s3.ListBuckets(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		var entries []listEntry
		for _, n := range names {
			if filter != "" && !containsCI(n, filter) {
				continue
			}
			entries = append(entries, listEntry{Title: n, ID: n})
		}
		return s3BucketsLoadedMsg{items: entries}
	}
}

func (m *model) runS3Menu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		return m, s3BucketsCmd(m, "")
	case 1:
		m.s3MenuUpload = false
		return m, s3BucketsCmd(m, "")
	case 2:
		m.s3MenuUpload = false
		return m, s3BucketsCmd(m, "")
	case 3:
		m.s3MenuUpload = true
		return m, s3BucketsCmd(m, "")
	}
	return m, nil
}

// --- ACM ---

func acmCertsCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.acm.ListCertificates(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, c := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-40s %-16s %s", c.DomainName, c.Type, c.Status),
				ID:    c.ARN,
			}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateACMCertList, append: more}
	}
}

func acmDescribeCmd(m *model, arn string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.acm.DescribeCertificate(m.ctx, arn)
		if err != nil {
			return errMsg{err: err}
		}
		return certDetailMsg{detail: detail.String()}
	}
}

func (m *model) runACMMenu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.resetPage()
		return m, acmCertsCmd(m, nil, false, "")
	}
	return m, nil
}

// --- Route53 ---

func r53ZonesCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.r53.ListHostedZones(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, z := range items {
			vis := "public"
			if z.Private {
				vis = "private"
			}
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-40s %-8s %d records", z.Name, vis, z.Records),
				ID:    z.ID + "|" + z.Name,
			}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateRoute53ZoneList, append: more}
	}
}

func r53RecordsCmd(m *model, zoneID string, startName *string, startType *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, nextName, nextType, err := m.r53.ListRecordSets(m.ctx, zoneID, startName, startType, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, r := range items {
			entries[i] = listEntry{
				Title: services.FormatRecordSet(r),
				ID:    r.Name,
			}
		}
		return r53RecordsMsg{items: entries, nextName: nextName, nextType: nextType, nextState: stateRoute53RecordList, append: more}
	}
}

func (m *model) runRoute53Menu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.resetPage()
		return m, r53ZonesCmd(m, nil, false, "")
	}
	return m, nil
}

// --- EKS ---

func eksClusterListCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.eksSvc.ListClusters(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, name := range items {
			entries[i] = listEntry{Title: name, ID: name}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateEKSClusterList, append: more}
	}
}

func eksDescribeClusterCmd(m *model, name string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.eksSvc.DescribeCluster(m.ctx, name)
		if err != nil {
			return errMsg{err: err}
		}
		return detailLoadedMsg{detail: detail, backState: stateEKSClusterList}
	}
}

// --- ECR ---

func ecrRepoListCmd(m *model, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.ecrSvc.ListRepositories(m.ctx, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, r := range items {
			entries[i] = listEntry{Title: fmt.Sprintf("%-40s %s", r.Name, r.URI), ID: r.Name}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateECRRepoList, append: more}
	}
}

func ecrImageListCmd(m *model, repoName string, token *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.ecrSvc.ListImages(m.ctx, repoName, token, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, line := range items {
			entries[i] = listEntry{Title: line, ID: line}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateECRImageList, append: more}
	}
}

// --- ELB ---

func elbListCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.elbSvc.ListLoadBalancers(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, lb := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-32s %-14s %-18s %s", lb.Name, lb.Type, lb.Scheme, lb.State),
				ID:    lb.Name,
			}
			dm[lb.Name] = services.FormatLBDetail(lb)
		}
		return elbListLoadedMsg{items: entries, nextToken: next, append: more, detailMap: dm}
	}
}

// --- IAM ---

func iamUsersCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.iamSvc.ListUsers(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, u := range items {
			entries[i] = listEntry{Title: fmt.Sprintf("%-30s %s", u.Name, u.Created), ID: u.Name}
			dm[u.Name] = services.FormatUserDetail(u)
		}
		return iamListLoadedMsg{items: entries, nextToken: next, nextState: stateIAMUserList, append: more, detailMap: dm}
	}
}

func iamRolesCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.iamSvc.ListRoles(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, r := range items {
			entries[i] = listEntry{Title: fmt.Sprintf("%-40s %s", r.Name, r.Desc), ID: r.Name}
			dm[r.Name] = services.FormatRoleDetail(r)
		}
		return iamListLoadedMsg{items: entries, nextToken: next, nextState: stateIAMRoleList, append: more, detailMap: dm}
	}
}

func iamPoliciesCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.iamSvc.ListPolicies(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, p := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-40s attached:%d", p.Name, p.AttachmentCount),
				ID:    p.Name,
			}
			dm[p.Name] = services.FormatPolicyDetail(p)
		}
		return iamListLoadedMsg{items: entries, nextToken: next, nextState: stateIAMPolicyList, append: more, detailMap: dm}
	}
}

func (m *model) runIAMMenu(idx int) (tea.Model, tea.Cmd) {
	m.resetPage()
	m.detailMap = nil
	switch idx {
	case 0:
		return m, iamUsersCmd(m, nil, false, "")
	case 1:
		return m, iamRolesCmd(m, nil, false, "")
	case 2:
		return m, iamPoliciesCmd(m, nil, false, "")
	}
	return m, nil
}

// --- RDS ---

func rdsInstanceListCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.rdsSvc.ListDBInstances(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, db := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-30s %-20s %-16s %-10s %s", db.ID, db.Engine, db.Class, fmt.Sprintf("%dGiB", db.Storage), db.Status),
				ID:    db.ID,
			}
			dm[db.ID] = services.FormatDBDetail(db)
		}
		return rdsListLoadedMsg{items: entries, nextToken: next, append: more, detailMap: dm}
	}
}

// --- KMS ---

func kmsKeyListCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.kmsSvc.ListKeys(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		for i, k := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-35s %s", k.Alias, k.ID),
				ID:    k.ID,
			}
		}
		return pagedListMsg{items: entries, nextToken: next, nextState: stateKMSKeyList, append: more}
	}
}

func kmsDescribeKeyCmd(m *model, keyID string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.kmsSvc.DescribeKey(m.ctx, keyID)
		if err != nil {
			return errMsg{err: err}
		}
		return detailLoadedMsg{detail: detail, backState: stateKMSKeyList}
	}
}

// --- CloudFront ---

func cfDistListCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.cfSvc.ListDistributions(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, d := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-16s %-40s %-10s %s", d.ID, d.Domain, d.Status, d.Aliases),
				ID:    d.ID,
			}
			dm[d.ID] = services.FormatDistDetail(d)
		}
		return cfListLoadedMsg{items: entries, nextToken: next, append: more, detailMap: dm}
	}
}

// --- Lambda ---

func lambdaFuncListCmd(m *model, marker *string, more bool, filter string) tea.Cmd {
	return func() tea.Msg {
		items, next, err := m.lambdaSvc.ListFunctions(m.ctx, marker, filter)
		if err != nil {
			return errMsg{err: err}
		}
		entries := make([]listEntry, len(items))
		dm := make(map[string]string, len(items))
		for i, f := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-40s %-14s %dMB", f.Name, f.Runtime, f.Memory),
				ID:    f.Name,
			}
			dm[f.Name] = services.FormatFuncDetail(f)
		}
		return lambdaListLoadedMsg{items: entries, nextToken: next, append: more, detailMap: dm}
	}
}

// --- Billing ---

func billingMonthlyCostCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		items, err := m.billingSvc.GetMonthlyCost(m.ctx, 6)
		if err != nil {
			return errMsg{err: err}
		}
		return billingCostMsg{
			detail:    services.FormatMonthlyCostDetail(items),
			backState: stateBillingMenu,
		}
	}
}

func billingServiceCostCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		items, err := m.billingSvc.GetCostByService(m.ctx)
		if err != nil {
			return errMsg{err: err}
		}
		maxSvc := 0
		maxCost := 0
		dollars := make([]string, len(items))
		for i, item := range items {
			dollars[i] = services.FmtDollar(item.Amount)
			if len(item.Service) > maxSvc {
				maxSvc = len(item.Service)
			}
			if len(dollars[i]) > maxCost {
				maxCost = len(dollars[i])
			}
		}
		entries := make([]listEntry, len(items))
		for i, item := range items {
			entries[i] = listEntry{
				Title: fmt.Sprintf("%-*s  %-*s", maxSvc, item.Service, maxCost, dollars[i]),
				ID:    item.Service,
			}
		}
		return listLoadedMsg{items: entries, nextState: stateBillingServiceCost}
	}
}

func billingServiceDetailCmd(m *model, serviceName string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.billingSvc.GetServiceCostDetail(m.ctx, serviceName, 50)
		if err != nil {
			return errMsg{err: err}
		}
		return billingCostMsg{
			detail:    services.FormatServiceDetailTable(serviceName, items),
			backState: stateBillingServiceCost,
		}
	}
}

func billingDailyCostCmd(m *model) tea.Cmd {
	return func() tea.Msg {
		items, err := m.billingSvc.GetDailyCost(m.ctx, 30)
		if err != nil {
			return errMsg{err: err}
		}
		return billingCostMsg{
			detail:    services.FormatDailyCostDetail(items),
			backState: stateBillingMenu,
		}
	}
}

func (m *model) runBillingMenu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		return m, billingMonthlyCostCmd(m)
	case 1:
		return m, billingServiceCostCmd(m)
	case 2:
		return m, billingDailyCostCmd(m)
	}
	return m, nil
}

// --- EC2 instance actions ---

func (m *model) onEC2InstanceActionSelect(menuIdx int) (tea.Model, tea.Cmd) {
	switch menuIdx {
	case 0:
		err := m.ec2.StartInstance(m.ctx, m.confirmTarget)
		if err != nil {
			m.setMessage(true, err.Error())
			return m, nil
		}
		m.setMessage(false, "Instance started.")
		return m, nil
	case 1:
		m.confirmMsg = "Stop instance " + m.confirmTarget + "? (y/n)"
		m.confirmAction = "stop"
		m.confirmBackState = stateEC2InstanceList
		m.kind = stateConfirm
		return m, nil
	case 2:
		m.confirmMsg = "Reboot instance " + m.confirmTarget + "? (y/n)"
		m.confirmAction = "reboot"
		m.confirmBackState = stateEC2InstanceList
		m.kind = stateConfirm
		return m, nil
	}
	return m, nil
}

func putS3ObjectCmd(m *model, bucket, key, localPath string) tea.Cmd {
	return func() tea.Msg {
		err := m.s3.PutObject(m.ctx, bucket, key, localPath)
		if err != nil {
			return errMsg{err: err}
		}
		return s3UploadDoneMsg{bucket: bucket, key: key}
	}
}

func getS3ObjectCmd(m *model, bucket, key, localPath string) tea.Cmd {
	return func() tea.Msg {
		err := m.s3.GetObject(m.ctx, bucket, key, localPath)
		if err != nil {
			return errMsg{err: err}
		}
		return s3DownloadDoneMsg{path: localPath}
	}
}

func (m *model) runConfirm() (tea.Model, tea.Cmd) {
	var err error
	switch m.confirmAction {
	case "stop":
		err = m.ec2.StopInstance(m.ctx, m.confirmTarget)
	case "reboot":
		err = m.ec2.RebootInstance(m.ctx, m.confirmTarget)
	case "start":
		err = m.ec2.StartInstance(m.ctx, m.confirmTarget)
	case "putSecret":
		err = m.secrets.PutSecretValue(m.ctx, m.confirmTarget, m.confirmData)
	}
	if err != nil {
		m.setMessage(true, err.Error())
		return m, nil
	}
	m.prevStateAfterMessage = m.confirmBackState
	m.setMessage(false, "Done.")
	return m, nil
}

// loadMoreCmd dispatches a "load more" fetch for the current state
func (m *model) loadMoreCmd() tea.Cmd {
	if !m.hasMore || m.pageToken == "" {
		return nil
	}
	token := m.pageToken
	filter := m.filter
	switch m.kind {
	case stateEC2InstanceList:
		return ec2InstancesCmd(m, &token, true, filter)
	case stateSSMParamList:
		return ssmParamsCmd(m, &token, true, filter)
	case stateSSMLoginInstanceList:
		return ssmLoginInstancesCmd(m, &token, true, filter)
	case stateSecretsList:
		return secretsListCmd(m, &token, true, filter)
	case stateS3ObjectList:
		return listS3ObjectsCmd(m, m.s3Bucket, m.s3Prefix, &token, true, filter)
	case stateACMCertList:
		return acmCertsCmd(m, &token, true, filter)
	case stateRoute53ZoneList:
		return r53ZonesCmd(m, &token, true, filter)
	case stateRoute53RecordList:
		return r53RecordsCmd(m, m.r53ZoneID, &token, m.r53RecordNextType, true, filter)
	case stateEKSClusterList:
		return eksClusterListCmd(m, &token, true, filter)
	case stateECRRepoList:
		return ecrRepoListCmd(m, &token, true, filter)
	case stateECRImageList:
		return ecrImageListCmd(m, m.ecrRepoName, &token, true, filter)
	case stateELBList:
		return elbListCmd(m, &token, true, filter)
	case stateIAMUserList:
		return iamUsersCmd(m, &token, true, filter)
	case stateIAMRoleList:
		return iamRolesCmd(m, &token, true, filter)
	case stateIAMPolicyList:
		return iamPoliciesCmd(m, &token, true, filter)
	case stateRDSInstanceList:
		return rdsInstanceListCmd(m, &token, true, filter)
	case stateKMSKeyList:
		return kmsKeyListCmd(m, &token, true, filter)
	case stateCloudFrontDistList:
		return cfDistListCmd(m, &token, true, filter)
	case stateLambdaFunctionList:
		return lambdaFuncListCmd(m, &token, true, filter)
	}
	return nil
}

// triggerSearchCmd dispatches a fresh search for the current state using m.filter
func (m *model) triggerSearchCmd() tea.Cmd {
	filter := m.filter
	switch m.kind {
	case stateEC2InstanceList:
		return ec2InstancesCmd(m, nil, false, filter)
	case stateEC2VPCList:
		return ec2VPCsCmd(m, filter)
	case stateEC2SubnetList:
		return ec2SubnetsCmd(m, filter)
	case stateEC2SGList:
		return ec2SGsCmd(m, filter)
	case stateEC2KeyList:
		return ec2KeyPairsCmd(m, filter)
	case stateEC2VolumeList:
		return ec2VolumesCmd(m, filter)
	case stateEC2AMIList:
		return ec2AMIsCmd(m, filter)
	case stateSSMParamList:
		return ssmParamsCmd(m, nil, false, filter)
	case stateSSMLoginInstanceList:
		return ssmLoginInstancesCmd(m, nil, false, filter)
	case stateSecretsList:
		return secretsListCmd(m, nil, false, filter)
	case stateS3BucketList:
		return s3BucketsCmd(m, filter)
	case stateS3ObjectList:
		return listS3ObjectsCmd(m, m.s3Bucket, m.s3Prefix, nil, false, filter)
	case stateACMCertList:
		return acmCertsCmd(m, nil, false, filter)
	case stateRoute53ZoneList:
		return r53ZonesCmd(m, nil, false, filter)
	case stateRoute53RecordList:
		return r53RecordsCmd(m, m.r53ZoneID, nil, nil, false, filter)
	case stateEKSClusterList:
		return eksClusterListCmd(m, nil, false, filter)
	case stateECRRepoList:
		return ecrRepoListCmd(m, nil, false, filter)
	case stateECRImageList:
		return ecrImageListCmd(m, m.ecrRepoName, nil, false, filter)
	case stateELBList:
		return elbListCmd(m, nil, false, filter)
	case stateIAMUserList:
		return iamUsersCmd(m, nil, false, filter)
	case stateIAMRoleList:
		return iamRolesCmd(m, nil, false, filter)
	case stateIAMPolicyList:
		return iamPoliciesCmd(m, nil, false, filter)
	case stateRDSInstanceList:
		return rdsInstanceListCmd(m, nil, false, filter)
	case stateKMSKeyList:
		return kmsKeyListCmd(m, nil, false, filter)
	case stateCloudFrontDistList:
		return cfDistListCmd(m, nil, false, filter)
	case stateLambdaFunctionList:
		return lambdaFuncListCmd(m, nil, false, filter)
	}
	return nil
}
