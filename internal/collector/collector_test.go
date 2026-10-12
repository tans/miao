package collector

import "testing"

func TestScheduleRejectsUnknownType(t *testing.T) {
	_, msg := NormalizeSchedule(map[string]any{"type": "hourly"})
	if msg == "" {
		t.Fatal("未知 schedule 类型应被拒绝")
	}
}

func TestScheduleNormalizesOnce(t *testing.T) {
	out, msg := NormalizeSchedule(map[string]any{"type": "once", "at": "2026-01-01T09:00:00+08:00"})
	if msg != "" {
		t.Fatalf("一次性 schedule 应通过，得到：%s", msg)
	}
	if out["type"] != "once" {
		t.Fatal("schedule 类型丢失")
	}
}

func TestScheduleRequiresWeekdays(t *testing.T) {
	_, msg := NormalizeSchedule(map[string]any{"type": "weekly", "time": "09:00"})
	if msg == "" {
		t.Fatal("weekly 缺 weekdays 应被拒绝")
	}
}

func TestFilterMatchEq(t *testing.T) {
	row := map[string]any{"name": "alpha", "count": 3}
	if !FilterMatch(row, []any{map[string]any{"field": "name", "operator": "eq", "value": "alpha"}}) {
		t.Fatal("eq 应匹配")
	}
	if FilterMatch(row, []any{map[string]any{"field": "name", "operator": "neq", "value": "alpha"}}) {
		t.Fatal("neq 不应匹配相等值")
	}
}

func TestFilterMatchNotPresent(t *testing.T) {
	row := map[string]any{"name": "alpha"}
	if !FilterMatch(row, []any{map[string]any{"field": "missing", "operator": "neq", "value": "x"}}) {
		t.Fatal("字段缺失时 neq 应匹配（缺失不等于该值）")
	}
	if FilterMatch(row, []any{map[string]any{"field": "missing", "operator": "eq", "value": "x"}}) {
		t.Fatal("字段缺失时 eq 不应匹配")
	}
	if !FilterMatch(row, []any{map[string]any{"field": "missing", "operator": "is_empty"}}) {
		t.Fatal("字段缺失时 is_empty 应匹配")
	}
}

func TestConvertNumber(t *testing.T) {
	n, ok := Convert("42", "number")
	if !ok || n != float64(42) {
		t.Fatalf("字符串 42 应转为数字，得到 %v %v", n, ok)
	}
}

func TestConvertBool(t *testing.T) {
	b, ok := Convert("yes", "bool")
	if !ok || b != true {
		t.Fatal("yes 应转为 true")
	}
}

func TestConvertRejectsUnknownType(t *testing.T) {
	if _, ok := Convert("x", "binary"); ok {
		t.Fatal("未知类型应拒绝")
	}
}

func TestDedupKeyStable(t *testing.T) {
	row := map[string]any{"id": 1, "name": "a"}
	if DedupKey(row, []string{"id", "name"}) != DedupKey(row, []string{"id", "name"}) {
		t.Fatal("相同行相同字段应产生相同去重键")
	}
	if DedupKey(row, []string{"id"}) == DedupKey(row, []string{"name"}) {
		t.Fatal("不同字段应产生不同去重键")
	}
}

func TestShouldSkipWritten(t *testing.T) {
	item := map[string]any{"status": "written", "source": map[string]any{"id": 1}}
	row := map[string]any{"id": 1}
	if !ShouldSkip(item, row) {
		t.Fatal("已写入且源相同的行应跳过")
	}
	if ShouldSkip(map[string]any{}, row) {
		t.Fatal("空 item 不应跳过")
	}
}
