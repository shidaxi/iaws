package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	itemStyle  = lipgloss.NewStyle()
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
)

func (m *model) searchSubtitle() string {
	if m.searching {
		if m.filter == "" {
			return "Loading..."
		}
		return "Searching: " + m.filter + " ..."
	}
	if m.filter == "" {
		return ""
	}
	if m.searchPending {
		return "Search: " + m.filter + "  (Enter or wait 2s)"
	}
	return "Search: " + m.filter
}

func (m *model) View() string {
	switch m.kind {
	case stateProfile:
		return m.viewList("Select profile", "filter: "+m.filter)
	case stateRegion:
		return m.viewList("Select region", "filter: "+m.filter)
	case stateMainMenu:
		return m.viewMenu("iaws — Main menu", "Profile: "+m.profile+"  Region: "+m.region)
	case stateEC2Menu:
		return m.viewMenu("EC2", "")
	case stateEC2InstanceAction:
		return m.viewMenu("Instance "+m.confirmTarget+" — Start/Stop/Reboot", "")
	case stateEC2InstanceList, stateEC2VPCList, stateEC2SubnetList, stateEC2SGList, stateEC2KeyList, stateEC2AMIList:
		return m.viewList(m.ec2ListTitle(), m.searchSubtitle())
	case stateEC2VolumeList:
		return m.viewVolumeList()
	case stateSSMMenu:
		return m.viewMenu("SSM", "")
	case stateSSMParamList:
		return m.viewList("SSM Parameters", m.searchSubtitle())
	case stateSSMParamGet:
		return m.viewList("SSM Parameters", "")
	case stateSSMLoginInstanceList:
		return m.viewList("Select instance for SSM session", m.searchSubtitle())
	case stateSecretsMenu:
		return m.viewMenu("Secrets Manager", "")
	case stateSecretsList:
		return m.viewList("Secrets", m.searchSubtitle())
	case stateSecretGet:
		return m.viewList("Secrets", "")
	case stateSecretPut:
		return m.viewSecretPutInput()
	case stateS3Menu:
		return m.viewMenu("S3", "")
	case stateS3BucketList:
		return m.viewList("S3 Buckets", m.searchSubtitle())
	case stateS3ObjectList:
		return m.viewList("S3: "+m.s3Bucket+"/"+m.s3Prefix, m.searchSubtitle())
	case stateS3GetObject:
		return m.viewList("S3", "")
	case stateACMMenu:
		return m.viewMenu("ACM (Certificate Manager)", "")
	case stateACMCertList:
		return m.viewList("ACM Certificates", m.searchSubtitle())
	case stateACMCertDetail:
		return m.viewList("Certificate Detail", "")
	case stateRoute53Menu:
		return m.viewMenu("Route 53", "")
	case stateRoute53ZoneList:
		return m.viewList("Hosted Zones", m.searchSubtitle())
	case stateRoute53RecordList:
		return m.viewList("Records: "+m.r53ZoneName, m.searchSubtitle())
	case stateEKSClusterList:
		return m.viewList("EKS Clusters", m.searchSubtitle())
	case stateECRRepoList:
		return m.viewList("ECR Repositories", m.searchSubtitle())
	case stateECRImageList:
		return m.viewList("ECR Images: "+m.ecrRepoName, m.searchSubtitle())
	case stateELBList:
		return m.viewList("Load Balancers", m.searchSubtitle())
	case stateIAMMenu:
		return m.viewMenu("IAM", "")
	case stateIAMUserList:
		return m.viewList("IAM Users", m.searchSubtitle())
	case stateIAMRoleList:
		return m.viewList("IAM Roles", m.searchSubtitle())
	case stateIAMPolicyList:
		return m.viewList("IAM Policies", m.searchSubtitle())
	case stateRDSInstanceList:
		return m.viewList("RDS Instances", m.searchSubtitle())
	case stateKMSKeyList:
		return m.viewList("KMS Keys", m.searchSubtitle())
	case stateCloudFrontDistList:
		return m.viewList("CloudFront Distributions", m.searchSubtitle())
	case stateLambdaFunctionList:
		return m.viewList("Lambda Functions", m.searchSubtitle())
	case stateBillingMenu:
		return m.viewMenu("Billing (Cost Explorer)", "")
	case stateBillingServiceCost:
		now := time.Now().UTC()
		return m.viewList("Cost by Service ("+now.Format("2006-01")+")", "filter: "+m.filter)
	case stateS3PutObject:
		return m.viewS3PutInput()
	case stateConfirm:
		return m.viewConfirm()
	case stateMessage:
		return m.viewMessage()
	default:
		return "Unknown state"
	}
}

func (m *model) ec2ListTitle() string {
	switch m.kind {
	case stateEC2InstanceList:
		return "EC2 Instances"
	case stateEC2VPCList:
		return "VPCs"
	case stateEC2SubnetList:
		return "Subnets"
	case stateEC2SGList:
		return "Security Groups"
	case stateEC2KeyList:
		return "Key Pairs"
	case stateEC2VolumeList:
		return "Volumes"
	case stateEC2AMIList:
		return "AMIs"
	}
	return "EC2"
}

func (m *model) viewMenu(title, subtitle string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	if subtitle != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(subtitle))
	}
	b.WriteString("\n\n")
	for i, item := range m.menuItems {
		if i == m.menuSelected {
			b.WriteString(selStyle.Render("> "+item) + "\n")
		} else {
			b.WriteString("  " + item + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back"))
	return b.String()
}

func (m *model) viewList(title, subtitle string) string {
	items := m.visibleItems()
	if len(items) == 0 {
		var b strings.Builder
		b.WriteString(titleStyle.Render(title) + "\n")
		if subtitle != "" {
			b.WriteString(dimStyle.Render(subtitle) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("(no items)"))
		b.WriteString("\n\n")
		if m.isRemoteSearchState() {
			b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back · type to search"))
		} else {
			b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back · type to filter"))
		}
		return b.String()
	}
	sel := m.selected
	if sel >= len(items) {
		sel = len(items) - 1
	}
	if sel < 0 {
		sel = 0
	}
	pageSize := 20
	start := sel - pageSize/2
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
		start = end - pageSize
		if start < 0 {
			start = 0
		}
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	if subtitle != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(subtitle))
	}
	b.WriteString("\n\n")
	for i := start; i < end; i++ {
		item := items[i]
		if i == sel {
			b.WriteString(selStyle.Render("> "+item.Title) + "\n")
		} else {
			b.WriteString("  " + item.Title + "\n")
		}
	}
	if len(items) > pageSize {
		b.WriteString(dimStyle.Render(fmt.Sprintf("— %d of %d —", sel+1, len(items))) + "\n")
	}
	b.WriteString("\n")
	if m.isRemoteSearchState() {
		b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back · type to search"))
	} else {
		b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back · type to filter"))
	}
	return b.String()
}

func (m *model) viewVolumeList() string {
	subtitle := m.searchSubtitle()
	if subtitle == "" {
		if m.volumeSortDesc {
			subtitle = "sorted by size ↓"
		} else if len(m.ec2Volumes) > 0 && m.items != nil {
			subtitle = "sorted by size ↑"
		}
	}
	items := m.visibleItems()
	if len(items) == 0 {
		var b strings.Builder
		b.WriteString(titleStyle.Render("Volumes") + "\n")
		if subtitle != "" {
			b.WriteString(dimStyle.Render(subtitle) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("(no items)"))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back · type to search · Opt-s sort by size"))
		return b.String()
	}
	sel := m.selected
	if sel >= len(items) {
		sel = len(items) - 1
	}
	if sel < 0 {
		sel = 0
	}
	pageSize := 20
	start := sel - pageSize/2
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
		start = end - pageSize
		if start < 0 {
			start = 0
		}
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Volumes"))
	if subtitle != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(subtitle))
	}
	b.WriteString("\n\n")
	for i := start; i < end; i++ {
		item := items[i]
		if i == sel {
			b.WriteString(selStyle.Render("> "+item.Title) + "\n")
		} else {
			b.WriteString("  " + item.Title + "\n")
		}
	}
	if len(items) > pageSize {
		b.WriteString(dimStyle.Render(fmt.Sprintf("— %d of %d —", sel+1, len(items))) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑/k ↓/j Enter Esc back · type to search · Opt-s sort by size"))
	return b.String()
}

func (m *model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Confirm") + "\n\n")
	b.WriteString(m.confirmMsg + "\n\n")
	b.WriteString(dimStyle.Render("y = yes  n/Esc = no"))
	return b.String()
}

func (m *model) viewSecretPutInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Put secret value: " + m.secretName) + "\n\n")
	b.WriteString("Value: " + m.inputValue + "_")
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Type value, Enter to confirm, Esc to cancel"))
	return b.String()
}

func (m *model) viewS3PutInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("S3 Upload: " + m.s3Bucket) + "\n\n")
	if m.s3PutStep == 0 {
		b.WriteString("Object key: " + m.s3PutKey + "_")
	} else {
		b.WriteString("Object key: " + m.s3PutKey + "\n")
		b.WriteString("Local path: " + m.s3LocalPath + "_")
	}
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Enter key then path, Enter to submit, Esc to back"))
	return b.String()
}

func (m *model) viewMessage() string {
	var b strings.Builder
	if m.msgErr {
		b.WriteString(errStyle.Render("Error") + "\n\n")
	} else {
		b.WriteString(okStyle.Render("OK") + "\n\n")
	}
	w := m.width
	if w <= 0 {
		w = 80
	}
	b.WriteString(wrapText(m.msgText, w))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Enter/Space/Esc to close"))
	return b.String()
}

func wrapText(s string, width int) string {
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if len(line) <= width {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		for len(line) > width {
			out.WriteString(line[:width])
			out.WriteByte('\n')
			line = line[width:]
		}
		if len(line) > 0 {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	result := out.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result
}
