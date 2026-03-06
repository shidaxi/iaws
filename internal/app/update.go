package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/shidaxi/iaws/internal/config"
	ilog "github.com/shidaxi/iaws/internal/log"
)

func prettyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// clear loading on any data-arrival message
	switch msg.(type) {
	case tea.WindowSizeMsg, tea.KeyMsg, spinnerTickMsg, searchTickMsg:
	default:
		m.loading = false
	}

	switch msg := msg.(type) {
	case spinnerTickMsg:
		if m.loading {
			m.spinnerFrame++
			return m, spinnerTick()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "q" && !m.filterMode && !m.popupVisible && m.kind != stateSecretPut && m.kind != stateS3PutObject {
			return m, tea.Quit
		}
		if m.popupVisible {
			return m.handlePopupKey(msg)
		}
		// list state: dispatch to filter mode or normal key handler
		if m.isListState() {
			if m.filterMode {
				return m.handleFilterModeKey(msg)
			}
			return m.handleListKey(msg)
		}

		switch m.kind {
		case stateConfirm:
			if msg.String() == "y" || msg.String() == "Y" {
				return m.runConfirm()
			}
			return m.handleBack()
		case stateMessage:
			if msg.String() == "enter" || msg.String() == " " || msg.String() == "esc" {
				return m.handleBack()
			}
			return m, nil
		case stateSecretPut:
			if msg.String() == "enter" && len(m.inputValue) > 0 {
				m.confirmMsg = "Put value to secret " + m.secretName + "? (y/n)"
				m.confirmAction = "putSecret"
				m.confirmTarget = m.secretName
				m.confirmData = m.inputValue
				m.confirmBackState = stateSecretsList
				m.kind = stateConfirm
				return m, nil
			}
			if msg.String() == "esc" {
				return m.handleBack()
			}
			if len(msg.Runes) == 1 {
				r := msg.Runes[0]
				if r == 8 || r == 127 {
					if len(m.inputValue) > 0 {
						m.inputValue = m.inputValue[:len(m.inputValue)-1]
					}
					return m, nil
				}
				if r >= 32 && r < 127 {
					m.inputValue += string(r)
					return m, nil
				}
			}
			return m, nil
		case stateS3PutObject:
			if msg.String() == "esc" {
				return m.handleBack()
			}
			if msg.String() == "enter" {
				if m.s3PutStep == 0 {
					if m.s3PutKey != "" {
						m.s3PutStep = 1
					}
					return m, nil
				}
				if m.s3PutStep == 1 && m.s3LocalPath != "" {
					return m, putS3ObjectCmd(m, m.s3Bucket, m.s3PutKey, m.s3LocalPath)
				}
				return m, nil
			}
			if len(msg.Runes) == 1 {
				r := msg.Runes[0]
				if r == 8 || r == 127 {
					if m.s3PutStep == 0 && len(m.s3PutKey) > 0 {
						m.s3PutKey = m.s3PutKey[:len(m.s3PutKey)-1]
					}
					if m.s3PutStep == 1 && len(m.s3LocalPath) > 0 {
						m.s3LocalPath = m.s3LocalPath[:len(m.s3LocalPath)-1]
					}
					return m, nil
				}
				if r >= 32 && r < 127 {
					if m.s3PutStep == 0 {
						m.s3PutKey += string(r)
					} else {
						m.s3LocalPath += string(r)
					}
					return m, nil
				}
			}
			return m, nil
		}

		if m.isMenuState() {
			return m.handleMenuKey(msg)
		}
		return m, nil

	case searchTickMsg:
		if msg.seq == m.searchSeq && m.searchPending {
			m.searchPending = false
			m.searching = true
			m.pageToken = ""
			m.hasMore = false
			return m.setLoading(m.triggerSearchCmd())
		}
		return m, nil

	case profilesLoadedMsg:
		m.items = make([]listEntry, len(msg.profiles))
		for i, p := range msg.profiles {
			m.items[i] = listEntry{Title: p, ID: p}
		}
		m.kind = stateProfile
		m.selected = 0
		return m, nil

	case awsLoadedMsg:
		m.aws = msg.aws
		m.profile = msg.profile
		m.region = msg.region
		config.SaveRecentProfile(msg.profile)
		m.ensureClients()
		m.savedProfileItems = m.items
		preferred := m.aws.Config.Region
		regions := config.RegionsWithPreferredFirst(preferred)
		m.items = make([]listEntry, len(regions))
		for i, r := range regions {
			m.items[i] = listEntry{Title: r, ID: r}
		}
		m.kind = stateRegion
		m.filter = ""
		m.selected = 0
		return m, nil

	case regionSelectedMsg:
		m.region = msg.region
		m.aws.Config.Region = m.region
		m.ec2, m.ssm, m.secrets, m.s3, m.acm, m.r53 = nil, nil, nil, nil, nil, nil
		m.eksSvc, m.ecrSvc, m.elbSvc, m.iamSvc = nil, nil, nil, nil
		m.rdsSvc, m.kmsSvc, m.cfSvc, m.lambdaSvc, m.billingSvc = nil, nil, nil, nil, nil
		m.ensureClients()
		m.savedRegionItems = m.items
		m.kind = stateMainMenu
		m.menuItems = mainMenuItems()
		m.menuSelected = 0
		return m, nil

	// handle search results, clear search markers
	// --- paged list (generic) ---
	case pagedListMsg:
		m.searching = false
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			isNewState := msg.nextState != m.kind
			m.kind = msg.nextState
			if isNewState {
				m.filter = ""
				m.filterMode = false
			}
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	// --- paged instances (EC2/SSM login) ---
	case pagedInstancesMsg:
		m.searching = false
		entries := make([]listEntry, len(msg.items))
		for i, inst := range msg.items {
			title := fmt.Sprintf("%-20s %-12s %-14s %-30s %s", inst.ID, inst.State, inst.Type, inst.Name, inst.PublicIP)
			entries[i] = listEntry{Title: title, ID: inst.ID}
		}
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.ec2Instances = append(m.ec2Instances, msg.items...)
			m.items = append(m.items, entries...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.ec2Instances = msg.items
			m.items = entries
			isNewState := msg.nextState != m.kind
			m.kind = msg.nextState
			if isNewState {
				m.filter = ""
				m.filterMode = false
			}
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	// --- Route53 record sets (dual-token pagination) ---
	case r53RecordsMsg:
		m.searching = false
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			isNewState := msg.nextState != m.kind
			m.kind = msg.nextState
			if isNewState {
				m.filter = ""
				m.filterMode = false
			}
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextName != nil
		m.r53RecordNextType = msg.nextType
		if msg.nextName != nil {
			m.pageToken = *msg.nextName
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	// --- non-paged messages (VPC, subnet, SG, key pair, volume, AMI, S3 bucket) ---
	case listLoadedMsg:
		m.searching = false
		m.items = msg.items
		isNewState := msg.nextState != m.kind
		m.kind = msg.nextState
		if isNewState {
			m.filter = ""
			m.filterMode = false
		}
		m.selected = 0
		m.hasMore = false
		m.pageToken = ""
		m.searchPending = false
		m.resetSort()
		if len(msg.columns) > 0 {
			m.tableColumns = msg.columns
		}
		return m, nil

	case ec2InstancesLoadedMsg:
		m.searching = false
		m.ec2Instances = msg.items
		m.items = make([]listEntry, len(msg.items))
		for i, inst := range msg.items {
			title := fmt.Sprintf("%s  %s  %s  %s", inst.ID, inst.State, inst.Type, inst.Name)
			if inst.PublicIP != "" {
				title += "  " + inst.PublicIP
			}
			m.items[i] = listEntry{Title: title, ID: inst.ID}
		}
		if m.kind != stateEC2InstanceList {
			m.filter = ""
			m.filterMode = false
		}
		m.kind = stateEC2InstanceList
		m.selected = 0
		return m, nil

	case ssmParamsLoadedMsg:
		m.searching = false
		m.items = msg.items
		if m.kind != stateSSMParamList {
			m.filter = ""
			m.filterMode = false
		}
		m.kind = stateSSMParamList
		m.selected = 0
		return m, nil

	case secretValueLoadedMsg:
		m.msgText = prettyJSON(msg.value)
		m.msgErr = false
		m.prevStateAfterMessage = stateSecretsList
		m.kind = stateMessage
		return m, nil

	case ssmParamValueLoadedMsg:
		m.msgText = msg.value
		m.msgErr = false
		m.prevStateAfterMessage = stateSSMParamList
		m.kind = stateMessage
		return m, nil

	case detailLoadedMsg:
		m.msgText = msg.detail
		m.msgErr = false
		m.prevStateAfterMessage = msg.backState
		m.kind = stateMessage
		return m, nil

	case certDetailMsg:
		m.msgText = msg.detail
		m.msgErr = false
		m.prevStateAfterMessage = stateACMCertList
		m.kind = stateMessage
		return m, nil

	case s3BucketsLoadedMsg:
		m.searching = false
		m.items = msg.items
		m.kind = stateS3BucketList
		m.selected = 0
		m.hasMore = false
		m.pageToken = ""
		m.searchPending = false
		m.resetSort()
		return m, nil

	case s3ObjectsLoadedMsg:
		m.searching = false
		m.items = msg.items
		if m.kind != stateS3ObjectList {
			m.filter = ""
			m.filterMode = false
		}
		m.kind = stateS3ObjectList
		m.selected = 0
		return m, nil

	case ssmLoginInstancesLoadedMsg:
		m.searching = false
		m.ec2Instances = msg.items
		m.items = make([]listEntry, len(msg.items))
		for i, inst := range msg.items {
			m.items[i] = listEntry{
				Title: fmt.Sprintf("%s  %s  %s", inst.ID, inst.State, inst.Name),
				ID:    inst.ID,
			}
		}
		if m.kind != stateSSMLoginInstanceList {
			m.filter = ""
			m.filterMode = false
		}
		m.kind = stateSSMLoginInstanceList
		m.selected = 0
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.hasMore = true
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	case elbListLoadedMsg:
		m.searching = false
		m.mergeDetailMap(msg.detailMap)
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			if m.kind != stateELBList {
				m.filter = ""
				m.filterMode = false
			}
			m.kind = stateELBList
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	case iamListLoadedMsg:
		m.searching = false
		m.mergeDetailMap(msg.detailMap)
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			isNewState := msg.nextState != m.kind
			m.kind = msg.nextState
			if isNewState {
				m.filter = ""
				m.filterMode = false
			}
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	case rdsListLoadedMsg:
		m.searching = false
		m.mergeDetailMap(msg.detailMap)
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			if m.kind != stateRDSInstanceList {
				m.filter = ""
				m.filterMode = false
			}
			m.kind = stateRDSInstanceList
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	case cfListLoadedMsg:
		m.searching = false
		m.mergeDetailMap(msg.detailMap)
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			if m.kind != stateCloudFrontDistList {
				m.filter = ""
				m.filterMode = false
			}
			m.kind = stateCloudFrontDistList
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	case volumeListLoadedMsg:
		m.searching = false
		m.items = msg.items
		m.ec2Volumes = msg.volumes
		m.kind = stateEC2VolumeList
		m.selected = 0
		m.hasMore = false
		m.pageToken = ""
		m.searchPending = false
		m.resetSort()
		return m, nil

	case snapshotListLoadedMsg:
		m.searching = false
		m.items = msg.items
		m.ec2Snapshots = msg.snapshots
		m.kind = stateEC2SnapshotList
		m.selected = 0
		m.hasMore = false
		m.pageToken = ""
		m.searchPending = false
		m.resetSort()
		return m, nil

	case lambdaListLoadedMsg:
		m.searching = false
		m.mergeDetailMap(msg.detailMap)
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
			if m.sortColIdx >= 0 {
				m.sortItemsByColumn()
			}
		} else {
			m.items = msg.items
			if m.kind != stateLambdaFunctionList {
				m.filter = ""
				m.filterMode = false
			}
			m.kind = stateLambdaFunctionList
			m.selected = 0
			m.searchPending = false
			m.resetSort()
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	case billingResourceListMsg:
		m.items = msg.items
		if m.kind != stateBillingTopResources {
			m.filter = ""
			m.filterMode = false
		}
		m.kind = stateBillingTopResources
		m.selected = 0
		m.detailMap = nil
		m.mergeDetailMap(msg.detailMap)
		m.resetSort()
		if len(msg.columns) > 0 {
			m.tableColumns = msg.columns
		}
		return m, nil

	case billingCostMsg:
		m.msgText = msg.detail
		m.msgErr = false
		m.prevStateAfterMessage = msg.backState
		m.kind = stateMessage
		return m, nil

	case errMsg:
		m.searching = false
		ilog.Error("TUI error: %v", msg.err)
		m.setMessage(true, msg.err.Error())
		return m, nil

	case sessionDoneMsg:
		ilog.Info("TUI: SSM session ended")
		m.setMessage(false, "Session ended.")
		return m, nil

	case s3DownloadDoneMsg:
		m.prevStateAfterMessage = stateS3ObjectList
		m.setMessage(false, "Downloaded to "+msg.path)
		return m, nil

	case s3UploadDoneMsg:
		m.prevStateAfterMessage = stateS3Menu
		m.setMessage(false, "Uploaded "+msg.key+" to s3://"+msg.bucket+"/")
		return m, nil
	}

	return m, nil
}

// visibleItems returns items for display.
// In remote search states, no local filtering is applied — items are already server-filtered.
// In local filter states, fuzzy matching is applied.
func (m *model) visibleItems() []listEntry {
	if m.isRemoteSearchState() {
		return m.items
	}
	return m.filteredItems()
}

func (m *model) isListState() bool {
	switch m.kind {
	case stateProfile, stateRegion, stateEC2InstanceList, stateEC2VPCList, stateEC2SubnetList, stateEC2SGList, stateEC2KeyList, stateEC2VolumeList, stateEC2SnapshotList, stateEC2AMIList,
		stateSSMParamList, stateSSMLoginInstanceList, stateSecretsList, stateS3BucketList, stateS3ObjectList,
		stateACMCertList, stateRoute53ZoneList, stateRoute53RecordList,
		stateEKSClusterList, stateECRRepoList, stateECRImageList, stateELBList,
		stateIAMUserList, stateIAMRoleList, stateIAMPolicyList,
		stateRDSInstanceList, stateKMSKeyList, stateCloudFrontDistList, stateLambdaFunctionList,
		stateBillingServiceCost, stateBillingTopResources:
		return true
	}
	return false
}

func (m *model) isMenuState() bool {
	switch m.kind {
	case stateMainMenu, stateEC2Menu, stateSSMMenu, stateSecretsMenu, stateS3Menu,
		stateACMMenu, stateRoute53Menu, stateIAMMenu, stateBillingMenu:
		return true
	}
	return false
}

func (m *model) onMenuSelect(idx int) (tea.Model, tea.Cmd) {
	switch m.kind {
	case stateProfile:
		if idx < 0 || idx >= len(m.items) {
			return m, nil
		}
		profile := m.items[idx].ID
		return m.setLoading(loadAWSCmd(m.ctx, profile, ""))
	case stateMainMenu:
		switch idx {
		case 0:
			m.kind = stateEC2Menu
			m.menuItems = []string{"Instances", "VPCs", "Subnets", "Security Groups", "Key Pairs", "Volumes", "Snapshots", "AMIs", "Back"}
			m.menuSelected = 0
			return m, nil
		case 1:
			m.kind = stateSSMMenu
			m.menuItems = []string{"SSM Login EC2", "Parameter list", "Get parameter value", "Back"}
			m.menuSelected = 0
			return m, nil
		case 2:
			m.kind = stateSecretsMenu
			m.menuItems = []string{"List secrets", "Get secret value", "Put secret value", "Back"}
			m.menuSelected = 0
			return m, nil
		case 3:
			m.kind = stateS3Menu
			m.menuItems = []string{"List buckets", "List objects", "Download object", "Upload file", "Back"}
			m.menuSelected = 0
			return m, nil
		case 4:
			m.kind = stateACMMenu
			m.menuItems = []string{"List certificates", "Back"}
			m.menuSelected = 0
			return m, nil
		case 5:
			m.kind = stateRoute53Menu
			m.menuItems = []string{"List hosted zones", "Back"}
			m.menuSelected = 0
			return m, nil
		case 6: // EKS — go directly to cluster list
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(eksClusterListCmd(m, nil, false, ""))
		case 7: // ECR — go directly to repository list
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(ecrRepoListCmd(m, nil, false, ""))
		case 8: // ELB — go directly to load balancer list
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(elbListCmd(m, nil, false, ""))
		case 9: // IAM
			m.kind = stateIAMMenu
			m.menuItems = []string{"Users", "Roles", "Policies", "Back"}
			m.menuSelected = 0
			return m, nil
		case 10: // RDS
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(rdsInstanceListCmd(m, nil, false, ""))
		case 11: // KMS
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(kmsKeyListCmd(m, nil, false, ""))
		case 12: // CloudFront
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(cfDistListCmd(m, nil, false, ""))
		case 13: // Lambda
			m.resetPage()
			m.detailMap = nil
			return m.setLoading(lambdaFuncListCmd(m, nil, false, ""))
		case 14: // Billing
			m.kind = stateBillingMenu
			m.menuItems = billingMenuItems()
			m.menuSelected = 0
			return m, nil
		case 15:
			return m, tea.Quit
		}
		return m, nil
	case stateEC2Menu:
		if idx == 8 {
			return m.handleBack()
		}
		return m.runEC2Menu(idx)
	case stateSSMMenu:
		if idx == 3 {
			return m.handleBack()
		}
		return m.runSSMMenu(idx)
	case stateSecretsMenu:
		if idx == 3 {
			return m.handleBack()
		}
		return m.runSecretsMenu(idx)
	case stateS3Menu:
		if idx == 4 {
			return m.handleBack()
		}
		if idx == 3 {
			return m.setLoading(s3BucketsCmd(m, ""))
		}
		return m.runS3Menu(idx)
	case stateACMMenu:
		if idx == 1 {
			return m.handleBack()
		}
		return m.runACMMenu(idx)
	case stateRoute53Menu:
		if idx == 1 {
			return m.handleBack()
		}
		return m.runRoute53Menu(idx)
	case stateIAMMenu:
		if idx == 3 {
			return m.handleBack()
		}
		return m.runIAMMenu(idx)
	case stateBillingMenu:
		if idx == 5 {
			return m.handleBack()
		}
		return m.runBillingMenu(idx)
	}
	return m, nil
}

func (m *model) onListSelect(entry listEntry) (tea.Model, tea.Cmd) {
	if entry.ID == loadMoreID {
		return m.setLoading(m.loadMoreCmd())
	}

	switch m.kind {
	case stateProfile:
		return m.setLoading(loadAWSCmd(m.ctx, entry.ID, ""))
	case stateRegion:
		return m, func() tea.Msg { return regionSelectedMsg{region: entry.ID} }
	case stateEC2InstanceList:
		m.popupVisible = true
		m.popupItems = []string{"Start", "Stop", "Reboot", "Terminate"}
		m.popupSelected = 0
		m.popupTarget = entry.ID
		m.popupAction = "ec2-instance"
		return m, nil
	case stateSSMParamList:
		m.ssmParamName = entry.ID
		return m.setLoading(getSSMParamCmd(m, entry.ID))
	case stateSSMLoginInstanceList:
		return m, startSSMSessionCmd(m, entry.ID)
	case stateSecretsList:
		m.secretName = entry.ID
		if m.prevSecretAction == "get" {
			return m.setLoading(getSecretValueCmd(m, entry.ID))
		}
		m.kind = stateSecretPut
		m.secretName = entry.ID
		return m, nil
	case stateS3BucketList:
		m.s3Bucket = entry.ID
		m.s3Prefix = ""
		if m.s3MenuUpload {
			m.s3MenuUpload = false
			m.kind = stateS3PutObject
			m.s3PutKey = ""
			m.s3LocalPath = ""
			m.s3PutStep = 0
			return m, nil
		}
		m.resetPage()
		return m.setLoading(listS3ObjectsCmd(m, entry.ID, "", nil, false, ""))
	case stateS3ObjectList:
		if entry.ID == ".." {
			m.s3Prefix = parentPrefix(m.s3Prefix)
			m.resetPage()
			return m.setLoading(listS3ObjectsCmd(m, m.s3Bucket, m.s3Prefix, nil, false, ""))
		}
		if entry.IsDir {
			m.resetPage()
			return m.setLoading(listS3ObjectsCmd(m, m.s3Bucket, entry.ID, nil, false, ""))
		}
		m.s3Key = entry.ID
		return m.setLoading(getS3ObjectCmd(m, m.s3Bucket, entry.ID, entry.ID))
	case stateACMCertList:
		return m.setLoading(acmDescribeCmd(m, entry.ID))
	case stateRoute53ZoneList:
		parts := strings.SplitN(entry.ID, "|", 2)
		m.r53ZoneID = parts[0]
		if len(parts) > 1 {
			m.r53ZoneName = parts[1]
		}
		m.resetPage()
		return m.setLoading(r53RecordsCmd(m, m.r53ZoneID, nil, nil, false, ""))
	case stateEKSClusterList:
		return m.setLoading(eksDescribeClusterCmd(m, entry.ID))
	case stateECRRepoList:
		m.ecrRepoName = entry.ID
		m.resetPage()
		m.detailMap = nil
		return m.setLoading(ecrImageListCmd(m, entry.ID, nil, false, ""))
	case stateELBList:
		if d, ok := m.detailMap[entry.ID]; ok {
			m.msgText = d
			m.msgErr = false
			m.prevStateAfterMessage = stateELBList
			m.kind = stateMessage
		}
		return m, nil
	case stateIAMUserList, stateIAMRoleList, stateIAMPolicyList:
		if d, ok := m.detailMap[entry.ID]; ok {
			m.msgText = d
			m.msgErr = false
			m.prevStateAfterMessage = m.kind
			m.kind = stateMessage
		}
		return m, nil
	case stateRDSInstanceList:
		if d, ok := m.detailMap[entry.ID]; ok {
			m.msgText = d
			m.msgErr = false
			m.prevStateAfterMessage = stateRDSInstanceList
			m.kind = stateMessage
		}
		return m, nil
	case stateKMSKeyList:
		return m.setLoading(kmsDescribeKeyCmd(m, entry.ID))
	case stateCloudFrontDistList:
		if d, ok := m.detailMap[entry.ID]; ok {
			m.msgText = d
			m.msgErr = false
			m.prevStateAfterMessage = stateCloudFrontDistList
			m.kind = stateMessage
		}
		return m, nil
	case stateLambdaFunctionList:
		if d, ok := m.detailMap[entry.ID]; ok {
			m.msgText = d
			m.msgErr = false
			m.prevStateAfterMessage = stateLambdaFunctionList
			m.kind = stateMessage
		}
		return m, nil
	case stateBillingServiceCost:
		return m.setLoading(billingServiceDetailCmd(m, entry.ID))
	case stateBillingTopResources:
		if d, ok := m.detailMap[entry.ID]; ok {
			m.msgText = d
			m.msgErr = false
			m.prevStateAfterMessage = stateBillingTopResources
			m.kind = stateMessage
		}
		return m, nil
	}
	return m, nil
}

func parentPrefix(p string) string {
	if p == "" {
		return ""
	}
	for i := len(p) - 2; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i+1]
		}
	}
	return ""
}
