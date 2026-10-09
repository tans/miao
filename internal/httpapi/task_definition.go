package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hhmmPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
var offsetPattern = regexp.MustCompile(`(?:Z|[+-]\d{2}:\d{2})$`)

type taskTrigger struct {
	Type     string `json:"type"`
	Timezone string `json:"timezone"`
	At       string `json:"at,omitempty"`
	Time     string `json:"time,omitempty"`
	Weekdays []int  `json:"weekdays,omitempty"`
	Table    string `json:"table,omitempty"`
	Field    string `json:"field,omitempty"`
	From     string `json:"from"`
	To       string `json:"to"`
}
type taskGrant struct {
	Table       string   `json:"table"`
	ReadFields  []string `json:"read_fields"`
	WriteFields []string `json:"write_fields"`
}
type taskScope struct {
	Tables          []taskGrant    `json:"tables"`
	ActionIDs       []string       `json:"action_ids,omitempty"`
	ActionRevisions map[string]int `json:"action_revisions,omitempty"`
	RecipientIDs    []string       `json:"recipient_ids"`
}
type taskLimits struct {
	MaxWrites                int `json:"max_writes"`
	MaxRequests              int `json:"max_requests"`
	TimeoutSeconds           int `json:"timeout_seconds"`
	ConfirmationTimeoutHours int `json:"confirmation_timeout_hours"`
}
type taskDefinition struct {
	Goal      string      `json:"goal"`
	Execution string      `json:"execution"`
	Trigger   taskTrigger `json:"trigger"`
	Scope     taskScope   `json:"scope"`
	Limits    taskLimits  `json:"limits"`
}

func normalizeTaskDefinition(ctx context.Context, s *Server, tenantID, appID string, raw any) (*taskDefinition, string) {
	if _, ok := raw.(map[string]any); !ok {
		return nil, "任务定义必须是对象"
	}
	definition := &taskDefinition{Execution: "agent", Trigger: taskTrigger{Type: "manual", Timezone: "Asia/Shanghai"}, Limits: taskLimits{10, 12, 180, 72}}
	encoded, err := json.Marshal(raw)
	if err != nil || json.Unmarshal(encoded, definition) != nil {
		return nil, "任务定义的字段、星期和运行限制类型无效"
	}
	definition.Goal = strings.TrimSpace(definition.Goal)
	if definition.Goal == "" || len([]rune(definition.Goal)) > 6000 {
		return nil, "请提供不超过 6000 字的任务目标"
	}
	if definition.Execution == "agent" {
		return nil, "后台 LLM Agent 能力暂时下架，请改用固定数据报告任务"
	}
	trigger := &definition.Trigger
	if !containsString([]string{"manual", "once", "daily", "weekly", "record_created", "status_changed"}, trigger.Type) {
		return nil, "触发类型无效"
	}
	if _, err := time.LoadLocation(trigger.Timezone); err != nil {
		return nil, "时区无效"
	}
	switch trigger.Type {
	case "once":
		if !offsetPattern.MatchString(trigger.At) {
			return nil, "一次性时间必须带时区"
		}
		at, err := time.Parse(time.RFC3339, trigger.At)
		if err != nil {
			return nil, "一次性时间必须带时区"
		}
		trigger.At = at.UTC().Format(time.RFC3339Nano)
	case "daily", "weekly":
		if !hhmmPattern.MatchString(trigger.Time) {
			return nil, "运行时间格式为 HH:mm"
		}
		if trigger.Type == "weekly" {
			if len(trigger.Weekdays) == 0 {
				return nil, "星期使用 0–6，0 为周日"
			}
			seen := map[int]bool{}
			days := []int{}
			for _, day := range trigger.Weekdays {
				if day < 0 || day > 6 {
					return nil, "星期使用 0–6，0 为周日"
				}
				if !seen[day] {
					seen[day] = true
					days = append(days, day)
				}
			}
			trigger.Weekdays = days
		}
	}
	tables, err := s.PB.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID)), "created")
	if err != nil {
		return nil, "任务数据表暂不可用，请稍后重试"
	}
	if len(definition.Scope.Tables) < 1 || len(definition.Scope.Tables) > 12 {
		return nil, "请明确授权 1–12 张数据表"
	}
	if len(definition.Scope.ActionIDs) > 20 {
		return nil, "后台任务最多授权 20 个业务动作"
	}
	definition.Scope.ActionRevisions = map[string]int{}
	for _, actionID := range definition.Scope.ActionIDs {
		action, actionErr := s.PB.Get(ctx, "business_actions", actionID)
		if actionErr != nil || action["tenant_id"] != tenantID || action["app_id"] != appID || action["status"] != "enabled" {
			return nil, "授权的业务动作不存在、未启用或不属于当前应用"
		}
		definition.Scope.ActionRevisions[actionID] = intValue(action["revision"])
	}
	definition.Scope.ActionIDs = uniqueStrings(definition.Scope.ActionIDs, 0)
	seenTable := map[string]bool{}
	for i := range definition.Scope.Tables {
		grant := &definition.Scope.Tables[i]
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == grant.Table {
				table = candidate
				break
			}
		}
		if table == nil || seenTable[grant.Table] {
			return nil, "授权的数据表不存在或重复"
		}
		seenTable[grant.Table] = true
		allowed := map[string]bool{}
		for _, field := range asSliceMap(table["fields"]) {
			if field["type"] != "file" && field["type"] != "relation" {
				allowed[stringValue(field["name"])] = true
			}
		}
		if len(grant.ReadFields) == 0 || len(grant.ReadFields) > 24 || len(grant.WriteFields) > 24 {
			return nil, "需明确授权可读字段；后台任务不支持附件或关联字段"
		}
		for _, name := range append(append([]string{}, grant.ReadFields...), grant.WriteFields...) {
			if !allowed[name] {
				return nil, "授权字段不存在或不支持后台执行"
			}
		}
		for _, name := range grant.WriteFields {
			if !containsString(grant.ReadFields, name) {
				return nil, "可写字段也必须授权读取，便于展示修改前后的内容"
			}
		}
		grant.ReadFields = uniqueStrings(grant.ReadFields, 0)
		grant.WriteFields = uniqueStrings(grant.WriteFields, 0)
	}
	if trigger.Type == "record_created" || trigger.Type == "status_changed" {
		if !seenTable[trigger.Table] {
			return nil, "业务事件必须来自获授权的数据表"
		}
		if trigger.Type == "status_changed" {
			var field map[string]any
			readable := false
			for _, table := range tables {
				if table["slug"] == trigger.Table {
					field = findField(asSliceMap(table["fields"]), trigger.Field)
				}
			}
			for _, grant := range definition.Scope.Tables {
				if grant.Table == trigger.Table {
					readable = containsString(grant.ReadFields, trigger.Field)
				}
			}
			if field == nil || field["type"] != "select" || !readable || !contains(field["options"], trigger.From) && !(trigger.From == "" && !boolValue(field["required"])) || !contains(field["options"], trigger.To) && !(trigger.To == "" && !boolValue(field["required"])) || trigger.From == trigger.To {
				return nil, "状态变化条件无效"
			}
		}
	}
	if len(definition.Scope.RecipientIDs) > 10 {
		return nil, "接收人最多 10 位"
	}
	for _, uid := range definition.Scope.RecipientIDs {
		if uid == "" {
			return nil, "接收人不能为空"
		}
		if _, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(tenantID), "user_id = "+pbFilterString(uid))); err != nil {
			return nil, "接收人必须是当前工作区成员"
		}
	}
	definition.Scope.RecipientIDs = uniqueStrings(definition.Scope.RecipientIDs, 0)
	if definition.Execution != "agent" && definition.Execution != "report" {
		return nil, "执行方式为 agent 或 report"
	}
	for _, limit := range [][3]int{{definition.Limits.MaxWrites, 0, 100}, {definition.Limits.MaxRequests, 1, 30}, {definition.Limits.TimeoutSeconds, 30, 600}, {definition.Limits.ConfirmationTimeoutHours, 1, 720}} {
		if limit[0] < limit[1] || limit[0] > limit[2] {
			return nil, fmt.Sprintf("运行限制必须是 %d–%d 的整数", limit[1], limit[2])
		}
	}
	return definition, ""
}

func nextScheduledRun(trigger map[string]any, after time.Time) string {
	typ := stringValue(trigger["type"])
	if typ == "manual" || typ == "record_created" || typ == "status_changed" {
		return ""
	}
	if typ == "once" {
		at := parseTime(trigger["at"])
		if at.After(after) {
			return at.UTC().Format(time.RFC3339Nano)
		}
		return ""
	}
	loc, err := time.LoadLocation(defaultString(stringValue(trigger["timezone"]), "Asia/Shanghai"))
	if err != nil {
		return ""
	}
	hourMinute := strings.Split(stringValue(trigger["time"]), ":")
	if len(hourMinute) != 2 {
		return ""
	}
	hour, _ := strconv.Atoi(hourMinute[0])
	minute, _ := strconv.Atoi(hourMinute[1])
	weekdays := map[int]bool{}
	for _, raw := range anySlice(trigger["weekdays"]) {
		weekdays[intValue(raw)] = true
	}
	local := after.In(loc)
	for day := 0; day < 10; day++ {
		date := time.Date(local.Year(), local.Month(), local.Day()+day, 0, 0, 0, 0, loc)
		if typ == "weekly" && !weekdays[int(date.Weekday())] {
			continue
		}
		candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, loc)
		actual := candidate.In(loc)
		if actual.Year() != date.Year() || actual.Month() != date.Month() || actual.Day() != date.Day() || actual.Hour() != hour || actual.Minute() != minute {
			continue
		}
		if candidate.After(after) {
			return candidate.UTC().Format(time.RFC3339Nano)
		}
	}
	return ""
}
