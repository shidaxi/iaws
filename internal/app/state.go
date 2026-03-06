package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shidaxi/iaws/internal/config"
	"github.com/shidaxi/iaws/internal/services"
)

type stateKind int

const (
	stateProfile stateKind = iota
	stateRegion
	stateMainMenu
	stateEC2Menu
	stateEC2InstanceList
	stateEC2VPCList
	stateEC2SubnetList
	stateEC2SGList
	stateEC2KeyList
	stateEC2VolumeList
	stateEC2AMIList
	stateEC2InstanceAction // Start/Stop/Reboot
	stateSSMMenu
	stateSSMParamList
	stateSSMParamGet
	stateSSMLoginInstanceList
	stateSecretsMenu
	stateSecretsList
	stateSecretGet
	stateSecretPut
	stateS3Menu
	stateS3BucketList
	stateS3ObjectList
	stateS3GetObject
	stateS3PutObject
	stateACMMenu
	stateACMCertList
	stateACMCertDetail
	stateRoute53Menu
	stateRoute53ZoneList
	stateRoute53RecordList
	stateEKSClusterList
	stateECRRepoList
	stateECRImageList
	stateELBList
	stateIAMMenu
	stateIAMUserList
	stateIAMRoleList
	stateIAMPolicyList
	stateRDSInstanceList
	stateKMSKeyList
	stateCloudFrontDistList
	stateLambdaFunctionList
	stateBillingMenu
	stateBillingMonthlyCost
	stateBillingServiceCost
	stateBillingDailyCost
	stateConfirm
	stateMessage // show error or success then back
)

const loadMoreID = "__load_more__"

type listEntry struct {
	Title string
	ID    string // instance id, secret name, bucket name, etc.
	IsDir bool   // for S3 prefix
}

// Model is the root Bubble Tea model.
type Model struct {
	ctx context.Context

	kind   stateKind
	width  int // terminal width
	aws    *config.AWS
	profile string
	region  string

	// list state: items and filter
	items    []listEntry
	filter   string
	selected int
	// 从 region 返回 profile 时恢复的 profile 列表
	savedProfileItems []listEntry
	// 从 main menu 返回 region 时恢复的 region 列表
	savedRegionItems []listEntry
	// for EC2 instance list we need full InstanceItem for display; reuse items as title, ID as instance id
	ec2Instances []services.InstanceItem
	// for EC2 volume list sorting
	ec2Volumes     []services.VolumeItem
	volumeSortDesc bool

	// menu: choices and selected index
	menuItems   []string
	menuSelected int

	// confirm
	confirmMsg      string
	confirmAction   string
	confirmTarget   string
	confirmData     string // extra data e.g. secret value
	confirmBackState stateKind

	// message (error/success)
	msgText              string
	msgErr               bool
	prevStateAfterMessage stateKind

	// SSM: param name for get
	ssmParamName string
	// Secrets: name for get/put
	secretName   string
	secretValue  string
	// S3: bucket, prefix, key, local path
	s3Bucket    string
	s3Prefix    string
	s3Key       string
	s3LocalPath string
	s3PutKey    string   // for upload flow
	s3PutStep   int      // 0=key, 1=path

	// Secrets: when in list, did we come from "get" or "put"
	prevSecretAction string
	// Optional text input for put secret value
	inputValue string

	// S3 menu: true when we chose "Upload file" and are selecting bucket
	s3MenuUpload bool

	// server-side pagination
	pageToken string // next-page token for current list
	hasMore   bool   // whether there are more pages

	// remote search (debounce)
	searchSeq     int  // incremented on each keystroke; used to discard stale ticks
	searchPending bool // filter changed since last search; Enter will trigger search
	searching     bool // a search request is in flight

	// Route53
	r53ZoneID          string
	r53ZoneName        string
	r53RecordNextType  *string

	// ECR: selected repo for image listing
	ecrRepoName string

	// detail cache: keyed by entry.ID, holds formatted detail strings
	detailMap map[string]string

	// services (created after aws loaded)
	ec2       *services.EC2Client
	ssm       *services.SSMClient
	secrets   *services.SecretsClient
	s3        *services.S3Client
	acm       *services.ACMClient
	r53       *services.Route53Client
	eksSvc    *services.EKSClient
	ecrSvc    *services.ECRClient
	elbSvc    *services.ELBClient
	iamSvc    *services.IAMClient
	rdsSvc    *services.RDSClient
	kmsSvc    *services.KMSClient
	cfSvc     *services.CloudFrontClient
	lambdaSvc  *services.LambdaClient
	billingSvc *services.BillingClient
}

// model is an alias for backward compatibility in this package.
type model = Model

// New creates a new Model with the given context.
func New(ctx context.Context) *Model {
	return &Model{ctx: ctx}
}

func (m *model) ensureClients() {
	if m.aws == nil {
		return
	}
	cfg := m.aws.Config
	if m.ec2 == nil {
		m.ec2 = services.NewEC2(cfg)
	}
	if m.ssm == nil {
		m.ssm = services.NewSSM(cfg, m.profile)
	}
	if m.secrets == nil {
		m.secrets = services.NewSecrets(cfg)
	}
	if m.s3 == nil {
		m.s3 = services.NewS3(cfg)
	}
	if m.acm == nil {
		m.acm = services.NewACM(cfg)
	}
	if m.r53 == nil {
		m.r53 = services.NewRoute53(cfg)
	}
	if m.eksSvc == nil {
		m.eksSvc = services.NewEKS(cfg)
	}
	if m.ecrSvc == nil {
		m.ecrSvc = services.NewECR(cfg)
	}
	if m.elbSvc == nil {
		m.elbSvc = services.NewELB(cfg)
	}
	if m.iamSvc == nil {
		m.iamSvc = services.NewIAM(cfg)
	}
	if m.rdsSvc == nil {
		m.rdsSvc = services.NewRDS(cfg)
	}
	if m.kmsSvc == nil {
		m.kmsSvc = services.NewKMS(cfg)
	}
	if m.cfSvc == nil {
		m.cfSvc = services.NewCloudFront(cfg)
	}
	if m.lambdaSvc == nil {
		m.lambdaSvc = services.NewLambda(cfg)
	}
	if m.billingSvc == nil {
		m.billingSvc = services.NewBilling(cfg)
	}
}

func mainMenuItems() []string {
	return []string{
		"EC2", "SSM", "Secrets Manager", "S3",
		"ACM", "Route53", "EKS", "ECR",
		"ELB", "IAM", "RDS", "KMS",
		"CloudFront", "Lambda", "Billing", "Quit",
	}
}

func (m *model) sortVolumesBySize() {
	if len(m.ec2Volumes) == 0 {
		return
	}
	m.volumeSortDesc = !m.volumeSortDesc
	sort.SliceStable(m.ec2Volumes, func(i, j int) bool {
		if m.volumeSortDesc {
			return m.ec2Volumes[i].Size > m.ec2Volumes[j].Size
		}
		return m.ec2Volumes[i].Size < m.ec2Volumes[j].Size
	})
	m.items = make([]listEntry, len(m.ec2Volumes))
	for i, v := range m.ec2Volumes {
		m.items[i] = listEntry{
			Title: fmt.Sprintf("%-24s %-10s %s", v.ID, fmt.Sprintf("%dGiB", v.Size), v.State),
			ID:    v.ID,
		}
	}
	m.selected = 0
}

func (m *model) mergeDetailMap(dm map[string]string) {
	if len(dm) == 0 {
		return
	}
	if m.detailMap == nil {
		m.detailMap = make(map[string]string)
	}
	for k, v := range dm {
		m.detailMap[k] = v
	}
}

func (m *model) resetPage() {
	m.pageToken = ""
	m.hasMore = false
	m.filter = ""
	m.selected = 0
}

func (m *model) filteredItems() []listEntry {
	if m.filter == "" {
		return m.items
	}
	f := []rune(m.filter)
	var out []listEntry
	for _, e := range m.items {
		if matchFilter(e.Title, f) {
			out = append(out, e)
		}
	}
	return out
}

func (m *model) isRemoteSearchState() bool {
	switch m.kind {
	case stateProfile, stateRegion, stateBillingServiceCost:
		return false
	}
	return m.isListState()
}

func containsCI(s, substr string) bool {
	sr := []rune(strings.ToLower(s))
	fr := []rune(strings.ToLower(substr))
	if len(fr) == 0 {
		return true
	}
	for i := 0; i <= len(sr)-len(fr); i++ {
		match := true
		for j := range fr {
			if sr[i+j] != fr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func matchFilter(s string, filter []rune) bool {
	sr := []rune(s)
	j := 0
	for i := 0; i < len(sr) && j < len(filter); i++ {
		if sr[i] == filter[j] || (filter[j] >= 'a' && filter[j] <= 'z' && sr[i] == filter[j]-32) || (filter[j] >= 'A' && filter[j] <= 'Z' && sr[i] == filter[j]+32) {
			j++
		}
	}
	return j == len(filter)
}
