// Package notiondesktop reads from Notion's local desktop SQLite cache.
//
// It snapshots the Notion app's notion.db (to avoid locking), then translates
// the internal block/collection schema into the same notion.Block types the API
// client produces — so the existing markdown converter works unchanged.
package notiondesktop

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexismanuel/nogo/internal/notion"
	_ "modernc.org/sqlite"
)

// PageEntry is a lightweight row returned by ListPages.
type PageEntry struct {
	Type       string // "page" or "database"
	ID         string
	Title      string
	LastEdited string // RFC3339 or empty
}

// Reader opens a read-only snapshot of Notion's desktop cache.
type Reader struct {
	db       *sql.DB
	snapshot string
	dbPath   string // original notion.db path (for resolving blob_storage)
}

// DefaultPath returns the default macOS path to Notion's desktop cache.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Notion", "notion.db")
}

// Available returns true if the local Notion desktop cache exists and is readable.
func Available() bool {
	_, err := os.Stat(DefaultPath())
	return err == nil
}

// SyncInfo holds metadata about how fresh the local cache is.
type SyncInfo struct {
	DBModified   string // mtime of notion.db (wall-clock last write)
	LastAutoSync string // last automatic sync timestamp
	LastRefetch  string // last full refetch timestamp
}

// LastSynced returns information about how recently the local cache was synced.
// It combines the DB file mtime with explicit sync timestamps from the cache.
func LastSynced() (*SyncInfo, error) {
	path := DefaultPath()
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("notion desktop cache not found: %w", err)
	}

	info := &SyncInfo{}

	// DB file modification time.
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	info.DBModified = st.ModTime().UTC().Format(time.RFC3339)

	// Try to read sync timestamps directly from the live database.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return info, nil // Return what we have; mtime is still useful.
	}
	defer db.Close()

	var autoSync, refetch int64
	_ = db.QueryRow(
		`SELECT last_executed_at FROM offline_download_metadata WHERE task_type = 'autosync' ORDER BY last_executed_at DESC LIMIT 1`,
	).Scan(&autoSync)
	_ = db.QueryRow(
		`SELECT last_executed_at FROM offline_download_metadata WHERE task_type = 'refetch' ORDER BY last_executed_at DESC LIMIT 1`,
	).Scan(&refetch)
	info.LastAutoSync = epochMsToRFC3339(autoSync)
	info.LastRefetch = epochMsToRFC3339(refetch)

	return info, nil
}

func epochMsToRFC3339(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.Unix(ms/1000, 0).UTC().Format(time.RFC3339)
}

// Open creates a snapshot of the Notion desktop cache and opens it read-only.
func Open(path string) (*Reader, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("notion desktop cache not found at %s: %w", path, err)
	}
	snapshot, err := snapshotDB(path)
	if err != nil {
		return nil, fmt.Errorf("snapshotting notion.db: %w", err)
	}
	db, err := sql.Open("sqlite", snapshot)
	if err != nil {
		os.Remove(snapshot)
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		os.Remove(snapshot)
		return nil, err
	}
	return &Reader{db: db, snapshot: snapshot, dbPath: path}, nil
}

// Close closes the database and removes the snapshot.
func (r *Reader) Close() error {
	if r.db != nil {
		r.db.Close()
	}
	if r.snapshot != "" {
		os.Remove(r.snapshot)
		os.Remove(r.snapshot + "-wal")
		os.Remove(r.snapshot + "-shm")
	}
	return nil
}

// ListPages returns all pages (and databases) from the local cache.
// When all is false, only top-level items (parent_table='space') are returned.
func (r *Reader) ListPages(all bool) ([]PageEntry, error) {
	pageQuery := `
		SELECT id, coalesce(properties, '{}'), max(coalesce(last_edited_time, 0))
		FROM block
		WHERE type = 'page' AND alive = 1
		AND parent_table NOT IN ('collection')
		AND json_extract(format, '$.copied_from_pointer') IS NULL`
	if !all {
		pageQuery += ` AND parent_table = 'space'`
	}
	pageQuery += ` GROUP BY id`

	pageRows, err := r.db.Query(pageQuery)
	if err != nil {
		return nil, fmt.Errorf("listing pages: %w", err)
	}
	defer pageRows.Close()

	// Collect pages and database blocks separately to avoid
	// nested queries while holding the single connection.
	var entries []PageEntry
	var dbBlockIDs []string
	for pageRows.Next() {
		var id, props string
		var let float64
		if err := pageRows.Scan(&id, &props, &let); err != nil {
			return nil, err
		}
		title := extractTitle(props)
		if title == "" {
			title = "Untitled"
		}
		entries = append(entries, PageEntry{Type: "page", ID: id, Title: title, LastEdited: formatTime(let)})
	}
	if err := pageRows.Err(); err != nil {
		return nil, err
	}

	// Fetch database block IDs and their collection titles.
	dbQuery := `
		SELECT b.id, coalesce(json_extract(b.format, '$.collection_pointer.id'), ''), max(coalesce(b.last_edited_time, 0))
		FROM block b
		WHERE b.type = 'collection_view_page' AND b.alive = 1`
	if !all {
		dbQuery += ` AND b.parent_table = 'space'`
	}
	dbQuery += ` GROUP BY b.id`

	dbRows, err := r.db.Query(dbQuery)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}
	defer dbRows.Close()

	// Map collection ID -> name for batch lookup.
	collectionIDs := map[string]string{}
	collLet := map[string]float64{}
	for dbRows.Next() {
		var blockID, collID string
		var let float64
		if err := dbRows.Scan(&blockID, &collID, &let); err != nil {
			return nil, err
		}
		if collID != "" {
			collectionIDs[collID] = blockID
		collLet[collID] = let
		dbBlockIDs = append(dbBlockIDs, blockID)
		}
	}
	if err := dbRows.Err(); err != nil {
		return nil, err
	}

	// Resolve collection names in a single pass.
	for collID, blockID := range collectionIDs {
		var name string
		r.db.QueryRow(`SELECT coalesce(name, '') FROM collection WHERE id = ?`, collID).Scan(&name)
		title := extractTitle(name)
		if title == "" {
			title = "Untitled"
		}
		entries = append(entries, PageEntry{Type: "database", ID: blockID, Title: title, LastEdited: formatTime(collLet[collID])})
	}

	return entries, nil
}

// BlockMeta holds lightweight metadata for a cached block.
type BlockMeta struct {
	ID          string
	LastEdited  string
	Created     string
}

// GetBlockMeta returns last-edited and created timestamps for a block.
func (r *Reader) GetBlockMeta(blockID string) (*BlockMeta, error) {
	cid := cacheID(blockID)
	var id string
	var let, ct float64
	err := r.db.QueryRow(
		`SELECT id, coalesce(last_edited_time, 0), coalesce(created_time, 0) FROM block WHERE id = ? AND alive = 1`, cid,
	).Scan(&id, &let, &ct)
	if err != nil {
		return nil, fmt.Errorf("block not found in local cache: %w", err)
	}
	return &BlockMeta{
		ID:         id,
		LastEdited: formatTime(let),
		Created:    formatTime(ct),
	}, nil
}

// ResolvePageID parses a URL or raw ID into a normalized page ID.
// This is a convenience function that doesn't require opening a Reader.
func ResolvePageID(input string) (string, error) {
	return notion.ParsePageID(input)
}

// LastEditedTime returns the last_edited_time epoch-ms for a block by ID.
// Returns 0 if not found.
func (r *Reader) LastEditedTime(blockID string) float64 {
	var ts float64
	r.db.QueryRow(
		`SELECT coalesce(last_edited_time, 0) FROM block WHERE id = ? AND alive = 1`, cacheID(blockID),
	).Scan(&ts)
	return ts
}

// LastEditedTimeByID resolves a URL/ID string and returns the last_edited_time.
// Returns 0 if not found.
func (r *Reader) LastEditedTimeByID(input string) float64 {
	id, err := notion.ParsePageID(input)
	if err != nil {
		return 0
	}
	return r.LastEditedTime(id)
}

// GetPage fetches a page by ID from the local cache.
func (r *Reader) GetPage(pageID string) (*notion.Page, error) {
	cid := cacheID(pageID)
	var id, props string
	var createdTime, lastEditedTime float64
	err := r.db.QueryRow(
		`SELECT id, coalesce(properties, '{}'), coalesce(created_time, 0), coalesce(last_edited_time, 0)
		 FROM block WHERE id = ? AND alive = 1`, cid,
	).Scan(&id, &props, &createdTime, &lastEditedTime)
	if err != nil {
		return nil, fmt.Errorf("page not found in local cache: %w", err)
	}

	title := extractTitle(props)
	properties := map[string]interface{}{
		"title": map[string]interface{}{
			"type":  "title",
			"title": []interface{}{
				map[string]interface{}{
					"type":       "text",
					"plain_text": title,
					"text":       map[string]interface{}{"content": title},
				},
			},
		},
	}
	return &notion.Page{
		ID:             id,
		URL:            "",
		Properties:     properties,
		CreatedTime:    formatTime(createdTime),
		LastEditedTime: formatTime(lastEditedTime),
	}, nil
}

// GetPageTitle extracts the title from a page (API-compatible signature).
func (r *Reader) GetPageTitle(page *notion.Page) string {
	for _, v := range page.Properties {
		prop, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if prop["type"] != "title" {
			continue
		}
		titles, ok := prop["title"].([]interface{})
		if !ok {
			continue
		}
		var sb strings.Builder
		for _, t := range titles {
			if obj, ok := t.(map[string]interface{}); ok {
				if pt, ok := obj["plain_text"].(string); ok {
					sb.WriteString(pt)
				}
			}
		}
		title := strings.TrimSpace(sb.String())
		if title != "" {
			return title
		}
	}
	return "Untitled"
}

// GetBlocks fetches all blocks for a parent ID, building the tree recursively.
func (r *Reader) GetBlocks(parentID string) ([]notion.Block, error) {
	return r.getChildren(cacheID(parentID))
}

// GetDatabase fetches a collection/database by ID.
func (r *Reader) GetDatabase(dbID string) (*notion.Database, error) {
	collectionID := r.resolveCollectionID(cacheID(dbID))

	var name, schema string
	err := r.db.QueryRow(
		`SELECT coalesce(name, ''), coalesce(schema, '{}') FROM collection WHERE id = ? AND alive = 1`, collectionID,
	).Scan(&name, &schema)
	if err != nil {
		return nil, fmt.Errorf("collection not found in local cache: %w", err)
	}

	collTitle := extractTitle(name)
	dbProperties := buildDBProperties(schema)

	return &notion.Database{
		ID:         collectionID,
		URL:        "",
		Title:      []notion.RichText{{Type: "text", PlainText: collTitle, Text: &notion.TextObject{Content: collTitle}}},
		Properties: dbProperties,
	}, nil
}

// GetDatabaseTitle returns the database title (API-compatible signature).
func (r *Reader) GetDatabaseTitle(db *notion.Database) string {
	var sb strings.Builder
	for _, rt := range db.Title {
		sb.WriteString(rt.PlainText)
	}
	title := strings.TrimSpace(sb.String())
	if title == "" {
		return "Untitled"
	}
	return title
}

// QueryDatabase fetches all rows for a collection, mapped to API-like properties.
func (r *Reader) QueryDatabase(dbID string) ([]notion.Page, error) {
	collectionID := r.resolveCollectionID(cacheID(dbID))

	var schemaJSON string
	r.db.QueryRow(
		`SELECT coalesce(schema, '{}') FROM collection WHERE id = ?`, collectionID,
	).Scan(&schemaJSON)
	schemaMap := parseSchema(schemaJSON)

	// Row blocks have parent_table='collection' and parent_id pointing to the collection.
	rows, err := r.db.Query(
		`SELECT id, coalesce(properties, '{}'), coalesce(created_time, 0), coalesce(last_edited_time, 0)
		 FROM block WHERE parent_table = 'collection' AND parent_id = ? AND alive = 1
		 ORDER BY created_time`, collectionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []notion.Page
	for rows.Next() {
		var id, props string
		var ct, let float64
		if err := rows.Scan(&id, &props, &ct, &let); err != nil {
			return nil, err
		}
		pageProps := mapPageProperties(props, schemaMap)
		pages = append(pages, notion.Page{
			ID:             id,
			Properties:     pageProps,
			CreatedTime:    formatTime(ct),
			LastEditedTime: formatTime(let),
		})
	}
	return pages, rows.Err()
}

// --- block tree ---

func (r *Reader) getChildren(parentID string) ([]notion.Block, error) {
	childIDs := r.getContentIDs(parentID)
	var blocks []notion.Block
	for _, cid := range childIDs {
		b, err := r.buildBlock(cid)
		if err != nil {
			return nil, err
		}
		if b != nil {
			blocks = append(blocks, *b)
		}
	}
	return blocks, nil
}

func (r *Reader) getContentIDs(blockID string) []string {
	var content string
	err := r.db.QueryRow(
		`SELECT coalesce(content, '[]') FROM block WHERE id = ?`, blockID,
	).Scan(&content)
	if err != nil {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(content), &ids); err != nil {
		return nil
	}
	return ids
}

func (r *Reader) buildBlock(id string) (*notion.Block, error) {
	var typ, props, format string
	var alive int
	err := r.db.QueryRow(
		`SELECT type, coalesce(properties, '{}'), coalesce(format, '{}'), alive
		 FROM block WHERE id = ?`, id,
	).Scan(&typ, &props, &format, &alive)
	if err != nil || alive == 0 {
		return nil, nil
	}

	properties := parseProperties(props)
	formatMap := parseFormat(format)
	blockType := mapBlockType(typ)

	// Don't recurse into child pages / databases — render as stubs.
	var children []notion.Block
	var hasChildren bool
	if blockType != "child_page" && blockType != "child_database" {
		childIDs := r.getContentIDs(id)
		hasChildren = len(childIDs) > 0
		for _, cid := range childIDs {
			child, err := r.buildBlock(cid)
			if err != nil {
				return nil, err
			}
			if child != nil {
				children = append(children, *child)
			}
		}
	}

	block := &notion.Block{
		ID:          id,
		Type:        blockType,
		HasChildren: hasChildren,
		Children:    children,
	}
	r.populateBlock(block, typ, properties, formatMap)
	return block, nil
}

func (r *Reader) populateBlock(block *notion.Block, rawType string, props map[string]interface{}, format map[string]interface{}) {
	switch rawType {
	case "text":
		block.Paragraph = &notion.ParagraphBlock{RichText: propRichText(props, "title")}

	case "header":
		rt := propRichText(props, "title")
		block.Heading1 = &notion.HeadingBlock{RichText: rt}
		block.Type = "heading_1"

	case "sub_header":
		rt := propRichText(props, "title")
		block.Heading2 = &notion.HeadingBlock{RichText: rt}
		block.Type = "heading_2"

	case "sub_sub_header":
		rt := propRichText(props, "title")
		block.Heading3 = &notion.HeadingBlock{RichText: rt}
		block.Type = "heading_3"

	case "bulleted_list":
		rt := propRichText(props, "title")
		block.BulletedListItem = &notion.ListItemBlock{RichText: rt}
		block.Type = "bulleted_list_item"

	case "numbered_list":
		rt := propRichText(props, "title")
		block.NumberedListItem = &notion.ListItemBlock{RichText: rt}
		block.Type = "numbered_list_item"

	case "to_do":
		rt := propRichText(props, "title")
		checked := formatChecked(format)
		block.ToDo = &notion.ToDoBlock{RichText: rt, Checked: checked}

	case "toggle":
		rt := propRichText(props, "title")
		block.Toggle = &notion.ToggleBlock{RichText: rt}

	case "quote":
		rt := propRichText(props, "title")
		block.Quote = &notion.QuoteBlock{RichText: rt}

	case "code":
		rt := propRichText(props, "title")
		lang := propPlainText(props, "language")
		if lang == "plain text" {
			lang = ""
		}
		block.Code = &notion.CodeBlock{RichText: rt, Language: lang}

	case "callout":
		rt := propRichText(props, "title")
		block.Callout = &notion.CalloutBlock{
			RichText: rt,
			Icon:     formatIcon(format),
		}

	case "divider":
		block.Divider = &struct{}{}

	case "image":
		source := propPlainText(props, "source")
		caption := propRichText(props, "caption")
		img := &notion.ImageBlock{Caption: caption}
		if strings.HasPrefix(source, "http") {
			img.Type = "external"
			img.External = &notion.ExternalObject{URL: source}
		} else {
			img.Type = "file"
			img.File = &notion.FileObject{URL: r.resolveAttachment(source)}
		}
		block.Image = img

	case "bookmark":
		link := propPlainText(props, "link")
		caption := propRichText(props, "title")
		block.Bookmark = &notion.BookmarkBlock{URL: link, Caption: caption}

	case "video":
		source := propPlainText(props, "source")
		caption := propRichText(props, "caption")
		vid := &notion.VideoBlock{Caption: caption}
		if strings.HasPrefix(source, "http") {
			vid.Type = "external"
			vid.External = &notion.ExternalObject{URL: source}
		} else {
			vid.Type = "file"
			vid.File = &notion.FileObject{URL: r.resolveAttachment(source)}
		}
		block.Video = vid

	case "embed":
		block.Embed = &notion.EmbedBlock{URL: formatURL(format)}

	case "external_object_instance":
		block.Embed = &notion.EmbedBlock{URL: formatURL(format)}
		block.Type = "embed"

	case "equation":
		block.Equation = &notion.EquationBlock{Expression: propPlainText(props, "title")}

	case "page":
		block.ChildPage = &notion.ChildPageBlock{Title: propPlainText(props, "title")}
		block.Type = "child_page"

	case "collection_view_page":
		title := propPlainText(props, "title")
		if title == "" {
			title = r.resolveCollectionTitle(format)
		}
		block.ChildDatabase = &notion.ChildDatabaseBlock{Title: title}
		block.Type = "child_database"

	case "table":
		block.Table = &notion.TableBlock{TableWidth: len(block.Children)}

	case "table_row":
		block.TableRow = &notion.TableRowBlock{Cells: tableRowCells(props)}
	}
}

// --- collection / database helpers ---

func (r *Reader) resolveCollectionID(blockID string) string {
	var format string
	err := r.db.QueryRow(
		`SELECT coalesce(format, '{}') FROM block WHERE id = ? AND alive = 1`, blockID,
	).Scan(&format)
	if err != nil {
		return blockID
	}
	var f map[string]interface{}
	json.Unmarshal([]byte(format), &f)
	if cp, ok := f["collection_pointer"].(map[string]interface{}); ok {
		if cid, ok := cp["id"].(string); ok {
			return cid
		}
	}
	return blockID
}

func (r *Reader) resolveCollectionTitle(formatMap map[string]interface{}) string {
	cp, ok := formatMap["collection_pointer"].(map[string]interface{})
	if !ok {
		return ""
	}
	cid, ok := cp["id"].(string)
	if !ok {
		return ""
	}
	var name string
	r.db.QueryRow(`SELECT coalesce(name, '') FROM collection WHERE id = ?`, cid).Scan(&name)
	return extractTitle(name)
}

func (r *Reader) resolveAttachment(source string) string {
	if !strings.HasPrefix(source, "attachment:") {
		return source
	}
	parts := strings.SplitN(source, ":", 3)
	if len(parts) >= 2 {
		dir := filepath.Dir(r.dbPath)
		return filepath.Join(dir, "blob_storage", parts[1])
	}
	return source
}

// --- rich text parsing (Notion internal cache format) ---
//
// The cache stores text as arrays of segments:
//
//	[["plain text"], ["bold text", [["b"]]], ["linked", [["a","https://..."]]]]
//
// Each segment is [text, annotations_array?] where annotations are
// ["b"]=bold, ["i"]=italic, ["c"]=code, ["s"]=strikethrough, ["a","url"]=link.

func propRichText(props map[string]interface{}, key string) []notion.RichText {
	raw, ok := props[key]
	if !ok {
		return nil
	}
	segments, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var result []notion.RichText
	for _, seg := range segments {
		rt := parseSegment(seg)
		if rt != nil {
			result = append(result, *rt)
		}
	}
	return result
}

func propPlainText(props map[string]interface{}, key string) string {
	raw, ok := props[key]
	if !ok {
		return ""
	}
	segments, ok := raw.([]interface{})
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, seg := range segments {
		switch s := seg.(type) {
		case string:
			sb.WriteString(s)
		case []interface{}:
			if len(s) > 0 {
				if text, ok := s[0].(string); ok && text != "‣" {
					sb.WriteString(text)
				}
			}
		}
	}
	return sb.String()
}

func parseSegment(seg interface{}) *notion.RichText {
	switch s := seg.(type) {
	case string:
		return &notion.RichText{
			Type:      "text",
			PlainText: s,
			Text:      &notion.TextObject{Content: s},
		}
	case []interface{}:
		if len(s) == 0 {
			return nil
		}
		text, ok := s[0].(string)
		if !ok {
			return nil
		}
		// Handle ‣ mentions: ["‣", [["p", "id", "spaceId"]]]
		if text == "‣" && len(s) >= 2 {
			return parseMention(s[1])
		}
		rt := &notion.RichText{
			Type:      "text",
			PlainText: text,
			Text:      &notion.TextObject{Content: text},
		}
		if len(s) >= 2 {
			if annList, ok := s[1].([]interface{}); ok {
				for _, ann := range annList {
					if arr, ok := ann.([]interface{}); ok && len(arr) > 0 {
						code, _ := arr[0].(string)
						switch code {
						case "b":
							rt.Annotations.Bold = true
						case "i":
							rt.Annotations.Italic = true
						case "c":
							rt.Annotations.Code = true
						case "s":
							rt.Annotations.Strikethrough = true
						case "u":
							rt.Annotations.Underline = true
						case "a":
							if len(arr) >= 2 {
								url, _ := arr[1].(string)
								rt.Href = url
								rt.Text.Link = &notion.Link{URL: url}
							}
						}
					}
				}
			}
		}
		return rt
	default:
		return nil
	}
}

func parseMention(data interface{}) *notion.RichText {
	arr, ok := data.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	mentionArr, ok := arr[0].([]interface{})
	if !ok || len(mentionArr) == 0 {
		return nil
	}
	kind, _ := mentionArr[0].(string)
	var label string
	switch kind {
	case "p": // page mention
		if len(mentionArr) >= 2 {
			label, _ = mentionArr[1].(string)
		}
	case "u": // user mention
		if len(mentionArr) >= 2 {
			label, _ = mentionArr[1].(string)
		}
	case "d": // date
		if len(mentionArr) >= 2 {
			if obj, ok := mentionArr[1].(map[string]interface{}); ok {
				if start, ok := obj["start_date"].(string); ok {
					label = start
				}
			}
		}
	case "m": // equation
		if len(mentionArr) >= 2 {
			label, _ = mentionArr[1].(string)
		}
	}
	if label == "" {
		label = kind
	}
	return &notion.RichText{
		Type:      "mention",
		PlainText: label,
		Text:      &notion.TextObject{Content: label},
	}
}

// --- table row helpers ---

func tableRowCells(props map[string]interface{}) [][]notion.RichText {
	// Preserve insertion order — Go maps don't guarantee it but
	// json.Unmarshal for map[string]interface{} keeps key order in practice.
	var cells [][]notion.RichText
	for _, v := range props {
		segments, ok := v.([]interface{})
		if !ok {
			continue
		}
		var rt []notion.RichText
		for _, seg := range segments {
			parsed := parseSegment(seg)
			if parsed != nil {
				rt = append(rt, *parsed)
			}
		}
		cells = append(cells, rt)
	}
	return cells
}

// --- database property mapping ---

// schemaEntry is a parsed collection schema property.
type schemaEntry struct {
	Name string
	Type string
}

func parseSchema(raw string) map[string]schemaEntry {
	var m map[string]interface{}
	json.Unmarshal([]byte(raw), &m)
	result := map[string]schemaEntry{}
	for k, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := obj["name"].(string)
		typ, _ := obj["type"].(string)
		result[k] = schemaEntry{Name: name, Type: typ}
	}
	return result
}

func buildDBProperties(schemaJSON string) map[string]interface{} {
	schema := parseSchema(schemaJSON)
	props := map[string]interface{}{}
	for shortID, entry := range schema {
		props[entry.Name] = map[string]interface{}{
			"type": entry.Type,
			"name": entry.Name,
			"id":   shortID,
		}
	}
	return props
}

func mapPageProperties(raw string, schema map[string]schemaEntry) map[string]interface{} {
	props := parseProperties(raw)
	result := map[string]interface{}{}
	for shortID, entry := range schema {
		value := props[shortID]
		propType := entry.Type
		result[entry.Name] = buildPropertyValue(value, propType)
	}
	return result
}

func buildPropertyValue(raw interface{}, propType string) map[string]interface{} {
	m := map[string]interface{}{"type": propType}

	plainText := segmentsToPlainText(raw)

	switch propType {
	case "title":
		m["title"] = []interface{}{
			map[string]interface{}{
				"type":       "text",
				"plain_text": plainText,
				"text":       map[string]interface{}{"content": plainText},
			},
		}
	case "rich_text", "text":
		m["rich_text"] = []interface{}{
			map[string]interface{}{
				"type":       "text",
				"plain_text": plainText,
				"text":       map[string]interface{}{"content": plainText},
			},
		}
	case "select":
		m["select"] = map[string]interface{}{"name": plainText}
	case "multi_select":
		items := strings.Split(plainText, ",")
		var opts []interface{}
		for _, item := range items {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				opts = append(opts, map[string]interface{}{"name": trimmed})
			}
		}
		m["multi_select"] = opts
	case "number":
		m["number"] = plainText
	case "checkbox":
		m["checkbox"] = plainText == "true" || plainText == "Yes"
	case "date":
		m["date"] = map[string]interface{}{"start": plainText}
	case "url":
		m["url"] = plainText
	case "email":
		m["email"] = plainText
	case "phone_number":
		m["phone_number"] = plainText
	case "status":
		m["status"] = map[string]interface{}{"name": plainText}
	case "people":
		m["people"] = []interface{}{}
	case "relation":
		m["relation"] = []interface{}{}
	case "files":
		m["files"] = []interface{}{}
	default:
		m[propType] = plainText
	}
	return m
}

func segmentsToPlainText(raw interface{}) string {
	segments, ok := raw.([]interface{})
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, seg := range segments {
		switch s := seg.(type) {
		case string:
			sb.WriteString(s)
		case []interface{}:
			if len(s) > 0 {
				if text, ok := s[0].(string); ok {
					if text == "‣" {
						// Extract mention display text
						if len(s) >= 2 {
							if arr, ok := s[1].([]interface{}); ok && len(arr) > 0 {
								if inner, ok := arr[0].([]interface{}); ok && len(inner) >= 2 {
									switch kind, _ := inner[0].(string); kind {
									case "p", "u", "m":
										if id, ok := inner[1].(string); ok {
											sb.WriteString(id)
										}
									case "d":
										if len(inner) >= 2 {
											if obj, ok := inner[1].(map[string]interface{}); ok {
												if start, ok := obj["start_date"].(string); ok {
													sb.WriteString(start)
												}
											}
										}
									}
								}
							}
						}
					} else {
						sb.WriteString(text)
					}
				}
			}
		}
	}
	return sb.String()
}

// --- JSON parsing helpers ---

func parseProperties(raw string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func parseFormat(raw string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func extractTitle(propsJSON string) string {
	// First try as a property map (block properties).
	props := parseProperties(propsJSON)
	for _, key := range []string{"title", "Name", "name"} {
		if val := propPlainText(props, key); val != "" {
			return val
		}
	}
	// Try as a raw segment array (collection name format: [["Title"]]).
	var segments []interface{}
	if err := json.Unmarshal([]byte(propsJSON), &segments); err == nil {
		return segmentsToPlainText(segments)
	}
	return ""
}

// --- format helpers ---

func formatIcon(format map[string]interface{}) *notion.Icon {
	icon, ok := format["page_icon"].(string)
	if !ok || icon == "" {
		return nil
	}
	return &notion.Icon{Type: "emoji", Emoji: icon}
}

func formatChecked(format map[string]interface{}) bool {
	checked, ok := format["checked"]
	if !ok {
		return false
	}
	switch v := checked.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "Yes" || v == "1"
	case float64:
		return v != 0
	}
	return false
}

func formatURL(format map[string]interface{}) string {
	if uri, ok := format["uri"].(string); ok {
		return uri
	}
	if original, ok := format["original_url"].(string); ok {
		return original
	}
	return ""
}

// --- ID helpers ---

// cacheID re-adds dashes to a stripped UUID for querying the cache.
func cacheID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) != 32 {
		return id
	}
	return clean[:8] + "-" + clean[8:12] + "-" + clean[12:16] + "-" + clean[16:20] + "-" + clean[20:]
}

func formatTime(epoch float64) string {
	if epoch <= 0 {
		return ""
	}
	return time.Unix(int64(epoch/1000), 0).UTC().Format(time.RFC3339)
}

// mapBlockType maps Notion's internal cache type names to API type names.
func mapBlockType(t string) string {
	switch t {
	case "text":
		return "paragraph"
	case "header":
		return "heading_1"
	case "sub_header":
		return "heading_2"
	case "sub_sub_header":
		return "heading_3"
	case "bulleted_list":
		return "bulleted_list_item"
	case "numbered_list":
		return "numbered_list_item"
	case "page":
		return "child_page"
	case "collection_view_page":
		return "child_database"
	case "external_object_instance":
		return "embed"
	default:
		return t
	}
}

// --- snapshot ---

func snapshotDB(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	dst := filepath.Join(os.TempDir(), fmt.Sprintf("nogo-notion-%d.db", time.Now().UnixMilli()))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return "", err
	}
	out.Close()

	for _, suffix := range []string{"-wal", "-shm"} {
		s := src + suffix
		if _, err := os.Stat(s); err == nil {
			copyFile(s, dst+suffix)
		}
	}
	return dst, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
