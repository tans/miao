package httpapi

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hhmmPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
var offsetPattern = regexp.MustCompile(`(?:Z|[+-]\d{2}:\d{2})$`)

func normalizeTaskDefinition(ctx context.Context, s *Server, tenantID, appID string, raw any) (map[string]any,string) {
	input,ok:=raw.(map[string]any);if !ok{return nil,"任务定义必须是对象"}
	goal:=strings.TrimSpace(stringValue(input["goal"]));if goal==""||len([]rune(goal))>6000{return nil,"请提供不超过 6000 字的任务目标"}
	triggerInput:=asMap(input["trigger"]);typ:=defaultString(stringValue(triggerInput["type"]),"manual");if !containsString([]string{"manual","once","daily","weekly","record_created","status_changed"},typ){return nil,"触发类型无效"}
	tz:=defaultString(stringValue(triggerInput["timezone"]),"Asia/Shanghai");if _,err:=time.LoadLocation(tz);err!=nil{return nil,"时区无效"};trigger:=map[string]any{"type":typ,"timezone":tz}
	switch typ{case "once":at:=stringValue(triggerInput["at"]);if !offsetPattern.MatchString(at){return nil,"一次性时间必须带时区"};parsed,err:=time.Parse(time.RFC3339,at);if err!=nil{return nil,"一次性时间必须带时区"};trigger["at"]=parsed.UTC().Format(time.RFC3339Nano)
	case "daily","weekly":when:=stringValue(triggerInput["time"]);if !hhmmPattern.MatchString(when){return nil,"运行时间格式为 HH:mm"};trigger["time"]=when;if typ=="weekly"{days:=anySlice(triggerInput["weekdays"]);if len(days)==0{return nil,"星期使用 0–6，0 为周日"};values:=[]int{};seen:=map[int]bool{};for _,rawDay:=range days{day:=intValue(rawDay);if floatValue(rawDay)!=float64(day)||day<0||day>6{return nil,"星期使用 0–6，0 为周日"};if !seen[day]{seen[day]=true;values=append(values,day)}};trigger["weekdays"]=values}}
	tables:=s.appTables(ctx,map[string]any{"id":appID,"tenant_id":tenantID},tenantID);grants:=anySlice(asMap(input["scope"])["tables"]);if len(grants)<1||len(grants)>12{return nil,"请明确授权 1–12 张数据表"};scopeTables:=[]map[string]any{};seenTable:=map[string]bool{}
	for _,rawGrant:=range grants{grant,ok:=rawGrant.(map[string]any);if !ok{return nil,"数据表授权必须是对象"};slug:=stringValue(grant["table"]);var table map[string]any;for _,candidate:=range tables{if candidate["slug"]==slug{table=candidate;break}};if table==nil||seenTable[slug]{return nil,"授权的数据表不存在或重复"};seenTable[slug]=true
		allowed:=map[string]bool{};for _,field:=range asSliceMap(table["fields"]){if field["type"]!="file"&&field["type"]!="relation"{allowed[stringValue(field["name"])]=true}}
		read:=anySlice(grant["read_fields"]);write:=anySlice(grant["write_fields"]);if len(read)==0||len(read)>24||len(write)>24{return nil,"需明确授权可读字段；首版后台任务不支持附件或关联字段"};readValues:=[]string{};readSet:=map[string]bool{};for _,v:=range read{name,ok:=v.(string);if !ok||!allowed[name]{return nil,"需明确授权可读字段；首版后台任务不支持附件或关联字段"};if !readSet[name]{readSet[name]=true;readValues=append(readValues,name)}};writeValues:=[]string{};writeSet:=map[string]bool{};for _,v:=range write{name,ok:=v.(string);if !ok||!allowed[name]{return nil,"需明确授权可读字段；首版后台任务不支持附件或关联字段"};if !readSet[name]{return nil,"可写字段也必须授权读取，便于展示修改前后的内容"};if !writeSet[name]{writeSet[name]=true;writeValues=append(writeValues,name)}};scopeTables=append(scopeTables,map[string]any{"table":slug,"read_fields":readValues,"write_fields":writeValues})}
	if typ=="record_created"||typ=="status_changed"{slug:=stringValue(triggerInput["table"]);var table map[string]any;for _,candidate:=range tables{if candidate["slug"]==slug{table=candidate;break}};if table==nil||!seenTable[slug]{return nil,"业务事件必须来自获授权的数据表"};trigger["table"]=slug;if typ=="status_changed"{field:=findField(asSliceMap(table["fields"]),stringValue(triggerInput["field"]));grantRead:=false;for _,grant:=range scopeTables{if grant["table"]==slug{grantRead=contains(grant["read_fields"],stringValue(asMap(field)["name"]))}};from,to:=triggerInput["from"],triggerInput["to"];if field==nil||field["type"]!="select"||!grantRead||!contains(field["options"],from)&&!(from==""&&!boolValue(field["required"]))||!contains(field["options"],to)&&!(to==""&&!boolValue(field["required"]))||equalJSON(from,to){return nil,"状态变化条件无效"};trigger["field"],trigger["from"],trigger["to"]=field["name"],from,to}}
	scopeInput:=asMap(input["scope"]);recipients:=anySlice(scopeInput["recipient_ids"]);if len(recipients)>10{return nil,"接收人最多 10 位"};recipientIDs:=[]string{};recipientSeen:=map[string]bool{};for _,v:=range recipients{uid,ok:=v.(string);if !ok||uid==""{return nil,"接收人最多 10 位"};if _,err:=s.PB.Find(ctx,"tenant_members",listFilter("tenant_id = "+pbFilterString(tenantID),"user_id = "+pbFilterString(uid)));err!=nil{return nil,"接收人必须是当前工作区成员"};if !recipientSeen[uid]{recipientSeen[uid]=true;recipientIDs=append(recipientIDs,uid)}}
	execution:=defaultString(stringValue(input["execution"]),"agent");if execution!="agent"&&execution!="report"{return nil,"执行方式为 agent 或 report"};limitsInput:=asMap(input["limits"]);limits:=map[string]any{};for key,spec:=range map[string][3]int{"max_writes":{10,0,100},"max_requests":{12,1,30},"timeout_seconds":{180,30,600},"confirmation_timeout_hours":{72,1,720}}{value:=spec[0];if raw,ok:=limitsInput[key];ok{value=intValue(raw);if floatValue(raw)!=float64(value)||value<spec[1]||value>spec[2]{return nil,fmt.Sprintf("运行限制必须是 %d–%d 的整数",spec[1],spec[2])}};limits[key]=value}
	return map[string]any{"goal":goal,"execution":execution,"trigger":trigger,"scope":map[string]any{"tables":scopeTables,"recipient_ids":recipientIDs},"limits":limits},""
}

func nextScheduledRun(trigger map[string]any,after time.Time)string{
	typ:=stringValue(trigger["type"]);if typ=="manual"||typ=="record_created"||typ=="status_changed"{return ""};if typ=="once"{at:=parseTime(trigger["at"]);if at.After(after){return at.UTC().Format(time.RFC3339Nano)};return ""};loc,err:=time.LoadLocation(defaultString(stringValue(trigger["timezone"]),"Asia/Shanghai"));if err!=nil{return ""};hourMinute:=strings.Split(stringValue(trigger["time"]),":");if len(hourMinute)!=2{return ""};hour,_:=strconv.Atoi(hourMinute[0]);minute,_:=strconv.Atoi(hourMinute[1]);weekdays:=map[int]bool{};for _,raw:=range anySlice(trigger["weekdays"]){weekdays[intValue(raw)]=true};local:=after.In(loc)
	for day:=0;day<10;day++{date:=time.Date(local.Year(),local.Month(),local.Day()+day,0,0,0,0,loc);if typ=="weekly"&&!weekdays[int(date.Weekday())]{continue};candidate:=time.Date(date.Year(),date.Month(),date.Day(),hour,minute,0,0,loc);actual:=candidate.In(loc);if actual.Year()!=date.Year()||actual.Month()!=date.Month()||actual.Day()!=date.Day()||actual.Hour()!=hour||actual.Minute()!=minute{continue};if candidate.After(after){return candidate.UTC().Format(time.RFC3339Nano)}};return ""
}
