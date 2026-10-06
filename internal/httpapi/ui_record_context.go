package httpapi

import (
	"context"

	"github.com/tans/miao/internal/pocketbase"
)

func validateUIRecordContexts(spec map[string]any, sources, tables []map[string]any) string {
	primaryID := uiDetailSource(map[string]any{"spec": spec})
	var primary map[string]any
	for _, source := range sources {
		if source["id"] == primaryID {
			primary = source
		}
	}
	for _, source := range sources {
		if source["context"] == nil {
			continue
		}
		binding := asMap(source["context"])
		if primary == nil || source["id"] == primaryID || binding["source"] != primaryID || primary["context"] != nil {
			return "关联上下文只能引用当前页的主记录详情，不支持循环或跨页引用"
		}
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == source["collection"] {
				table = candidate
			}
		}
		field := findField(asSliceMap(table["fields"]), stringValue(binding["field"]))
		if field == nil || field["type"] != "relation" || field["target"] != primary["collection"] {
			return "关联上下文字段必须引用主记录详情的数据表"
		}
	}
	return ""
}

func uiContextRecord(ctx context.Context, pb *pocketbase.Client, app, page map[string]any, tables []map[string]any, recordID string) (map[string]any, error) {
	if recordID == "" {
		return nil, nil
	}
	primaryID := uiDetailSource(page)
	for _, source := range asSliceMap(page["data_sources"]) {
		if source["id"] != primaryID {
			continue
		}
		for _, table := range tables {
			if table["slug"] != source["collection"] {
				continue
			}
			filters := []string{"tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"])), "id = " + pbFilterString(recordID)}
			filters = append(filters, uiQueryFilter(source)...)
			row, err := pb.Find(ctx, stringValue(table["pb_collection"]), listFilter(filters...))
			if isMissing(err) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			label := stringValue(row["id"])
			for _, field := range asSliceMap(table["fields"]) {
				if field["type"] == "text" && stringValue(row[stringValue(field["name"])]) != "" {
					label = stringValue(row[stringValue(field["name"])])
					break
				}
			}
			return map[string]any{"id": row["id"], "label": label}, nil
		}
	}
	return nil, nil
}

func appendContextOption(options []map[string]any, contextRecord map[string]any) []map[string]any {
	if contextRecord == nil {
		return options
	}
	for _, option := range options {
		if option["id"] == contextRecord["id"] {
			return options
		}
	}
	return append(options, contextRecord)
}

func addRelatedDetailSources(page, primaryTable map[string]any, tables []map[string]any) {
	elements := asMap(asMap(page["spec"])["elements"])
	section := asMap(elements["section"])
	for _, table := range tables {
		if table["slug"] == primaryTable["slug"] || len(anySlice(page["data_sources"])) >= 12 {
			continue
		}
		for _, field := range asSliceMap(table["fields"]) {
			if field["type"] != "relation" || field["target"] != primaryTable["slug"] {
				continue
			}
			id := backendOpaqueID("related_", stringValue(table["slug"]), stringValue(field["name"]))
			fields := []any{}
			for _, field := range asSliceMap(table["fields"]) {
				fields = append(fields, field["name"])
			}
			source := map[string]any{"id": id, "collection": table["slug"], "fields": fields, "actions": []any{}, "context": map[string]any{"source": "records", "field": field["name"]}}
			page["data_sources"] = append(anySlice(page["data_sources"]), source)
			elements[id] = map[string]any{"type": "RecordCards", "props": map[string]any{"source": id, "title": table["name"]}, "children": []any{}}
			section["children"] = append(anySlice(section["children"]), id)
			break
		}
	}
}
