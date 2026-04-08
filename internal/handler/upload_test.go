package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUploadFile(t *testing.T) {
	// 创建临时上传目录
	uploadDir := filepath.Join("../../web", "static", "uploads", "test")
	os.MkdirAll(uploadDir, 0755)
	defer os.RemoveAll(filepath.Join("../../web", "static", "uploads", "test"))

	tests := []struct {
		name           string
		fileType       string
		fileName       string
		fileContent    string
		contentType    string
		expectedStatus int
		expectedCode   int
	}{
		{
			name:           "上传图片成功",
			fileType:       "image",
			fileName:       "test.jpg",
			fileContent:    "fake image content",
			contentType:    "image/jpeg",
			expectedStatus: http.StatusOK,
			expectedCode:   CodeSuccess,
		},
		{
			name:           "上传视频成功",
			fileType:       "video",
			fileName:       "test.mp4",
			fileContent:    "fake video content",
			contentType:    "video/mp4",
			expectedStatus: http.StatusOK,
			expectedCode:   CodeSuccess,
		},
		{
			name:           "不支持的图片格式",
			fileType:       "image",
			fileName:       "test.txt",
			fileContent:    "text content",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   40002,
		},
		{
			name:           "不支持的文件类型参数",
			fileType:       "invalid",
			fileName:       "test.jpg",
			fileContent:    "fake content",
			contentType:    "image/jpeg",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   40001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 multipart form
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile("file", tt.fileName)
			assert.NoError(t, err)

			_, err = io.WriteString(part, tt.fileContent)
			assert.NoError(t, err)

			writer.Close()

			// 创建请求
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest("POST", "/api/v1/upload?type="+tt.fileType, body)
			c.Request.Header.Set("Content-Type", writer.FormDataContentType())

			// 模拟认证用户
			c.Set("user_id", int64(1))

			UploadFile(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response Response
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, response.Code)
		})
	}
}

func TestUploadFile_NoFile(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("POST", "/api/v1/upload?type=image", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data")
	c.Set("user_id", int64(1))

	UploadFile(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 40001, response.Code)
}
