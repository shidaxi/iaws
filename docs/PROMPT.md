# iaws 实现提示词

请按以下规范实现 **iaws（Interactive AWS CLI）**。

---

## 项目名称与描述

**iaws**：基于 Go 的交互式 AWS CLI，通过 Terminal UI 选择与过滤完成常用查看与少量写操作，无需拼接长 flag 与复杂参数。

---

## 核心目标与原则

1. **零长命令、全交互**：用户执行 `iaws` 后，所有参数与选项通过菜单/列表选择完成，无需手写一长串 flag。
2. **支持 filter**：列表类界面支持输入关键词过滤选项，便于在大量资源中快速定位。
3. **认证走 AWS config + profile**：自动读取 `~/.aws/config` 与 `~/.aws/credentials`，支持选择 profile（含 default），使用 AWS SDK 默认 credential chain。
4. **可用 LocalStack 测试**：核心路径（EC2 list、SSM list/get、Secrets Manager list/get/put、S3 list/get）能在 LocalStack 下运行，并在文档中说明如何用 LocalStack 做功能测试。

---

## 功能清单

### 认证与环境

- 启动时或全局设置中**选择 profile**（default 及 config 中的具名 profile），后续命令共用该 profile。
- **Region**：从所选 profile 继承；或提供交互式 region 选择（常用 region 列表或从 config 读取）。

### 按服务划分的操作

#### 第一期（必做）：EC2、SSM、Secrets Manager、S3

**EC2**

- 查看：实例列表、VPC 列表、子网列表、安全组列表、密钥对列表、卷列表、AMI 列表。
- 轻量写：实例 Start / Stop / Reboot（选择实例后一键，危险操作需确认）。

**SSM**

- 查看：参数列表、获取参数值（GetParameter）。
- 交互：**SSM 登录 EC2**：选 region/profile → 列实例（可 filter）→ 选实例 → 调用 `aws ssm start-session`（或等价 SDK）；依赖本地已安装 Session Manager Plugin。

**Secrets Manager**

- 查看：密钥列表、GetSecretValue（查看单个 secret 的 value）。
- 轻量写：PutSecretValue（单条 value，key 可选或固定，需确认）。

**S3**

- 查看：桶列表、对象列表（选桶后列 prefix/object）、下载对象（GetObject 到本地）。
- 轻量写：上传单个文件（选桶 + 选/输入 key + 选本地文件）。

#### 后续可扩展（可选）

- **RDS**：实例列表、集群列表、快照列表。
- **Lambda**：函数列表、函数详情/配置、别名与版本。
- **ECS**：集群列表、服务列表、任务列表、任务定义。
- **EKS**：集群列表、节点组、集群详情。
- **IAM**：用户/角色/策略列表、用户/角色详情。
- **CloudWatch**：日志组/流列表、指标列表、告警列表。

### 特别说明

- **SSM 登录 EC2**：流程为「选 profile/region → 列出 EC2 实例（可 filter）→ 选实例 → 执行 start-session」；若本地未安装 Session Manager Plugin，应给出明确提示。
- **Secrets Manager get/put**：get 为选择 secret 后展示 value（或写入临时文件）；put 为单条 value 的写入，关键操作需二次确认。
- **错误与权限**：命令失败时输出清晰错误信息；权限不足时尽量提示可能缺失的 IAM 动作（如 min 权限建议）。

---

## 交互与 UX 要求

1. **主菜单**：按服务/场景入口（如 EC2、SSM、Secrets Manager、S3），或按「查看 / 写操作」分类。
2. **二级**：进入某服务后为操作类型（如「实例列表」「登录实例」「参数列表」）。
3. **列表**：所有列表支持关键词过滤（输入即时过滤），支持键盘上下选择、Enter 确认。
4. **关键操作确认**：实例 Stop/Reboot、PutSecretValue、覆盖 S3 对象等需确认（Y/N 或明确按钮）。
5. **退出**：每层支持 Esc 或 q 返回上级或退出程序。

---

## 技术约束

- **语言**：Go。
- **Terminal UI**：使用 [Bubble Tea](https://github.com/charmbracelet/bubbletea)（bubbletea）+ [Lipgloss](https://github.com/charmbracelet/lipgloss)，实现列表、多选、输入框、filter。
- **AWS**：使用 [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2)，与 `aws configure` / `~/.aws/config` 兼容；使用 SDK 默认 credential 与 config 加载逻辑，不重复造轮子。
- **LocalStack**：通过环境变量或配置指定 endpoint（如 `AWS_ENDPOINT_URL`），使 EC2/SSM/Secrets Manager/S3 等调用可指向 LocalStack；在 README 或 docs 中说明如何启动 LocalStack 并运行功能测试。

---

## 交付物与验收

1. **可执行二进制**：项目可构建为单一二进制 `iaws`，执行后进入交互式主菜单。
2. **README**：包含安装方式、依赖（如 Session Manager Plugin 用于 SSM 登录）、配置说明（profile/region）、常用使用示例。
3. **LocalStack 测试**：提供如何在 LocalStack 下运行功能测试的说明（如 docker-compose 或命令示例），覆盖 EC2 list、SSM list/get、Secrets Manager list/get/put、S3 list/get 等核心路径。

完成以上即视为实现达标；后续可按需扩展更多服务与操作。
