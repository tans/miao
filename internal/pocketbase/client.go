package pocketbase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Record = map[string]any

type Error struct {
	Status  int
	Message string
	Data    Record
}

func (e *Error) Error() string { return e.Message }

type Upload struct {
	Name, Filename, ContentType string
	Data                        []byte
}

type Client struct {
	base, email, password string
	http                  *http.Client
	mu                    sync.Mutex
	token                 string
	tokenUntil            time.Time
}

func New(base, email, password string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), email: email, password: password,
		http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) adminToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.tokenUntil) > time.Minute {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"identity": c.email, "password": c.password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/collections/_superusers/auth-with-password", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || result.Token == "" {
		return "", fmt.Errorf("PocketBase admin authentication failed (%d)", res.StatusCode)
	}
	c.token, c.tokenUntil = result.Token, time.Now().Add(12*time.Hour)
	return c.token, nil
}

func (c *Client) Do(ctx context.Context, method, endpoint string, query url.Values, body any, userToken string, headers http.Header) (Record, []byte, int, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, 0, err
	}
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	token := userToken
	if token == "" {
		token, err = c.adminToken(ctx)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	req.Header.Set("Authorization", token)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, nil, res.StatusCode, err
	}
	var record Record
	if len(data) > 0 {
		_ = json.Unmarshal(data, &record)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := http.StatusText(res.StatusCode)
		if value, ok := record["message"].(string); ok && value != "" {
			message = value
		}
		return record, data, res.StatusCode, &Error{Status: res.StatusCode, Message: message, Data: record}
	}
	return record, data, res.StatusCode, nil
}

func (c *Client) Request(ctx context.Context, method, endpoint string, query url.Values, body any, token string) (Record, error) {
	row, _, _, err := c.Do(ctx, method, endpoint, query, body, token, nil)
	return row, err
}

func (c *Client) RequestHeaders(ctx context.Context, method, endpoint string, query url.Values, body any, token string, headers map[string]string) (Record, error) {
	h := http.Header{}
	for key, value := range headers {
		h.Set(key, value)
	}
	row, _, _, err := c.Do(ctx, method, endpoint, query, body, token, h)
	return row, err
}

func collectionPath(collection string) string {
	return path.Join("/api/collections", url.PathEscape(collection), "records")
}

func (c *Client) List(ctx context.Context, collection, filter, sort string, page, perPage int) ([]Record, int, int, error) {
	query := url.Values{"page": {strconv.Itoa(page)}, "perPage": {strconv.Itoa(perPage)}}
	if filter != "" {
		query.Set("filter", filter)
	}
	if sort != "" {
		query.Set("sort", sort)
	}
	data, err := c.Request(ctx, http.MethodGet, collectionPath(collection), query, nil, "")
	if err != nil {
		return nil, 0, 0, err
	}
	items, _ := data["items"].([]any)
	rows := make([]Record, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows, asInt(data["totalItems"]), asInt(data["totalPages"]), nil
}

func (c *Client) ListAll(ctx context.Context, collection, filter, sort string) ([]Record, error) {
	rows := []Record{}
	for page := 1; page <= 10000; page++ {
		items, _, pages, err := c.List(ctx, collection, filter, sort, page, 500)
		if err != nil {
			return nil, err
		}
		rows = append(rows, items...)
		if page >= pages {
			return rows, nil
		}
	}
	return nil, errors.New("PocketBase pagination limit exceeded")
}

func (c *Client) Get(ctx context.Context, collection, id string) (Record, error) {
	return c.Request(ctx, http.MethodGet, collectionPath(collection)+"/"+url.PathEscape(id), nil, nil, "")
}
func (c *Client) Find(ctx context.Context, collection, filter string) (Record, error) {
	rows, _, _, err := c.List(ctx, collection, filter, "", 1, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &Error{Status: 404, Message: "record not found"}
	}
	return rows[0], nil
}
func (c *Client) Create(ctx context.Context, collection string, body Record) (Record, error) {
	return c.Request(ctx, http.MethodPost, collectionPath(collection), nil, body, "")
}
func (c *Client) Update(ctx context.Context, collection, id string, body Record) (Record, error) {
	return c.Request(ctx, http.MethodPatch, collectionPath(collection)+"/"+url.PathEscape(id), nil, body, "")
}
func (c *Client) Delete(ctx context.Context, collection, id string) error {
	_, err := c.Request(ctx, http.MethodDelete, collectionPath(collection)+"/"+url.PathEscape(id), nil, nil, "")
	return err
}

func (c *Client) Collection(ctx context.Context, name string) (Record, error) {
	return c.Request(ctx, http.MethodGet, "/api/collections/"+url.PathEscape(name), nil, nil, "")
}
func (c *Client) CreateCollection(ctx context.Context, schema Record) (Record, error) {
	return c.Request(ctx, http.MethodPost, "/api/collections", nil, schema, "")
}
func (c *Client) UpdateCollection(ctx context.Context, name string, schema Record) (Record, error) {
	return c.Request(ctx, http.MethodPatch, "/api/collections/"+url.PathEscape(name), nil, schema, "")
}
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	_, err := c.Request(ctx, http.MethodDelete, "/api/collections/"+url.PathEscape(name), nil, nil, "")
	return err
}

func (c *Client) UpdateBusiness(ctx context.Context, collection, id string, body Record, expected, actor, source string) (Record, error) {
	return c.RequestHeaders(ctx, http.MethodPatch, collectionPath(collection)+"/"+url.PathEscape(id), nil, body, "", map[string]string{"X-Miao-Expected-Updated": expected, "X-Miao-Actor-Id": actor, "X-Miao-Event-Source": source})
}

func (c *Client) UploadBusiness(ctx context.Context, collection, id string, values Record, files []Upload, expected, actor, source string) (Record, error) {
	return c.upload(ctx, http.MethodPatch, collection, id, values, files, map[string]string{"X-Miao-Expected-Updated": expected, "X-Miao-Actor-Id": actor, "X-Miao-Event-Source": source})
}
func (c *Client) UploadNew(ctx context.Context, collection string, values Record, files []Upload) (Record, error) {
	return c.upload(ctx, http.MethodPost, collection, "", values, files, nil)
}
func (c *Client) upload(ctx context.Context, method, collection, id string, values Record, files []Upload, headers map[string]string) (Record, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for key, value := range values {
		// PocketBase expects scalar form values as their plain representation. JSON
		// quoting strings here silently stores quotes in text fields.
		if value == nil {
			continue
		}
		var field string
		switch typed := value.(type) {
		case string:
			field = typed
		case bool:
			field = strconv.FormatBool(typed)
		case float64:
			field = strconv.FormatFloat(typed, 'f', -1, 64)
		case int:
			field = strconv.Itoa(typed)
		case int64:
			field = strconv.FormatInt(typed, 10)
		default:
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			field = string(encoded)
		}
		if err := w.WriteField(key, field); err != nil {
			return nil, err
		}
	}
	for _, file := range files {
		part, err := w.CreateFormFile(file.Name, file.Filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(file.Data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	endpoint := collectionPath(collection)
	if id != "" {
		endpoint += "/" + url.PathEscape(id)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+endpoint, &body)
	if err != nil {
		return nil, err
	}
	token, err := c.adminToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	var row Record
	_ = json.Unmarshal(data, &row)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &Error{Status: res.StatusCode, Message: string(rowString(row, "message")), Data: row}
	}
	return row, nil
}

// ProtectedFile downloads an attachment from a protected PocketBase field.
// The short lived PB file token is kept server-side and never exposed to the UI.
func (c *Client) ProtectedFile(ctx context.Context, collection, recordID, filename string) ([]byte, string, string, error) {
	tokenData, err := c.Request(ctx, http.MethodPost, "/api/files/token", nil, map[string]any{}, "")
	if err != nil {
		return nil, "", "", err
	}
	token := rowString(tokenData, "token")
	if token == "" {
		return nil, "", "", errors.New("PocketBase file token missing")
	}
	endpoint := "/api/files/" + url.PathEscape(collection) + "/" + url.PathEscape(recordID) + "/" + url.PathEscape(filename)
	query := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, "", "", err
	}
	admin, err := c.adminToken(ctx)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("Authorization", admin)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("PocketBase protected file request failed (%d)", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 6<<20))
	if err != nil {
		return nil, "", "", err
	}
	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	name := filename
	if disposition := res.Header.Get("Content-Disposition"); disposition != "" {
		if _, params, parseErr := mime.ParseMediaType(disposition); parseErr == nil && params["filename"] != "" {
			name = params["filename"]
		}
	}
	return data, contentType, name, nil
}

func (c *Client) AuthRefresh(ctx context.Context, token string) (Record, error) {
	return c.Request(ctx, http.MethodPost, "/api/collections/users/auth-refresh", nil, map[string]any{}, token)
}
func (c *Client) AuthPassword(ctx context.Context, email, password string) (Record, error) {
	return c.Request(ctx, http.MethodPost, "/api/collections/users/auth-with-password", nil, map[string]string{"identity": email, "password": password}, "")
}
func (c *Client) Send(ctx context.Context, method, endpoint string, body any, token string) (Record, error) {
	return c.Request(ctx, method, endpoint, nil, body, token)
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}
func rowString(row Record, key string) string {
	if value, ok := row[key].(string); ok {
		return value
	}
	return ""
}
