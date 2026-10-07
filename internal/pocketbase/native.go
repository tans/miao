// Package pocketbase provides server-side persistence. Business authorization
// belongs to the shared HTTP/worker code, not this privileged adapter.
package pocketbase

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"

	"github.com/pocketbase/dbx"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/search"
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
	App  core.App
	auth http.Handler
}

func New(app core.App) (*Client, error) {
	// Authentication keeps PocketBase's rules and token validation. This mux
	// runs in process and is never mounted on the public HTTP server.
	router, err := apis.NewRouter(app)
	if err != nil {
		return nil, err
	}
	handler, err := router.BuildMux()
	if err != nil {
		return nil, err
	}
	return &Client{App: app, auth: handler}, nil
}
func recordMap(record *core.Record) Record {
	return jsonMap(record.IgnoreEmailVisibility(true).WithCustomData(false).PublicExport())
}
func jsonMap(value any) Record {
	data, _ := json.Marshal(value)
	var result Record
	_ = json.Unmarshal(data, &result)
	return result
}
func persistenceError(err error) error {
	var api *Error
	if errors.As(err, &api) {
		return api
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &Error{Status: 404, Message: "record not found"}
	}
	var invalid validation.Errors
	if errors.As(err, &invalid) {
		return &Error{Status: 400, Message: "record validation failed", Data: jsonMap(invalid)}
	}
	return err
}
func (c *Client) query(ctx context.Context, name, filter, sort string) (*dbx.SelectQuery, error) {
	collection, err := c.App.FindCachedCollectionByNameOrId(name)
	if err != nil {
		return nil, fmt.Errorf("collection %s lookup failed: %w", name, err)
	}
	query := c.App.RecordQuery(collection).WithContext(ctx)
	resolver := core.NewRecordFieldResolver(c.App, collection, nil, true)
	if filter != "" {
		expr, err := search.FilterData(filter).BuildExpr(resolver)
		if err != nil {
			return nil, err
		}
		query.AndWhere(expr)
	}
	for _, field := range search.ParseSortFromString(sort) {
		if sort == "" {
			break
		}
		expr, err := field.BuildExpr(resolver)
		if err != nil {
			return nil, err
		}
		if expr != "" {
			query.AndOrderBy(expr)
		}
	}
	if err := resolver.UpdateQuery(query); err != nil {
		return nil, err
	}
	return query, nil
}
func (c *Client) List(ctx context.Context, collection, filter, sort string, page, perPage int) ([]Record, int, int, error) {
	page, perPage = max(1, page), max(1, min(100000, perPage))
	query, err := c.query(ctx, collection, filter, "")
	if err != nil {
		return nil, 0, 0, err
	}
	var total int
	if err := query.Select("count(*)").Row(&total); err != nil {
		return nil, 0, 0, err
	}
	query, err = c.query(ctx, collection, filter, sort)
	if err != nil {
		return nil, 0, 0, err
	}
	var records []*core.Record
	if err := query.Limit(int64(perPage)).Offset(int64(page-1) * int64(perPage)).All(&records); err != nil {
		return nil, 0, 0, err
	}
	rows := make([]Record, 0, len(records))
	for _, record := range records {
		rows = append(rows, recordMap(record))
	}
	return rows, total, (total + perPage - 1) / perPage, nil
}
func (c *Client) ListAll(ctx context.Context, collection, filter, sort string) ([]Record, error) {
	query, err := c.query(ctx, collection, filter, sort)
	if err != nil {
		return nil, err
	}
	var records []*core.Record
	if err := query.All(&records); err != nil {
		return nil, err
	}
	rows := make([]Record, 0, len(records))
	for _, record := range records {
		rows = append(rows, recordMap(record))
	}
	return rows, nil
}
func (c *Client) nativeRecord(ctx context.Context, collection, id string) (*core.Record, error) {
	var record core.Record
	err := c.App.RecordQuery(collection).WithContext(ctx).AndWhere(dbx.HashExp{"id": id}).One(&record)
	return &record, persistenceError(err)
}
func (c *Client) Get(ctx context.Context, collection, id string) (Record, error) {
	record, err := c.nativeRecord(ctx, collection, id)
	if err != nil {
		return nil, err
	}
	return recordMap(record), nil
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
func (c *Client) save(ctx context.Context, collection, id string, body Record, files []Upload, expected, actor, source string) (Record, error) {
	var record *core.Record
	var err error
	if id == "" {
		schema, err := c.App.FindCachedCollectionByNameOrId(collection)
		if err != nil {
			return nil, persistenceError(err)
		}
		record = core.NewRecord(schema)
	} else {
		record, err = c.nativeRecord(ctx, collection, id)
		if err != nil {
			return nil, err
		}
	}
	form := forms.NewRecordUpsert(c.App, record)
	form.SetContext(ctx)
	form.GrantSuperuserAccess()
	form.Load(body)
	for _, upload := range files {
		file, err := filesystem.NewFileFromBytes(upload.Data, upload.Filename)
		if err != nil {
			return nil, err
		}
		record.Set(upload.Name, file)
	}
	// Metadata is assigned by trusted callers, never copied from request bodies.
	record.Set("__miao_expected_updated", expected)
	record.Set("__miao_actor_id", actor)
	record.Set("__miao_source", source)
	record.Set("__miao_skip_events", source == "background")
	if err := form.Submit(); err != nil {
		return nil, persistenceError(err)
	}
	return recordMap(record), nil
}
func (c *Client) Create(ctx context.Context, collection string, body Record) (Record, error) {
	return c.save(ctx, collection, "", body, nil, "", "", "interactive")
}
func (c *Client) Update(ctx context.Context, collection, id string, body Record) (Record, error) {
	return c.save(ctx, collection, id, body, nil, "", "", "interactive")
}

// UpdateWhere performs a conditional internal update and returns whether one
// row matched. It is reserved for server-owned coordination records where a
// form submission would turn a compare-and-swap into a read-then-write race.
func (c *Client) UpdateWhere(ctx context.Context, collection string, body Record, where dbx.Expression) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	result, err := c.App.NonconcurrentDB().Update(collection, dbx.Params(body), where).WithContext(ctx).Execute()
	if err != nil {
		return false, persistenceError(err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
func (c *Client) Delete(ctx context.Context, collection, id string) error {
	record, err := c.nativeRecord(ctx, collection, id)
	if err != nil {
		return err
	}
	return persistenceError(c.App.DeleteWithContext(ctx, record))
}

// RevokeAuthTokens invalidates every outstanding PocketBase auth token for an
// account by rotating the auth record token key.
func (c *Client) RevokeAuthTokens(ctx context.Context, userID string) error {
	user, err := c.nativeRecord(ctx, "users", userID)
	if err != nil {
		return err
	}
	user.RefreshTokenKey()
	return persistenceError(c.App.SaveWithContext(ctx, user))
}

// DeleteExpiredWorkerLease only removes the observed lease if it is still
// expired. This prevents a stale worker from deleting a lease another worker
// renewed after the initial read.
func (c *Client) DeleteExpiredWorkerLease(ctx context.Context, id, now string) (bool, error) {
	result, err := c.App.ConcurrentDB().NewQuery("DELETE FROM miao_runtime_locks WHERE id = {:id} AND expires_at <= {:now}").Bind(dbx.Params{"id": id, "now": now}).WithContext(ctx).Execute()
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// Increment bumps a numeric counter field with a direct SQL update so frequent
// stat writes stay atomic and skip the record save pipeline, leaving the
// record's updated timestamp untouched.
func (c *Client) Increment(ctx context.Context, collection, id, field string) error {
	_, err := c.App.ConcurrentDB().NewQuery(fmt.Sprintf("UPDATE {{%s}} SET [[%s]] = COALESCE([[%s]], 0) + 1 WHERE [[id]] = {:id}", collection, field, field)).Bind(dbx.Params{"id": id}).WithContext(ctx).Execute()
	return err
}
func (c *Client) Collection(ctx context.Context, name string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	collection, err := c.App.FindCollectionByNameOrId(name)
	if err != nil {
		return nil, persistenceError(err)
	}
	return jsonMap(collection), nil
}
func (c *Client) saveCollection(ctx context.Context, name string, schema Record) (Record, error) {
	collection := core.NewBaseCollection("")
	if name != "" {
		var err error
		collection, err = c.App.FindCollectionByNameOrId(name)
		if err != nil {
			return nil, persistenceError(err)
		}
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, collection); err != nil {
		return nil, err
	}
	if err := c.App.SaveWithContext(ctx, collection); err != nil {
		return nil, persistenceError(err)
	}
	return jsonMap(collection), nil
}
func (c *Client) CreateCollection(ctx context.Context, schema Record) (Record, error) {
	return c.saveCollection(ctx, "", schema)
}
func (c *Client) UpdateCollection(ctx context.Context, name string, schema Record) (Record, error) {
	return c.saveCollection(ctx, name, schema)
}
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	collection, err := c.App.FindCollectionByNameOrId(name)
	if err != nil {
		return persistenceError(err)
	}
	return persistenceError(c.App.DeleteWithContext(ctx, collection))
}
func (c *Client) UpdateBusiness(ctx context.Context, collection, id string, body Record, expected, actor, source string) (Record, error) {
	return c.save(ctx, collection, id, body, nil, expected, actor, source)
}
func (c *Client) UploadBusiness(ctx context.Context, collection, id string, values Record, files []Upload, expected, actor, source string) (Record, error) {
	return c.save(ctx, collection, id, values, files, expected, actor, source)
}
func (c *Client) UploadNew(ctx context.Context, collection string, values Record, files []Upload) (Record, error) {
	return c.save(ctx, collection, "", values, files, "", "", "interactive")
}

// Called only after the shared business layer authorizes access to the record.
func (c *Client) ProtectedFile(ctx context.Context, collection, recordID, filename string) ([]byte, string, string, error) {
	record, err := c.nativeRecord(ctx, collection, recordID)
	if err != nil {
		return nil, "", "", err
	}
	if record.FindFileFieldByFile(filename) == nil {
		return nil, "", "", &Error{Status: 404, Message: "file not found"}
	}
	fsys, err := c.App.NewFilesystem()
	if err != nil {
		return nil, "", "", err
	}
	defer fsys.Close()
	fsys.SetContext(ctx)
	reader, err := fsys.GetReader(path.Join(record.BaseFilesPath(), filename))
	if err != nil {
		return nil, "", "", err
	}
	defer reader.Close()
	if reader.Size() > 6<<20 {
		return nil, "", "", errors.New("file exceeds download limit")
	}
	data, err := io.ReadAll(io.LimitReader(reader, (6<<20)+1))
	return data, reader.ContentType(), filename, err
}
func (c *Client) authenticate(ctx context.Context, endpoint string, body Record, token string) (Record, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://miao.internal/api/collections/users/"+endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	response := httptest.NewRecorder()
	c.auth.ServeHTTP(response, req)
	var result Record
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("invalid authentication response: %w", err)
	}
	if response.Code != http.StatusOK {
		return nil, &Error{Status: response.Code, Message: rowString(result, "message"), Data: result}
	}
	return result, nil
}
func (c *Client) AuthRefresh(ctx context.Context, token string) (Record, error) {
	return c.authenticate(ctx, "auth-refresh", Record{}, token)
}
func (c *Client) AuthPassword(ctx context.Context, email, password string) (Record, error) {
	return c.authenticate(ctx, "auth-with-password", Record{"identity": email, "password": password}, "")
}
func (c *Client) Health(ctx context.Context) error {
	var ok int
	return c.App.ConcurrentDB().NewQuery("SELECT 1").WithContext(ctx).Row(&ok)
}
func rowString(row Record, key string) string { value, _ := row[key].(string); return value }

// Transaction shares PocketBase's transaction app with every operation in fn.
func (c *Client) Transaction(ctx context.Context, fn func(*Client) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.App.RunInTransaction(func(tx core.App) error { return fn(&Client{App: tx, auth: c.auth}) })
}
