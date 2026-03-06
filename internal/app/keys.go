package app

import (
	"time"

	"github.com/charmbracelet/bubbletea"
)

// handleListKey handles keys when NOT in filter mode:
// / = enter filter mode, tab = cycle column, s = toggle sort,
// up/down/enter/esc = navigation.
func (m *model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "/":
		m.filterMode = true
		return m, nil
	case "tab":
		cols := m.getTableColumns()
		if len(cols) > 0 {
			if m.sortColIdx < 0 {
				m.sortColIdx = 0
			} else {
				m.sortColIdx = (m.sortColIdx + 1) % len(cols)
			}
			m.sortDesc = true
		}
		return m, nil
	case "s":
		cols := m.getTableColumns()
		if len(cols) > 0 {
			if m.sortColIdx < 0 {
				m.sortColIdx = 0
			}
			m.sortDesc = !m.sortDesc
			m.sorted = true
			m.sortItemsByColumn()
		}
		return m, nil
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case "down", "j":
		items := m.visibleItems()
		if m.selected < len(items)-1 {
			m.selected++
		}
		return m, nil
	case "enter":
		return m.handleListEnter()
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.selected = 0
			if m.isRemoteSearchState() {
				m.searchPending = false
				m.searching = true
				m.pageToken = ""
				m.hasMore = false
				return m, m.triggerSearchCmd()
			}
			return m, nil
		}
		return m.handleBack()
	}
	return m, nil
}

// handleFilterModeKey handles keys when in filter mode:
// typing updates filter, Enter exits, Esc clears + exits.
func (m *model) handleFilterModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "enter":
		m.filterMode = false
		if m.isRemoteSearchState() && m.searchPending {
			m.searchPending = false
			m.searching = true
			m.pageToken = ""
			m.hasMore = false
			return m, m.triggerSearchCmd()
		}
		return m, nil
	case msg.String() == "esc":
		m.filterMode = false
		if m.filter != "" {
			m.filter = ""
			m.selected = 0
			if m.isRemoteSearchState() {
				m.searchPending = false
				m.searching = true
				m.pageToken = ""
				m.hasMore = false
				return m, m.triggerSearchCmd()
			}
		}
		return m, nil
	case msg.String() == "backspace" || (len(msg.Runes) == 1 && (msg.Runes[0] == 8 || msg.Runes[0] == 127)):
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.selected = 0
			if m.isRemoteSearchState() {
				m.searchPending = true
				m.searchSeq++
				seq := m.searchSeq
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return searchTickMsg{seq: seq}
				})
			}
		}
		return m, nil
	case len(msg.Runes) == 1 && msg.Runes[0] >= 32 && msg.Runes[0] < 127:
		m.filter += string(msg.Runes[0])
		m.selected = 0
		if m.isRemoteSearchState() {
			m.searchPending = true
			m.searchSeq++
			seq := m.searchSeq
			return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return searchTickMsg{seq: seq}
			})
		}
		return m, nil
	}
	return m, nil
}

func (m *model) handleListEnter() (tea.Model, tea.Cmd) {
	items := m.visibleItems()
	if len(items) == 0 {
		return m, nil
	}
	idx := m.selected
	if idx < 0 || idx >= len(items) {
		return m, nil
	}
	entry := items[idx]
	return m.onListSelect(entry)
}

func (m *model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuSelected > 0 {
			m.menuSelected--
		}
		return m, nil
	case "down", "j":
		if m.menuSelected < len(m.menuItems)-1 {
			m.menuSelected++
		}
		return m, nil
	case "enter":
		return m.onMenuSelect(m.menuSelected)
	case "esc":
		return m.handleBack()
	}
	return m, nil
}

func (m *model) handleBack() (tea.Model, tea.Cmd) {
	m.resetSort()
	m.filterMode = false
	switch m.kind {
	case stateProfile:
		return m, tea.Quit
	case stateRegion:
		m.kind = stateProfile
		m.items = m.savedProfileItems
		m.selected = 0
		m.filter = ""
		return m, nil
	case stateMainMenu:
		m.kind = stateRegion
		m.items = m.savedRegionItems
		m.selected = 0
		m.filter = ""
		return m, nil
	case stateEC2Menu, stateSSMMenu, stateSecretsMenu, stateS3Menu:
		m.kind = stateMainMenu
		m.menuItems = mainMenuItems()
		m.menuSelected = 0
		return m, nil
	case stateEC2InstanceList, stateEC2VPCList, stateEC2SubnetList, stateEC2SGList, stateEC2KeyList, stateEC2VolumeList, stateEC2SnapshotList, stateEC2AMIList:
		m.kind = stateEC2Menu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateEC2InstanceAction:
		m.kind = stateEC2InstanceList
		m.selected = 0
		return m, nil
	case stateSSMParamList, stateSSMLoginInstanceList:
		m.kind = stateSSMMenu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateSSMParamGet:
		m.kind = stateSSMParamList
		m.selected = 0
		return m, nil
	case stateSecretsList:
		m.kind = stateSecretsMenu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateSecretGet:
		m.kind = stateSecretsList
		m.selected = 0
		return m, nil
	case stateSecretPut:
		m.kind = stateSecretsList
		m.selected = 0
		m.inputValue = ""
		return m, nil
	case stateS3BucketList:
		m.kind = stateS3Menu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateS3ObjectList, stateS3GetObject:
		m.kind = stateS3BucketList
		m.s3Prefix = ""
		m.s3Key = ""
		m.items = nil
		m.selected = 0
		return m, nil
	case stateACMMenu, stateRoute53Menu, stateIAMMenu, stateBillingMenu:
		m.kind = stateMainMenu
		m.menuItems = mainMenuItems()
		m.menuSelected = 0
		return m, nil
	case stateBillingServiceCost, stateBillingTopResources:
		m.kind = stateBillingMenu
		m.menuItems = billingMenuItems()
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.detailMap = nil
		return m, nil
	case stateACMCertList, stateACMCertDetail:
		m.kind = stateACMMenu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateRoute53ZoneList:
		m.kind = stateRoute53Menu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateRoute53RecordList:
		m.kind = stateRoute53ZoneList
		m.items = nil
		m.selected = 0
		m.filter = ""
		m.searchPending = false
		return m, nil
	case stateEKSClusterList, stateECRRepoList, stateELBList, stateRDSInstanceList,
		stateKMSKeyList, stateCloudFrontDistList, stateLambdaFunctionList:
		m.kind = stateMainMenu
		m.menuItems = mainMenuItems()
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		m.detailMap = nil
		return m, nil
	case stateECRImageList:
		m.resetPage()
		m.detailMap = nil
		return m, ecrRepoListCmd(m, nil, false, "")
	case stateIAMUserList, stateIAMRoleList, stateIAMPolicyList:
		m.kind = stateIAMMenu
		m.menuSelected = 0
		m.items = nil
		m.filter = ""
		m.searchPending = false
		m.detailMap = nil
		return m, nil
	case stateS3PutObject:
		if m.s3PutStep == 0 {
			m.kind = stateS3BucketList
			m.s3PutKey = ""
			m.s3PutStep = 0
			return m, nil
		}
		m.s3PutStep = 0
		m.s3LocalPath = ""
		return m, nil
	case stateConfirm:
		m.kind = m.confirmBackState
		m.selected = 0
		m.confirmMsg = ""
		m.confirmAction = ""
		m.confirmTarget = ""
		m.confirmBackState = 0
		return m, nil
	case stateMessage:
		m.kind = m.prevStateAfterMessage
		m.msgText = ""
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) setMessage(err bool, text string) {
	m.msgErr = err
	m.msgText = text
	m.prevStateAfterMessage = m.kind
	m.kind = stateMessage
}
