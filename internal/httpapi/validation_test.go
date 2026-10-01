package httpapi

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"
)

func TestSpreadsheetScalarTypesAndLimits(t *testing.T) {
	csv, err := readSpreadsheet([]byte("姓名,金额\r\n\"张,三\",0\r\n"), "客户.csv", "")
	if err != nil {
		t.Fatal(err)
	}
	rows := asSliceMap(csv["rows"])
	if len(rows) != 1 || rows[0]["姓名"] != "张,三" || rows[0]["金额"] != "0" {
		t.Fatal(csv)
	}
	if _, err := readSpreadsheet([]byte("姓名,姓名\na,b"), "bad.csv", ""); err == nil {
		t.Fatal("duplicate headers accepted")
	}
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for name, value := range map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="客户" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>name</t></is></c><c r="B1" t="inlineStr"><is><t>value</t></is></c><c r="C1" t="inlineStr"><is><t>active</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>张三</t></is></c><c r="B2"><v>0</v></c><c r="C2" t="b"><v>0</v></c></row></sheetData></worksheet>`,
	} {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	xlsx, err := readSpreadsheet(body.Bytes(), "客户.xlsx", "客户")
	if err != nil {
		t.Fatal(err)
	}
	rows = asSliceMap(xlsx["rows"])
	if len(rows) != 1 || rows[0]["name"] != "张三" || rows[0]["value"] != float64(0) || rows[0]["active"] != false {
		t.Fatal(xlsx)
	}
	if _, err := readSpreadsheet(body.Bytes(), "客户.xlsx", "missing"); err == nil {
		t.Fatal("missing sheet accepted")
	}
	var big bytes.Buffer
	big.WriteString("name\n")
	for i := 0; i < 105; i++ {
		big.WriteString("row\n")
	}
	limited, err := readSpreadsheet(big.Bytes(), "big.csv", "")
	if err != nil || len(asSliceMap(limited["rows"])) != 100 || limited["truncated"] != true {
		t.Fatalf("CSV limit failed: %v %v", limited, err)
	}
}
func TestUIDefinitionValidationAndNativeSlices(t *testing.T) {
	tables := []map[string]any{{"slug": "customers", "fields": []map[string]any{{"name": "name", "type": "text", "required": true}, {"name": "status", "type": "select", "options": []string{"new", "done"}}, {"name": "file", "type": "file"}}}}
	definition := map[string]any{"schema_version": 2, "title": "客户", "pages": []map[string]any{{"id": "customers", "title": "客户", "collection": "customers", "fields": []string{"name", "status", "file"}, "actions": []map[string]any{{"id": "complete", "label": "完成", "set": map[string]any{"status": "done"}}}}}}
	normalized, message := validateAppUIDefinition(definition, tables)
	if message != "" || len(appUIPages(normalized)) != 1 {
		t.Fatalf("native slices lost: %v %s", normalized, message)
	}
	for _, bad := range []map[string]any{
		{"schema_version": 2, "title": "客户", "pages": definition["pages"], "script": "alert(1)"},
		{"schema_version": 2, "title": "客户", "pages": []any{appUIPages(normalized)[0], appUIPages(normalized)[0]}},
		{"schema_version": 2, "title": "客户", "pages": []any{map[string]any{"id": "customers", "title": "客户", "collection": "customers", "fields": []string{"missing"}}}},
	} {
		if _, message := validateAppUIDefinition(bad, tables); message == "" {
			t.Fatal("invalid definition accepted", bad)
		}
	}
}
func TestPocketBaseTimestampDoesNotExpireFreshPlan(t *testing.T) {
	for _, value := range []string{"2026-10-01 10:00:00.123Z", "2026-10-01T10:00:00.123Z"} {
		parsed := parseTime(value)
		if parsed.IsZero() || parsed.Nanosecond() != 123000000 {
			t.Fatal(value, parsed)
		}
	}
}
func TestScheduleDaylightSavingBoundaries(t *testing.T) {
	for _, check := range []struct {
		name, now, want string
		trigger         map[string]any
	}{
		{"spring skip", "2026-03-08T05:00:00Z", "2026-03-09T06:30:00Z", map[string]any{"type": "daily", "timezone": "America/New_York", "time": "02:30"}},
		{"fall first", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z", map[string]any{"type": "daily", "timezone": "America/New_York", "time": "01:30"}},
	} {
		t.Run(check.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, check.now)
			if err != nil {
				t.Fatal(err)
			}
			actual := parseTime(nextScheduledRun(check.trigger, now))
			want := parseTime(check.want)
			if !actual.Equal(want) {
				t.Fatalf("scheduled=%v want=%v", actual, want)
			}
		})
	}
}
