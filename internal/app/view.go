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

func (m *model) filterSubtitle() string {
	spin := ""
	if m.loading {
		spin = " " + m.spinnerChar()
	}
	if m.filterMode {
		if m.searching {
			return "/" + m.filter + spin
		}
		if m.searchPending {
			return "/" + m.filter + "  (Enter to search)"
		}
		return "/" + m.filter + "_"
	}
	if m.searching {
		if m.filter != "" {
			return "search: " + m.filter + spin
		}
		return m.spinnerChar() + " Loading..."
	}
	if m.filter != "" {
		return "filter: " + m.filter
	}
	return ""
}

func (m *model) loadingIndicator() string {
	if !m.loading || m.searching {
		return ""
	}
	return "\n" + dimStyle.Render(m.spinnerChar()+" Loading...")
}

func (m *model) View() string {
	base := m.renderBase()
	if m.mfaPromptVisible {
		return base + "\n" + m.renderMFAPrompt()
	}
	return base
}

func (m *model) renderMFAPrompt() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(0, 1)
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("MFA required")
	body := fmt.Sprintf("%s  Enter MFA code: %s_", label, m.mfaInput)
	hint := dimStyle.Render("Enter submit · Esc cancel · Backspace delete")
	return style.Render(body + "\n" + hint)
}

func (m *model) renderBase() string {
	switch m.kind {
	case stateProfile:
		return m.viewList("Select profile", m.filterSubtitle())
	case stateRegion:
		return m.viewList("Select region", m.filterSubtitle())
	case stateMainMenu:
		return m.viewMenu("iaws — Main menu", "Profile: "+m.profile+"  Region: "+m.region)
	case stateEC2Menu:
		return m.viewMenu("EC2", "")
	case stateEC2InstanceList, stateEC2VPCList, stateEC2SubnetList, stateEC2SGList, stateEC2KeyList, stateEC2VolumeList, stateEC2SnapshotList, stateEC2AMIList:
		return m.viewList(m.ec2ListTitle(), m.filterSubtitle())
	case stateSSMMenu:
		return m.viewMenu("SSM", "")
	case stateSSMParamList:
		return m.viewList("SSM Parameters", m.filterSubtitle())
	case stateSSMParamGet:
		return m.viewList("SSM Parameters", "")
	case stateSSMLoginInstanceList:
		return m.viewList("Select instance for SSM session", m.filterSubtitle())
	case stateSecretsMenu:
		return m.viewMenu("Secrets Manager", "")
	case stateSecretsList:
		return m.viewList("Secrets", m.filterSubtitle())
	case stateSecretGet:
		return m.viewList("Secrets", "")
	case stateSecretPut:
		return m.viewSecretPutInput()
	case stateS3Menu:
		return m.viewMenu("S3", "")
	case stateS3BucketList:
		return m.viewList("S3 Buckets", m.filterSubtitle())
	case stateS3ObjectList:
		return m.viewList("S3: "+m.s3Bucket+"/"+m.s3Prefix, m.filterSubtitle())
	case stateS3GetObject:
		return m.viewList("S3", "")
	case stateACMMenu:
		return m.viewMenu("ACM (Certificate Manager)", "")
	case stateACMCertList:
		return m.viewList("ACM Certificates", m.filterSubtitle())
	case stateACMCertDetail:
		return m.viewList("Certificate Detail", "")
	case stateRoute53Menu:
		return m.viewMenu("Route 53", "")
	case stateRoute53ZoneList:
		return m.viewList("Hosted Zones", m.filterSubtitle())
	case stateRoute53RecordList:
		return m.viewList("Records: "+m.r53ZoneName, m.filterSubtitle())
	case stateEKSClusterList:
		return m.viewList("EKS Clusters", m.filterSubtitle())
	case stateECRRepoList:
		return m.viewList("ECR Repositories", m.filterSubtitle())
	case stateECRImageList:
		return m.viewList("ECR Images: "+m.ecrRepoName, m.filterSubtitle())
	case stateELBList:
		return m.viewList("Load Balancers", m.filterSubtitle())
	case stateIAMMenu:
		return m.viewMenu("IAM", "")
	case stateIAMUserList:
		return m.viewList("IAM Users", m.filterSubtitle())
	case stateIAMRoleList:
		return m.viewList("IAM Roles", m.filterSubtitle())
	case stateIAMPolicyList:
		return m.viewList("IAM Policies", m.filterSubtitle())
	case stateRDSInstanceList:
		return m.viewList("RDS Instances", m.filterSubtitle())
	case stateKMSKeyList:
		return m.viewList("KMS Keys", m.filterSubtitle())
	case stateCloudFrontDistList:
		return m.viewList("CloudFront Distributions", m.filterSubtitle())
	case stateLambdaFunctionList:
		return m.viewList("Lambda Functions", m.filterSubtitle())
	case stateBillingMenu:
		return m.viewMenu("Billing (Cost Explorer)", "")
	case stateBillingServiceCost:
		now := time.Now().UTC()
		return m.viewList("Cost by Service ("+now.Format("2006-01")+")", m.filterSubtitle())
	case stateBillingTopResources:
		now := time.Now().UTC()
		return m.viewList("Top Resources ("+now.Format("2006-01")+")", m.filterSubtitle())
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
	case stateEC2SnapshotList:
		return "Snapshots"
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
	b.WriteString(m.loadingIndicator())
	return b.String()
}

func (m *model) renderTableHeader() string {
	cols := m.getTableColumns()
	if len(cols) == 0 {
		return ""
	}
	var header strings.Builder
	totalWidth := 0
	for i, col := range cols {
		if i > 0 {
			header.WriteString(" ")
			totalWidth++
		}
		name := col.Name
		if i == m.sortColIdx && m.sorted {
			if m.sortDesc {
				name += "↓"
			} else {
				name += "↑"
			}
		}
		w := col.Width
		if w < 0 {
			w = len([]rune(name))
		}
		padded := fmt.Sprintf("%-*s", w, name)
		if i == m.sortColIdx {
			header.WriteString(selStyle.Render(padded))
		} else {
			header.WriteString(dimStyle.Render(padded))
		}
		totalWidth += w
	}
	sep := dimStyle.Render(strings.Repeat("─", totalWidth))
	return "  " + header.String() + "\n  " + sep + "\n"
}

func (m *model) renderPopup() string {
	var content strings.Builder
	for i, item := range m.popupItems {
		if i == m.popupSelected {
			content.WriteString(selStyle.Render("> " + item))
		} else {
			content.WriteString("  " + item)
		}
		if i < len(m.popupItems)-1 {
			content.WriteString("\n")
		}
	}
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(0, 1)
	return popupStyle.Render(content.String())
}

func (m *model) viewList(title, subtitle string) string {
	items := m.visibleItems()
	cols := m.getTableColumns()
	hasHeader := len(cols) > 0

	var hint string
	if m.popupVisible {
		hint = "↑/↓ select · Enter confirm · Esc cancel"
	} else if m.filterMode {
		hint = "type to filter · Enter done · Esc clear"
	} else {
		hint = "↑/k ↓/j Enter Esc · / search"
		if hasHeader {
			hint += " · Tab col · s sort"
		}
	}

	if len(items) == 0 {
		var b strings.Builder
		b.WriteString(titleStyle.Render(title) + "\n")
		if subtitle != "" {
			b.WriteString(dimStyle.Render(subtitle) + "\n")
		}
		b.WriteString("\n")
		if hasHeader {
			b.WriteString(m.renderTableHeader())
		}
		b.WriteString(dimStyle.Render("(no items)"))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render(hint))
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
	if hasHeader {
		b.WriteString(m.renderTableHeader())
	}
	for i := start; i < end; i++ {
		item := items[i]
		if i == sel {
			b.WriteString(selStyle.Render("> "+item.Title) + "\n")
			if m.popupVisible {
				popup := m.renderPopup()
				for _, line := range strings.Split(popup, "\n") {
					b.WriteString("      " + line + "\n")
				}
			}
		} else {
			b.WriteString("  " + item.Title + "\n")
		}
	}
	if len(items) > pageSize {
		b.WriteString(dimStyle.Render(fmt.Sprintf("— %d of %d —", sel+1, len(items))) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(hint))
	b.WriteString(m.loadingIndicator())
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
