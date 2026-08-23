# CRM 客户关系管理

## 目标
面向销售团队的统一客户关系管理，覆盖线索、客户、联系人、商机、跟进记录和任务，帮助团队沉淀客户资产、推进销售阶段并提升线索转化。

## Objects
### 线索
Properties:
- 名称: string required
- 公司: string
- 电话: string
- 邮箱: string
- 来源: enum[官网,广告,展会,转介绍,外呼,其他]
- 状态: enum[新建,跟进中,已转换,已废弃]
- 负责人: string
- 备注: string

### 客户
Properties:
- 名称: string required
- 行业: string
- 电话: string
- 邮箱: string
- 地址: string
- 等级: enum[普通,重要,VIP]
- 状态: enum[潜在客户,跟进中,已成交,已流失]
- 来源: enum[线索转化,官网,展会,转介绍,其他]
- 负责人: string
- 备注: string

### 联系人
Properties:
- 姓名: string required
- 职位: string
- 电话: string
- 邮箱: string
- 是否主联系人: enum[是,否]
- 备注: string

### 商机
Properties:
- 名称: string required
- 预计金额: number
- 阶段: enum[初步接洽,需求确认,方案报价,商务谈判,赢单,输单]
- 赢单概率: number
- 预计成交日期: string
- 负责人: string
- 备注: string

### 跟进记录
Properties:
- 方式: enum[电话,拜访,邮件,微信,会议,其他]
- 内容: string required
- 结果: string
- 下次跟进时间: string
- 负责人: string

### 任务
Properties:
- 标题: string required
- 截止日期: string
- 优先级: enum[低,中,高]
- 状态: enum[待处理,进行中,已完成,已取消]
- 负责人: string

## Links
- 线索 has 客户
- 客户 has 联系人
- 客户 has 商机
- 客户 has 跟进记录
- 商机 has 跟进记录
- 客户 has 任务

## Actions
### 线索转客户
Input:
- 线索: ref 线索 required
- 客户: ref 客户 required
Rules:
- state: 状态 跟进中
Mutations:
- set: 线索.状态 = 已转换
- link: 线索_客户 from 线索 to 客户

### 客户成交
Input:
- 客户: ref 客户 required
Rules:
- state: 状态 跟进中
Mutations:
- set: 客户.状态 = 已成交

### 客户流失
Input:
- 客户: ref 客户 required
Rules:
- state: 状态 跟进中
Mutations:
- set: 客户.状态 = 已流失

### 商机阶段推进
Input:
- 商机: ref 商机 required
- 阶段: enum[初步接洽,需求确认,方案报价,商务谈判,赢单,输单] required
Mutations:
- set: 商机.阶段 = {阶段}

### 分配客户负责人
Input:
- 客户: ref 客户 required
- 负责人: string required
Mutations:
- set: 客户.负责人 = {负责人}

### 添加跟进记录
Input:
- 客户: ref 客户 required
- 跟进记录: ref 跟进记录 required
Mutations:
- link: 客户_跟进记录 from 客户 to 跟进记录

### 完成任务
Input:
- 任务: ref 任务 required
Rules:
- state: 状态 进行中
Mutations:
- set: 任务.状态 = 已完成

## 业务规则
- 所有关键变更记录到历史。
- 客户与商机状态、金额变更必须记录操作者和时间。
- 已成交客户或赢单商机默认不物理删除。
- 未经授权不得跨租户读取或导出客户数据。

## 文件
- 支持上传 CSV、Excel、PDF、DOCX、图片和 Markdown 业务资料。
- 导入前展示预览，导入后保留原始文件。

## 页面
- 工作台
- 线索
- 客户
- 联系人
- 商机
- 跟进记录
- 任务
- 数据导入
- 历史记录
