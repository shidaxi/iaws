package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.kind != stateConfirm && m.kind != stateMessage {
			if m.isListState() {
				if m.isRemoteSearchState() {
					return m.handleRemoteSearchKey(msg)
				}
				// local filter for profile/region
				if msg.String() == "backspace" || (len(msg.Runes) == 1 && (msg.Runes[0] == 8 || msg.Runes[0] == 127)) {
					if len(m.filter) > 0 {
						m.filter = m.filter[:len(m.filter)-1]
						m.selected = 0
					}
					return m, nil
				}
				if len(msg.Runes) == 1 {
					r := msg.Runes[0]
					if r >= 32 && r < 127 {
						m.filter += string(r)
						m.selected = 0
						return m, nil
					}
				}
			}
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

		if m.isListState() {
			return m.handleListKey(msg)
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
			return m, m.triggerSearchCmd()
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
		m.rdsSvc, m.kmsSvc, m.cfSvc, m.lambdaSvc = nil, nil, nil, nil
		m.ensureClients()
		m.savedRegionItems = m.items
		m.kind = stateMainMenu
		m.menuItems = mainMenuItems()
		m.menuSelected = 0
		return m, nil

	// handle search results clearing searching flag
	// --- paginated list (generic) ---
	case pagedListMsg:
		m.searching = false
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
		} else {
			m.items = msg.items
			m.kind = msg.nextState
			m.filter = ""
			m.selected = 0
			m.searchPending = false
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	// --- paginated instances (EC2/SSM login) ---
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
		} else {
			m.ec2Instances = msg.items
			m.items = entries
			m.kind = msg.nextState
			m.filter = ""
			m.selected = 0
			m.searchPending = false
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	// --- Route53 record sets (two tokens) ---
	case r53RecordsMsg:
		m.searching = false
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
		} else {
			m.items = msg.items
			m.kind = msg.nextState
			m.filter = ""
			m.selected = 0
			m.searchPending = false
		}
		m.pageToken = ""
		m.hasMore = msg.nextName != nil
		m.r53RecordNextType = msg.nextType
		if msg.nextName != nil {
			m.pageToken = *msg.nextName
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
		return m, nil

	// --- legacy non-paged messages (VPC, Subnet, SG, KeyPair, Volume, AMI, S3 buckets) ---
	case listLoadedMsg:
		m.searching = false
		m.items = msg.items
		m.kind = msg.nextState
		m.selected = 0
		m.hasMore = false
		m.pageToken = ""
		m.searchPending = false
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
		m.kind = stateEC2InstanceList
		m.filter = ""
		m.selected = 0
		return m, nil

	case ssmParamsLoadedMsg:
		m.searching = false
		m.items = msg.items
		m.kind = stateSSMParamList
		m.filter = ""
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
		return m, nil

	case s3ObjectsLoadedMsg:
		m.searching = false
		m.items = msg.items
		m.kind = stateS3ObjectList
		m.filter = ""
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
		m.kind = stateSSMLoginInstanceList
		m.filter = ""
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
		} else {
			m.items = msg.items
			m.kind = stateELBList
			m.filter = ""
			m.selected = 0
			m.searchPending = false
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
		} else {
			m.items = msg.items
			m.kind = msg.nextState
			m.filter = ""
			m.selected = 0
			m.searchPending = false
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
		} else {
			m.items = msg.items
			m.kind = stateRDSInstanceList
			m.filter = ""
			m.selected = 0
			m.searchPending = false
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
		} else {
			m.items = msg.items
			m.kind = stateCloudFrontDistList
			m.filter = ""
			m.selected = 0
			m.searchPending = false
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
		m.volumeSortDesc = false
		m.kind = stateEC2VolumeList
		m.selected = 0
		m.hasMore = false
		m.pageToken = ""
		m.searchPending = false
		return m, nil

	case lambdaListLoadedMsg:
		m.searching = false
		m.mergeDetailMap(msg.detailMap)
		if msg.append {
			if len(m.items) > 0 && m.items[len(m.items)-1].ID == loadMoreID {
				m.items = m.items[:len(m.items)-1]
			}
			m.items = append(m.items, msg.items...)
		} else {
			m.items = msg.items
			m.kind = stateLambdaFunctionList
			m.filter = ""
			m.selected = 0
			m.searchPending = false
		}
		m.pageToken = ""
		m.hasMore = msg.nextToken != nil
		if msg.nextToken != nil {
			m.pageToken = *msg.nextToken
			m.items = append(m.items, listEntry{Title: "→ Load more...", ID: loadMoreID})
		}
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

// handleRemoteSearchKey handles keyboard input for remote-searchable list states.
// Printable chars and backspace update m.filter and start a 2s debounce timer.
// Enter triggers immediate search if pending; otherwise selects the item.
// Esc clears filter (if non-empty) or goes back.
func (m *model) handleRemoteSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "backspace" || (len(msg.Runes) == 1 && (msg.Runes[0] == 8 || msg.Runes[0] == 127)):
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			if m.filter == "" {
				m.searchPending = false
				m.searching = true
				m.pageToken = ""
				m.hasMore = false
				m.selected = 0
				return m, m.triggerSearchCmd()
			}
			m.searchPending = true
			m.searchSeq++
			seq := m.searchSeq
			m.selected = 0
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return searchTickMsg{seq: seq}
			})
		}
		return m, nil

	case len(msg.Runes) == 1 && msg.Runes[0] >= 32 && msg.Runes[0] < 127:
		m.filter += string(msg.Runes[0])
		m.searchPending = true
		m.searchSeq++
		seq := m.searchSeq
		m.selected = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return searchTickMsg{seq: seq}
		})

	case msg.String() == "enter":
		if m.searchPending {
			m.searchPending = false
			m.searching = true
			m.pageToken = ""
			m.hasMore = false
			return m, m.triggerSearchCmd()
		}
		return m.handleListEnter()

	case msg.String() == "up" || msg.String() == "k":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil

	case msg.String() == "down" || msg.String() == "j":
		items := m.visibleItems()
		if m.selected < len(items)-1 {
			m.selected++
		}
		return m, nil

	case msg.String() == "alt+s":
		if m.kind == stateEC2VolumeList {
			m.sortVolumesBySize()
			return m, nil
		}
		return m, nil

	case msg.String() == "esc":
		if m.filter != "" {
			m.filter = ""
			m.searchPending = false
			m.searching = true
			m.pageToken = ""
			m.hasMore = false
			m.selected = 0
			return m, m.triggerSearchCmd()
		}
		return m.handleBack()
	}
	return m, nil
}

// visibleItems returns items for display.
// For remote-searchable states, no local filtering — items are already filtered server-side.
// For local-filter states, apply fuzzy match.
func (m *model) visibleItems() []listEntry {
	if m.isRemoteSearchState() {
		return m.items
	}
	return m.filteredItems()
}

func (m *model) isListState() bool {
	switch m.kind {
	case stateProfile, stateRegion, stateEC2InstanceList, stateEC2VPCList, stateEC2SubnetList, stateEC2SGList, stateEC2KeyList, stateEC2VolumeList, stateEC2AMIList,
		stateSSMParamList, stateSSMLoginInstanceList, stateSecretsList, stateS3BucketList, stateS3ObjectList,
		stateACMCertList, stateRoute53ZoneList, stateRoute53RecordList,
		stateEKSClusterList, stateECRRepoList, stateECRImageList, stateELBList,
		stateIAMUserList, stateIAMRoleList, stateIAMPolicyList,
		stateRDSInstanceList, stateKMSKeyList, stateCloudFrontDistList, stateLambdaFunctionList:
		return true
	}
	return false
}

func (m *model) isMenuState() bool {
	switch m.kind {
	case stateMainMenu, stateEC2Menu, stateEC2InstanceAction, stateSSMMenu, stateSecretsMenu, stateS3Menu,
		stateACMMenu, stateRoute53Menu, stateIAMMenu:
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
		return m, loadAWSCmd(m.ctx, profile, "")
	case stateMainMenu:
		switch idx {
		case 0:
			m.kind = stateEC2Menu
			m.menuItems = []string{"Instances", "VPCs", "Subnets", "Security Groups", "Key Pairs", "Volumes", "AMIs", "Back"}
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
		case 6: // EKS — direct to cluster list
			m.resetPage()
			m.detailMap = nil
			return m, eksClusterListCmd(m, nil, false, "")
		case 7: // ECR — direct to repo list
			m.resetPage()
			m.detailMap = nil
			return m, ecrRepoListCmd(m, nil, false, "")
		case 8: // ELB — direct to LB list
			m.resetPage()
			m.detailMap = nil
			return m, elbListCmd(m, nil, false, "")
		case 9: // IAM
			m.kind = stateIAMMenu
			m.menuItems = []string{"Users", "Roles", "Policies", "Back"}
			m.menuSelected = 0
			return m, nil
		case 10: // RDS
			m.resetPage()
			m.detailMap = nil
			return m, rdsInstanceListCmd(m, nil, false, "")
		case 11: // KMS
			m.resetPage()
			m.detailMap = nil
			return m, kmsKeyListCmd(m, nil, false, "")
		case 12: // CloudFront
			m.resetPage()
			m.detailMap = nil
			return m, cfDistListCmd(m, nil, false, "")
		case 13: // Lambda
			m.resetPage()
			m.detailMap = nil
			return m, lambdaFuncListCmd(m, nil, false, "")
		case 14:
			return m, tea.Quit
		}
		return m, nil
	case stateEC2Menu:
		if idx == 7 {
			return m.handleBack()
		}
		return m.runEC2Menu(idx)
	case stateEC2InstanceAction:
		return m.onEC2InstanceActionSelect(idx)
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
			return m, s3BucketsCmd(m, "")
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
	}
	return m, nil
}

func (m *model) onListSelect(entry listEntry) (tea.Model, tea.Cmd) {
	if entry.ID == loadMoreID {
		return m, m.loadMoreCmd()
	}

	switch m.kind {
	case stateProfile:
		return m, loadAWSCmd(m.ctx, entry.ID, "")
	case stateRegion:
		return m, func() tea.Msg { return regionSelectedMsg{region: entry.ID} }
	case stateEC2InstanceList:
		m.confirmTarget = entry.ID
		m.menuItems = []string{"Start", "Stop", "Reboot"}
		m.menuSelected = 0
		m.kind = stateEC2InstanceAction
		return m, nil
	case stateSSMParamList:
		m.ssmParamName = entry.ID
		return m, getSSMParamCmd(m, entry.ID)
	case stateSSMLoginInstanceList:
		return m, startSSMSessionCmd(m, entry.ID)
	case stateSecretsList:
		m.secretName = entry.ID
		if m.prevSecretAction == "get" {
			return m, getSecretValueCmd(m, entry.ID)
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
		return m, listS3ObjectsCmd(m, entry.ID, "", nil, false, "")
	case stateS3ObjectList:
		if entry.ID == ".." {
			m.s3Prefix = parentPrefix(m.s3Prefix)
			m.resetPage()
			return m, listS3ObjectsCmd(m, m.s3Bucket, m.s3Prefix, nil, false, "")
		}
		if entry.IsDir {
			m.resetPage()
			return m, listS3ObjectsCmd(m, m.s3Bucket, entry.ID, nil, false, "")
		}
		m.s3Key = entry.ID
		return m, getS3ObjectCmd(m, m.s3Bucket, entry.ID, entry.ID)
	case stateACMCertList:
		return m, acmDescribeCmd(m, entry.ID)
	case stateRoute53ZoneList:
		parts := strings.SplitN(entry.ID, "|", 2)
		m.r53ZoneID = parts[0]
		if len(parts) > 1 {
			m.r53ZoneName = parts[1]
		}
		m.resetPage()
		return m, r53RecordsCmd(m, m.r53ZoneID, nil, nil, false, "")
	case stateEKSClusterList:
		return m, eksDescribeClusterCmd(m, entry.ID)
	case stateECRRepoList:
		m.ecrRepoName = entry.ID
		m.resetPage()
		m.detailMap = nil
		return m, ecrImageListCmd(m, entry.ID, nil, false, "")
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
		return m, kmsDescribeKeyCmd(m, entry.ID)
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
