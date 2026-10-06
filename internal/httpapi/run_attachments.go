package httpapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/tans/miao/internal/harness"
)

func (s *Server) addRunAttachments(ctx context.Context, actor executionActor, input, runContext map[string]any) error {
	attachments := anySlice(runContext["attachments"])
	if raw, exists := input["attachment_ids"]; exists && !isUIArray(raw) {
		return businessError(400, "附件引用必须为数组")
	}
	if len(anySlice(input["attachment_ids"])) > 4 {
		return businessError(400, "每轮最多 4 份附件")
	}
	if raw, exists := input["attachment"]; exists {
		item := asMap(raw)
		name, text := stringValue(item["name"]), stringValue(item["text"])
		ext := strings.ToLower(filepath.Ext(name))
		if !containsString([]string{".txt", ".md", ".csv"}, ext) || len(name) > 240 || text == "" || len(text) > 64000 || !utf8.ValidString(text) {
			return businessError(400, "工作区附件只支持不超过 64 KB 的 UTF-8 文本、Markdown 或 CSV；其他文件请先选择应用上传")
		}
		content, err := readSpreadsheet([]byte(text), name, "")
		if err != nil {
			return businessError(400, err.Error())
		}
		data, _ := json.Marshal(content)
		attachments = append(attachments, map[string]any{"name": filepath.Base(name), "excerpt": clip(string(data), 12000), "source": "user_attachment", "bounded_sample": true})
	}
	for _, rawID := range anySlice(input["attachment_ids"]) {
		fileID, ok := rawID.(string)
		if !ok || actor.AppID == "" {
			return businessError(400, "附件引用需要已选择的应用")
		}
		file, err := s.PB.Get(ctx, "app_files", fileID)
		if err != nil || file["tenant_id"] != actor.TenantID || file["app_id"] != actor.AppID || file["user_id"] != actor.UserID {
			return businessError(404, "附件不存在或不属于当前用户和应用")
		}
		name := stringValue(file["name"])
		item := map[string]any{"id": fileID, "name": name, "source": "app_file"}
		if containsString([]string{".txt", ".md", ".csv", ".xlsx"}, strings.ToLower(filepath.Ext(name))) {
			data, _, _, err := s.appFileBytes(ctx, file)
			if err != nil {
				return err
			}
			content, err := readSpreadsheet(data, name, "")
			if err != nil {
				return businessError(400, err.Error())
			}
			excerpt, _ := json.Marshal(content)
			item["excerpt"], item["bounded_sample"] = clip(string(excerpt), 12000), true
		} else {
			item["note"] = "已上传附件；不提供 OCR、图片理解或 PDF 文本提取。只能使用用户明确提供的内容或作为指定记录附件。"
		}
		attachments = append(attachments, item)
	}
	if len(attachments) > 4 {
		return businessError(400, "每轮最多 4 份附件，请新建运行后继续")
	}
	runContext["attachments"] = attachments
	return nil
}

func (s *Server) validateRunAttachments(ctx context.Context, run *harness.Run) error {
	for _, item := range asSliceMap(asMap(run.Context)["attachments"]) {
		if item["source"] != "app_file" {
			continue
		}
		file, err := s.PB.Get(ctx, "app_files", stringValue(item["id"]))
		if err != nil || file["tenant_id"] != run.TenantID || file["app_id"] != run.AppID || file["user_id"] != run.UserID {
			return businessError(409, "运行附件引用已失效；请重新上传后继续")
		}
	}
	return nil
}
