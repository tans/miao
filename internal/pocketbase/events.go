package pocketbase

import (
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

var businessCollection = regexp.MustCompile(`^app_[a-z0-9]+_`)

// BindBusinessEvents protects every core save path, including background writes.
// The record, its audit history, and durable task events share one transaction.
func BindBusinessEvents(app core.App) {
	app.OnRecordCreate().BindFunc(func(e *core.RecordEvent) error { return commitBusinessEvent(e, "created") })
	app.OnRecordUpdate().BindFunc(func(e *core.RecordEvent) error { return commitBusinessEvent(e, "updated") })
}

func commitBusinessEvent(e *core.RecordEvent, event string) error {
	name := e.Record.Collection().Name
	if !businessCollection.MatchString(name) {
		return e.Next()
	}
	originalApp := e.App
	defer func() { e.App = originalApp }()
	return originalApp.RunInTransaction(func(tx core.App) error {
		e.App = tx
		metadata, err := tx.FindFirstRecordByData("app_collections", "pb_collection", name)
		if err != nil {
			return err
		}
		var before *core.Record
		if event == "updated" {
			before, err = tx.FindRecordById(name, e.Record.Id)
			if err != nil {
				return err
			}
			expected := e.Record.GetString("__miao_expected_updated")
			if expected == "" {
				expected = e.Record.Original().GetString("updated")
			}
			if expected != before.GetString("updated") {
				return &Error{Status: 409, Message: "记录已变化，请重新读取后操作"}
			}
			if before.GetString("tenant_id") != e.Record.GetString("tenant_id") || before.GetString("app_id") != e.Record.GetString("app_id") {
				return &Error{Status: 403, Message: "不能改变记录归属"}
			}
			// Millisecond timestamps must advance even for two saves in one tick.
			now := time.Now().UTC().Truncate(time.Millisecond)
			if !now.After(before.GetDateTime("updated").Time()) {
				now = before.GetDateTime("updated").Time().Add(time.Millisecond)
			}
			updated, _ := types.ParseDateTime(now)
			e.Record.SetRaw("updated", updated)
		}
		if metadata.GetString("tenant_id") != e.Record.GetString("tenant_id") || metadata.GetString("app_id") != e.Record.GetString("app_id") {
			return &Error{Status: 403, Message: "记录归属无效"}
		}
		if err := e.Next(); err != nil {
			return err
		}
		var fields []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}
		if err := metadata.UnmarshalJSONField("fields", &fields); err != nil {
			return err
		}
		previous, next := Record{}, Record{}
		for _, field := range fields {
			if field.Type == "file" {
				continue
			}
			next[field.Name] = e.Record.Get(field.Name)
			if before != nil {
				previous[field.Name] = before.Get(field.Name)
			}
		}
		changed := before == nil || !reflect.DeepEqual(previous, next)
		if changed {
			if before != nil {
				previous["id"], previous["updated"] = before.Id, before.GetString("updated")
			}
			next["id"], next["updated"] = e.Record.Id, e.Record.GetString("updated")
			collection, err := tx.FindCachedCollectionByNameOrId("miao_record_changes")
			if err != nil {
				return err
			}
			change := core.NewRecord(collection)
			source := e.Record.GetString("__miao_source")
			if source == "" {
				source = "interactive"
			}
			processed := e.Record.GetBool("__miao_skip_events")
			change.Load(Record{"tenant_id": metadata.GetString("tenant_id"), "app_id": metadata.GetString("app_id"), "table": metadata.GetString("slug"), "record_id": e.Record.Id, "actor_id": e.Record.GetString("__miao_actor_id"), "source": source, "before": previous, "after": next, "event": event, "automation_processed": processed})
			if err := tx.SaveWithContext(e.Context, change); err != nil {
				return err
			}
		}
		if e.Record.GetBool("__miao_skip_events") {
			return nil
		}
		params := dbx.Params{"tenant": metadata.GetString("tenant_id"), "app": metadata.GetString("app_id")}
		for offset := 0; ; offset += 200 {
			tasks, err := tx.FindRecordsByFilter("miao_tasks", `tenant_id = {:tenant} && app_id = {:app} && status = "enabled"`, "id", 200, offset, params)
			if err != nil {
				return err
			}
			for _, task := range tasks {
				var definition struct {
					Goal      string `json:"goal"`
					Execution string `json:"execution"`
					Trigger   Record `json:"trigger"`
					Scope     Record `json:"scope"`
					Limits    Record `json:"limits"`
				}
				if err := task.UnmarshalJSONField("definition", &definition); err != nil {
					return err
				}
				trigger := definition.Trigger
				if trigger["table"] != metadata.GetString("slug") {
					continue
				}
				matches := trigger["type"] == "record_created" && event == "created"
				if trigger["type"] == "status_changed" && event == "updated" && before != nil {
					field := rowString(trigger, "field")
					matches = trigger["from"] != trigger["to"] && before.GetString(field) == rowString(trigger, "from") && e.Record.GetString(field) == rowString(trigger, "to")
				}
				if !matches {
					continue
				}
				// Preserve the existing JS hook key for installations with queued runs.
				key := fmt.Sprintf("record:%d:%s:%s:%s", task.GetInt("revision"), event, e.Record.Id, e.Record.GetString("updated"))
				existing, err := tx.FindRecordsByFilter("miao_runs", "task_id = {:task} && event_key = {:key}", "", 1, 0, dbx.Params{"task": task.Id, "key": key})
				if err != nil {
					return err
				}
				if len(existing) > 0 {
					continue
				}
				collection, err := tx.FindCachedCollectionByNameOrId("miao_runs")
				if err != nil {
					return err
				}
				run := core.NewRecord(collection)
				run.Load(Record{"tenant_id": params["tenant"], "app_id": params["app"], "task_id": task.Id, "created_by": task.GetString("created_by"), "event_key": key, "status": "queued", "delivery_status": "pending", "snapshot": Record{"goal": definition.Goal, "execution": definition.Execution, "trigger": definition.Trigger, "scope": definition.Scope, "limits": definition.Limits, "name": task.GetString("name"), "revision": task.GetInt("revision"), "input": Record{"event": event, "table": metadata.GetString("slug"), "record_id": e.Record.Id, "updated_at": e.Record.GetString("updated")}}})
				if err := tx.SaveWithContext(e.Context, run); err != nil {
					return err
				}
			}
			if len(tasks) < 200 {
				return nil
			}
		}
	})
}
