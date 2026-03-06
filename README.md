# iaws (Interactive AWS CLI)

An interactive AWS CLI built with Go — no need to compose long flags or complex arguments. Browse, filter, and manage AWS resources through a Terminal UI.

## Installation

```bash
git clone <repo>
cd iaws
go mod tidy
go build -o iaws .
# optionally move iaws to your PATH
```

## Supported AWS Services

| Service | Features |
|---------|----------|
| **EC2** | Instances (Start/Stop/Reboot), VPCs, Subnets, Security Groups, Key Pairs, Volumes, Snapshots, AMIs |
| **SSM** | Login to EC2 (Session Manager), Parameter list, Get parameter value |
| **Secrets Manager** | List secrets, Get/Put secret value |
| **S3** | Bucket list, Object browsing (directory navigation), Download/Upload files |
| **ACM** | Certificate list, Certificate detail |
| **Route 53** | Hosted zone list, DNS record list |
| **EKS** | Cluster list, Cluster detail |
| **ECR** | Repository list, Image list |
| **ELB** | Load balancer list and detail |
| **IAM** | Users, Roles, Policies list and detail |
| **RDS** | Instance list and detail |
| **KMS** | Key list and detail |
| **CloudFront** | Distribution list and detail |
| **Lambda** | Function list and detail |
| **Billing** | Monthly cost (6 months), Cost by service, Daily cost (30 days), Top resources, Cost optimization suggestions |

## Key Bindings

### Global

| Key | Action |
|-----|--------|
| `Ctrl+C` | Quit |

### Menu Views

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Select |
| `Esc` | Go back |

### List Views

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Select / View detail |
| `Esc` | Clear filter or go back |
| `/` | Enter filter/search mode |
| `Tab` | Cycle sort column |
| `s` | Sort (press once for ascending, again for descending) |

### Filter/Search Mode

| Key | Action |
|-----|--------|
| Type | Update filter keyword (remote search debounced at 2s) |
| `Backspace` | Delete last character |
| `Enter` | Exit filter mode (triggers search for remote states) |
| `Esc` | Clear filter and exit filter mode |

### Confirm Dialog

| Key | Action |
|-----|--------|
| `y` / `Y` | Confirm |
| `n` / `Esc` | Cancel |

## Menu Structure

```
Select Profile
└── Select Region
    └── Main Menu
        ├── EC2
        │   ├── Instances → Start/Stop/Reboot
        │   ├── VPCs / Subnets / Security Groups / Key Pairs
        │   ├── Volumes / Snapshots / AMIs
        │   └── Back
        ├── SSM
        │   ├── SSM Login EC2 → Select instance → Start session
        │   ├── Parameter list → Get value
        │   └── Back
        ├── Secrets Manager
        │   ├── List / Get / Put
        │   └── Back
        ├── S3
        │   ├── List buckets → Browse objects → Download
        │   ├── Upload file → Select bucket → Enter path
        │   └── Back
        ├── ACM → Certificate list → Detail
        ├── Route 53 → Hosted zones → DNS records
        ├── EKS → Cluster list → Detail
        ├── ECR → Repository list → Image list
        ├── ELB → Load balancer list → Detail
        ├── IAM → Users / Roles / Policies → Detail
        ├── RDS → Instance list → Detail
        ├── KMS → Key list → Detail
        ├── CloudFront → Distribution list → Detail
        ├── Lambda → Function list → Detail
        ├── Billing
        │   ├── Monthly cost (6-month trend)
        │   ├── Cost by service → Select service → Usage type breakdown
        │   ├── Daily cost (30 days)
        │   ├── Top resources (top 50 this month)
        │   ├── Cost optimization (suggestions)
        │   └── Back
        └── Quit
```

## Tables & Sorting

All data list views include **column headers**. Use `Tab` to cycle the sort column and `s` to sort. Numeric values (e.g. `$123.45`, `10GiB`, `128MB`) are automatically detected and sorted numerically rather than lexicographically.

The active sort column is highlighted in the header, with ↑ (ascending) / ↓ (descending) arrows shown after sorting.

## Dependencies & Configuration

- **AWS Credentials**: Uses `~/.aws/config` and `~/.aws/credentials`. On launch, select a **profile** (including default), then a **region**. All subsequent operations use that profile/region.
- **Assume Role + MFA**: If the selected profile uses MFA (e.g. `role_arn` + `mfa_serial`), set the environment variable **`AWS_MFA_CODE`** to your current MFA code before running. Example: `AWS_MFA_CODE=123456 ./iaws`. **Credentials are cached in `~/.aws/cli/cache/`** (same directory as AWS CLI). Within the validity period, re-running iaws or SSM login does not require re-entering MFA.
- **Shared cache with AWS CLI / kubectl**: iaws stores assume-role cache in **`~/.aws/cli/cache/`** (filenames prefixed with `iaws_` to avoid conflicts with CLI). If valid cache from other tools exists in this directory, iaws will reuse it; cache written by iaws can also be shared with other tools. kubectl accesses EKS via `aws eks get-token` using the default AWS credential chain, sharing the same credentials and cache directory.
- **SSM Login to EC2**: Requires [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) installed locally, and instances must have SSM configured with appropriate IAM permissions. If you see **"Plugin with name Standard_Stream not found"**, this is typically a resource issue on the **EC2 instance side**: disk full, inotify exhausted, or file descriptor limits exceeded. Check `df -h` and `/proc/sys/fs/inotify/max_user_watches` on the instance; expand disk or increase `fs.inotify.max_user_watches` / `fs.file-max` then restart SSM Agent or the instance. Locally, update Session Manager Plugin to the latest version.

## Usage

```bash
./iaws
# 1) Select profile (e.g. default)
# 2) Select region (e.g. us-east-1)
# 3) Choose a service from the main menu (EC2 / SSM / S3 / Billing etc.)
# 4) Press / to enter filter mode, type keywords to filter; ↑/k ↓/j to navigate, Enter to select, Esc to go back
# 5) Press Tab to cycle sort column, press s to sort
# 6) Destructive operations (e.g. Stop instance, Put Secret) require confirmation (y/n)
```

With an MFA-protected profile:

```bash
AWS_MFA_CODE=123456 ./iaws
```

## Testing with LocalStack

Core paths (EC2 list, SSM list/get, Secrets Manager list/get/put, S3 list/get, etc.) can be verified without a real AWS account.

1. **Start LocalStack** (e.g. with Docker):

```bash
docker run --rm -d -p 4566:4566 -p 4510-4559:4510-4559 localstack/localstack
```

2. **Set environment variables and run iaws**:

```bash
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_ENDPOINT_URL=http://localhost:4566
./iaws
```

3. Select a profile (e.g. default) and region (e.g. `us-east-1`) to operate against LocalStack. To pre-populate test data, use `awslocal` or AWS CLI pointed at `http://localhost:4566` to create EC2 instances, SSM parameters, Secrets, S3 buckets, etc., then verify in iaws.

## Tech Stack

- **Language**: Go
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **AWS SDK**: [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2)
