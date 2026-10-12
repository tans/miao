package httpapi

import (
	"context"

	"github.com/tans/miao/internal/pocketbase"
	uidoc "github.com/tans/miao/internal/ui"
)




func uiContextRecord(ctx context.Context, pb *pocketbase.Client, app, page map[string]any, tables []map[string]any, recordID string) (map[string]any, error) {
	if recordID == "" {
		return nil, nil
	}
	primaryID := uidoc.DetailSource(page)
	for _, source := range asSliceMap(page["data_sources"]) {
		if source["id"] != primaryID {
			continue
		}
		for _, table := range tables {
			if table["slug"] != source["collection"] {
				continue
			}
			filters := []string{"tenant_id = " + pbFilterString(stringValue(app["tenant_id"])), "app_id = " + pbFilterString(stringValue(app["id"])), "id = " + pbFilterString(recordID)}
			filters = append(filters, uidoc.QueryFilter(source)...)
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






