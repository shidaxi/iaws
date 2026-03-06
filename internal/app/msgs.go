package app

import (
	"github.com/shidaxi/iaws/internal/config"
	"github.com/shidaxi/iaws/internal/services"
)

type profilesLoadedMsg struct {
	profiles []string
}

type awsLoadedMsg struct {
	aws     *config.AWS
	profile string
	region  string
}

type regionSelectedMsg struct {
	region string
}

// pagedListMsg is the generic message for server-side paged list results.
type pagedListMsg struct {
	items     []listEntry
	nextToken *string // nil means no more pages
	nextState stateKind
	append    bool // true = append to existing items (load more)
}

// pagedInstancesMsg is for EC2 instances (needs InstanceItem for detail display)
type pagedInstancesMsg struct {
	items     []services.InstanceItem
	nextToken *string
	nextState stateKind
	append    bool
}

type listLoadedMsg struct {
	items     []listEntry
	nextState stateKind
	columns   []columnDef
}

type ec2InstancesLoadedMsg struct {
	items []services.InstanceItem
}

type ssmParamsLoadedMsg struct {
	items []listEntry
}

type secretValueLoadedMsg struct {
	value string
}

type ssmParamValueLoadedMsg struct {
	value string
}

type s3BucketsLoadedMsg struct {
	items []listEntry
}

type s3ObjectsLoadedMsg struct {
	items []listEntry
}

type ssmLoginInstancesLoadedMsg struct {
	items     []services.InstanceItem
	nextToken *string
}

type s3DownloadDoneMsg struct {
	path string
}

type s3UploadDoneMsg struct {
	bucket string
	key    string
}

type certDetailMsg struct {
	detail string
}

type searchTickMsg struct {
	seq int
}

type volumeListLoadedMsg struct {
	items   []listEntry
	volumes []services.VolumeItem
}

type snapshotListLoadedMsg struct {
	items     []listEntry
	snapshots []services.SnapshotItem
}

type detailLoadedMsg struct {
	detail    string
	backState stateKind
}

type elbListLoadedMsg struct {
	items     []listEntry
	nextToken *string
	append    bool
	detailMap map[string]string
}

type iamListLoadedMsg struct {
	items     []listEntry
	nextToken *string
	nextState stateKind
	append    bool
	detailMap map[string]string
}

type rdsListLoadedMsg struct {
	items     []listEntry
	nextToken *string
	append    bool
	detailMap map[string]string
}

type cfListLoadedMsg struct {
	items     []listEntry
	nextToken *string
	append    bool
	detailMap map[string]string
}

type lambdaListLoadedMsg struct {
	items     []listEntry
	nextToken *string
	append    bool
	detailMap map[string]string
}

type billingCostMsg struct {
	detail    string
	backState stateKind
}

type billingResourceListMsg struct {
	items     []listEntry
	detailMap map[string]string
	columns   []columnDef
}

// Route53 record set pagination requires two tokens
type r53RecordsMsg struct {
	items         []listEntry
	nextName      *string
	nextType      *string
	nextState     stateKind
	append        bool
}
