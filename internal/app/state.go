package app

import (
	"context"
	"sort"
	"strconv"
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
	stateEC2SnapshotList
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
	stateBillingTopResources
	stateConfirm
	stateMessage // show error or success then return
)

const loadMoreID = "__load_more__"

type listEntry struct {
	Title string
	ID    string // instance ID, secret name, bucket name, etc.
	IsDir bool   // S3 directory prefix
}

// Model is the root Bubble Tea model.
type Model struct {
	ctx context.Context

	kind   stateKind
	width  int // terminal width
	aws    *config.AWS
	profile string
	region  string

	// list state: entries and filter
	items      []listEntry
	filter     string
	filterMode bool // true = actively typing filter keywords
	selected   int
	// saved profile items restored when going back from region to profile
	savedProfileItems []listEntry
	// saved region items restored when going back from main menu to region
	savedRegionItems []listEntry
	// EC2 instance list needs full InstanceItem for display; reuses items title, ID is instance ID
	ec2Instances []services.InstanceItem
	// EC2 volume/snapshot raw data
	ec2Volumes   []services.VolumeItem
	ec2Snapshots []services.SnapshotItem

	// generic column sorting
	tableColumns []columnDef
	sortColIdx   int  // highlighted column (-1 = none)
	sortDesc     bool // sort direction
	sorted       bool // true after pressing 's' (actively sorted)

	// menu: items and selected index
	menuItems   []string
	menuSelected int

	// confirm
	confirmMsg      string
	confirmAction   string
	confirmTarget   string
	confirmData     string // extra data, e.g. secret value
	confirmBackState stateKind

	// message (error/success)
	msgText              string
	msgErr               bool
	prevStateAfterMessage stateKind

	// SSM: parameter name for get
	ssmParamName string
	// Secrets: secret name for get/put
	secretName   string
	secretValue  string
	// S3: bucket, prefix, key, local path
	s3Bucket    string
	s3Prefix    string
	s3Key       string
	s3LocalPath string
	s3PutKey    string   // used during upload flow
	s3PutStep   int      // 0=key, 1=path

	// Secrets: whether list was entered via "get" or "put"
	prevSecretAction string
	// optional text input for setting secret value
	inputValue string

	// S3 menu: true when selecting bucket after choosing "Upload file"
	s3MenuUpload bool

	// server-side pagination
	pageToken string // next page token for current list
	hasMore   bool   // whether there are more pages

	// remote search (debounced)
	searchSeq     int  // incremented per keystroke; used to discard stale timer callbacks
	searchPending bool // filter changed since last search; Enter triggers search
	searching     bool // search request in progress

	// Route53
	r53ZoneID          string
	r53ZoneName        string
	r53RecordNextType  *string

	// ECR: selected repository for image list
	ecrRepoName string

	// detail cache: keyed by entry.ID, stores formatted detail strings
	detailMap map[string]string

	// service clients (created after AWS config loaded)
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

// model is a backward-compatible alias within this package.
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

func billingMenuItems() []string {
	return []string{
		"Monthly cost (6 months)",
		"Cost by service (this month)",
		"Daily cost (30 days)",
		"Top resources (this month)",
		"Cost optimization",
		"Back",
	}
}

// ─── column-based table headers and sorting ────────────────────────────────────

type columnDef struct {
	Name  string
	Width int // -1 = last column (variable width)
}

func columnsForState(kind stateKind) []columnDef {
	switch kind {
	case stateEC2InstanceList, stateSSMLoginInstanceList:
		return []columnDef{{"ID", 20}, {"State", 12}, {"Type", 14}, {"Name", 30}, {"PublicIP", -1}}
	case stateEC2VPCList:
		return []columnDef{{"ID", 24}, {"CIDR", 20}, {"State", -1}}
	case stateEC2SubnetList:
		return []columnDef{{"ID", 26}, {"CIDR", 20}, {"AZ", -1}}
	case stateEC2SGList:
		return []columnDef{{"ID", 24}, {"Name", -1}}
	case stateEC2VolumeList:
		return []columnDef{{"ID", 24}, {"Type", 8}, {"Size", 10}, {"State", -1}}
	case stateEC2SnapshotList:
		return []columnDef{{"ID", 24}, {"Volume", 24}, {"Size", 10}, {"State", 12}, {"Time", -1}}
	case stateEC2AMIList:
		return []columnDef{{"ID", 24}, {"Name", -1}}
	case stateSSMParamList:
		return []columnDef{{"Name", 50}, {"Type", -1}}
	case stateACMCertList:
		return []columnDef{{"Domain", 40}, {"Type", 16}, {"Status", -1}}
	case stateRoute53ZoneList:
		return []columnDef{{"Name", 40}, {"Type", 8}, {"Records", -1}}
	case stateRoute53RecordList:
		return []columnDef{{"Name", 40}, {"Type", 8}, {"TTL", 10}, {"Values", -1}}
	case stateECRRepoList:
		return []columnDef{{"Name", 40}, {"URI", -1}}
	case stateELBList:
		return []columnDef{{"Name", 32}, {"Type", 14}, {"Scheme", 18}, {"State", -1}}
	case stateIAMUserList:
		return []columnDef{{"Name", 30}, {"Created", -1}}
	case stateIAMRoleList:
		return []columnDef{{"Name", 40}, {"Description", -1}}
	case stateIAMPolicyList:
		return []columnDef{{"Name", 40}, {"Attached", -1}}
	case stateRDSInstanceList:
		return []columnDef{{"ID", 30}, {"Engine", 20}, {"Class", 16}, {"Storage", 10}, {"Status", -1}}
	case stateKMSKeyList:
		return []columnDef{{"Alias", 35}, {"ID", -1}}
	case stateCloudFrontDistList:
		return []columnDef{{"ID", 16}, {"Domain", 40}, {"Status", 10}, {"Aliases", -1}}
	case stateLambdaFunctionList:
		return []columnDef{{"Name", 40}, {"Runtime", 14}, {"Memory", -1}}
	}
	return nil
}

func (m *model) getTableColumns() []columnDef {
	if len(m.tableColumns) > 0 {
		return m.tableColumns
	}
	return columnsForState(m.kind)
}

func (m *model) resetSort() {
	m.sortColIdx = -1
	m.sortDesc = true // so the first 's' press toggles to false = ascending
	m.sorted = false
	m.tableColumns = nil
}

func (m *model) sortItemsByColumn() {
	cols := m.getTableColumns()
	if m.sortColIdx < 0 || m.sortColIdx >= len(cols) || len(m.items) == 0 {
		return
	}
	idx := m.sortColIdx
	desc := m.sortDesc
	sort.SliceStable(m.items, func(i, j int) bool {
		vi := extractColumn(m.items[i].Title, cols, idx)
		vj := extractColumn(m.items[j].Title, cols, idx)
		ni, oki := parseNumericValue(vi)
		nj, okj := parseNumericValue(vj)
		if oki && okj {
			if desc {
				return ni > nj
			}
			return ni < nj
		}
		if desc {
			return vi > vj
		}
		return vi < vj
	})
	m.selected = 0
}

func extractColumn(title string, cols []columnDef, colIdx int) string {
	if colIdx >= len(cols) {
		return ""
	}
	runes := []rune(title)
	start := 0
	for i := 0; i < colIdx; i++ {
		w := cols[i].Width
		if w < 0 {
			return ""
		}
		start += w + 1
	}
	if start >= len(runes) {
		return ""
	}
	col := cols[colIdx]
	if col.Width < 0 || start+col.Width > len(runes) {
		return strings.TrimSpace(string(runes[start:]))
	}
	return strings.TrimSpace(string(runes[start : start+col.Width]))
}

func parseNumericValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	for _, suffix := range []string{"GiB", "GB", "MB", "KB", " records", "ms"} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.TrimPrefix(s, "attached:")
	s = strings.TrimPrefix(s, "TTL:")
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil
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
	case stateProfile, stateRegion, stateBillingServiceCost, stateBillingTopResources:
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
