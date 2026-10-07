package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

func collectionScriptSameRevision(current, baseline map[string]any) bool {
	return current["tenant_id"] == baseline["tenant_id"] && current["app_id"] == baseline["app_id"] &&
		current["created_by"] == baseline["created_by"] && intValue(current["revision"]) == intValue(baseline["revision"]) &&
		equalJSON(current["definition"], baseline["definition"])
}

func (s *Server) collectionScriptExecutionGuardWith(ctx context.Context, pb *pocketbase.Client, script map[string]any) error {
	if leaseID, ok := ctx.Value(collectionLeaseKey{}).(string); ok {
		lock, err := pb.Get(ctx, "miao_runtime_locks", leaseID)
		if err != nil || lock["owner"] != s.workerID || !parseTime(lock["expires_at"]).After(time.Now()) {
			return businessError(409, "采集执行租约已失效，后续操作停止")
		}
	}
	current, err := pb.Get(ctx, "collection_scripts", stringValue(script["id"]))
	if err != nil {
		return err
	}
	if !collectionScriptSameRevision(current, script) || current["status"] != "enabled" {
		return businessError(409, "采集脚本已暂停或版本已变化；已完成的记录保留，后续写入已停止")
	}
	_, err = s.collectionScriptAuthorityWith(ctx, pb, current)
	return err
}

func collectionScriptSourceKey(row map[string]any, fields []string) (string, error) {
	for _, field := range fields {
		value, present := collectionScriptValue(row, field)
		if !present || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return "", businessError(400, "稳定来源键缺少字段："+field)
		}
		if !isActionValue(value) {
			return "", businessError(400, "稳定来源键必须为标量："+field)
		}
	}
	return collectionScriptDedupKey(row, fields), nil
}

func collectionScriptRequestKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "collection:" + hex.EncodeToString(sum[:])
}

func (s *Server) deliverCollectionScriptNotification(ctx context.Context, baseline map[string]any) (map[string]any, error) {
	var saved map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		notification, err := tx.Get(ctx, "collection_script_notifications", stringValue(baseline["id"]))
		if err != nil {
			return err
		}
		script, err := tx.Get(ctx, "collection_scripts", stringValue(notification["script_id"]))
		if err != nil {
			return err
		}
		if script["tenant_id"] != notification["tenant_id"] || script["app_id"] != notification["app_id"] || script["status"] == "archived" {
			return businessError(403, "通知所属采集脚本已失效")
		}
		app, err := s.collectionScriptAuthorityWith(ctx, tx, script)
		if err != nil {
			return err
		}
		recipient := stringValue(notification["recipient_id"])
		id, err := s.workspaceActor(ctx, tx, executionActor{UserID: recipient, TenantID: stringValue(notification["tenant_id"]), AppID: stringValue(notification["app_id"]), Source: "collection_notification"})
		if err != nil {
			return err
		}
		access, err := applicationAccess(ctx, tx, app, id)
		if err != nil {
			return err
		}
		if access.Role == "" {
			return businessError(403, "通知接收人已失去应用访问权限")
		}
		if notification["status"] == "delivered" {
			saved = notification
			return nil
		}
		key := stringValue(notification["event_key"])
		_, err = tx.Find(ctx, "automation_notifications", listFilter("tenant_id = "+pbFilterString(stringValue(notification["tenant_id"])), "app_id = "+pbFilterString(stringValue(notification["app_id"])), "rule_id = "+pbFilterString(stringValue(script["id"])), "event_key = "+pbFilterString(key), "user_id = "+pbFilterString(recipient)))
		if isMissing(err) {
			_, err = tx.Create(ctx, "automation_notifications", map[string]any{"tenant_id": notification["tenant_id"], "app_id": notification["app_id"], "rule_id": script["id"], "run_id": notification["run_id"], "event_key": key, "user_id": recipient, "message": notification["message"], "read": false})
		}
		if err != nil {
			return err
		}
		saved, err = tx.Update(ctx, "collection_script_notifications", stringValue(notification["id"]), map[string]any{"status": "delivered", "error": ""})
		return err
	})
	if err != nil {
		status := "failed"
		if errStatus(err) == 403 || isMissing(err) {
			status = "blocked"
		}
		finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		// Do not replace a concurrent successful delivery with a stale failure.
		if saveErr := s.PB.Transaction(finalCtx, func(tx *pocketbase.Client) error {
			current, loadErr := tx.Get(finalCtx, "collection_script_notifications", stringValue(baseline["id"]))
			if loadErr != nil {
				return loadErr
			}
			if current["status"] == "delivered" {
				return nil
			}
			_, saveErr := tx.Update(finalCtx, "collection_script_notifications", stringValue(baseline["id"]), map[string]any{"status": status, "error": clip(err.Error(), 1000)})
			return saveErr
		}); saveErr != nil {
			s.Logger.Error("collection notification failure persistence failed", "notification_id", baseline["id"], "error", saveErr)
		}
	}
	return saved, err
}

func collectionScriptMappedData(row map[string]any, mapping map[string]any) (map[string]any, error) {
	data := map[string]any{}
	for name, raw := range mapping {
		spec := asMap(raw)
		value, present := collectionScriptValue(row, stringValue(spec["from"]))
		if !present {
			continue
		}
		converted, ok := collectionScriptConvert(value, stringValue(spec["type"]))
		if !ok {
			return nil, businessError(400, "来源值无法转换："+name)
		}
		data[name] = converted
	}
	return data, nil
}

// The record, dedup receipt and notification outbox commit together. A failed
// receipt cannot leave an untracked record that a later run would create again.
func (s *Server) saveCollectionScriptItem(ctx context.Context, script, run, table, previous, source, data map[string]any, key string, notify bool) (map[string]any, error) {
	actor := executionActor{UserID: stringValue(script["created_by"]), TenantID: stringValue(script["tenant_id"]), AppID: stringValue(script["app_id"]), Source: "collection_script"}
	recordID, updated := stringValue(previous["target_record_id"]), ""
	if recordID != "" {
		record, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), recordID)
		if err != nil {
			return nil, err
		}
		updated = stringValue(record["updated"])
	}
	var item map[string]any
	_, err := s.saveBusinessRecord(ctx, recordWrite{Actor: actor, Table: stringValue(table["slug"]), RecordID: recordID, ExpectedUpdated: updated, Data: data, AllowedFields: mapKeys(data), FieldsSnapshot: asSliceMap(table["fields"])}, func(tx *pocketbase.Client, saved map[string]any) error {
		values := map[string]any{"status": "written", "target_record_id": saved["id"], "last_run_id": run["id"], "source": source, "error": ""}
		var err error
		if previous == nil {
			values["tenant_id"], values["app_id"], values["script_id"], values["dedup_key"] = script["tenant_id"], script["app_id"], script["id"], key
			item, err = tx.Create(ctx, "collection_script_items", values)
		} else {
			item, err = tx.Update(ctx, "collection_script_items", stringValue(previous["id"]), values)
		}
		if err != nil {
			return err
		}
		if !notify || previous != nil {
			return nil
		}
		for _, recipient := range collectionScriptStrings(asMap(run["snapshot"])["recipients"]) {
			if _, err := tx.Create(ctx, "collection_script_notifications", map[string]any{
				"tenant_id": script["tenant_id"], "app_id": script["app_id"], "script_id": script["id"], "run_id": run["id"],
				"item_id": item["id"], "recipient_id": recipient, "event_key": stringValue(script["id"]) + ":" + stringValue(item["id"]) + ":" + recipient,
				"message": stringValue(asMap(run["snapshot"])["name"]) + "：采集脚本新增了一条记录。", "status": "pending", "error": "",
			}); err != nil {
				return err
			}
		}
		return nil
	}, func(tx *pocketbase.Client) error {
		if err := s.collectionScriptExecutionGuardWith(ctx, tx, script); err != nil {
			return err
		}
		current, err := tx.Find(ctx, "collection_script_items", listFilter("script_id = "+pbFilterString(stringValue(script["id"])), "dedup_key = "+pbFilterString(key)))
		if previous == nil && isMissing(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if previous == nil || !equalJSON(current, previous) {
			return businessError(409, "来源键回执已变化，请在下次运行重新读取")
		}
		return nil
	})
	return item, err
}

// Samples stay bounded independently of the page/item budget and storage limit.
func appendCollectionScriptSample(samples []any, item map[string]any) []any {
	if len(samples) >= 25 {
		return samples
	}
	next := append(append([]any{}, samples...), item)
	data, err := json.Marshal(next)
	if err != nil || len(data) > 128<<10 {
		return samples
	}
	return next
}
