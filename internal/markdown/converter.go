// Package markdown converts Notion blocks into GitHub-flavored Markdown.
package markdown

import (
	"fmt"
	"strings"

	"github.com/alexismanuel/nogo/internal/notion"
)

// Options controls optional output features.
type Options struct {
	Frontmatter bool // prepend YAML frontmatter
}

// Convert renders a slice of Notion blocks as a Markdown string.
// indent is the current nesting level (0 = top-level).
func Convert(blocks []notion.Block, indent int) string {
	var sb strings.Builder
	numberedIdx := 0 // tracks position in a numbered list run

	for i, b := range blocks {
		// Detect list breaks so we can reset numbering.
		if b.Type != "numbered_list_item" {
			numberedIdx = 0
		}

		switch b.Type {
		case "paragraph":
			text := renderRichText(b.Paragraph.RichText)
			writeLine(&sb, indent, text)
			sb.WriteString("\n")

		case "heading_1":
			text := renderRichText(b.Heading1.RichText)
			writeLine(&sb, 0, "# "+text)
			sb.WriteString("\n")

		case "heading_2":
			text := renderRichText(b.Heading2.RichText)
			writeLine(&sb, 0, "## "+text)
			sb.WriteString("\n")

		case "heading_3":
			text := renderRichText(b.Heading3.RichText)
			writeLine(&sb, 0, "### "+text)
			sb.WriteString("\n")

		case "bulleted_list_item":
			text := renderRichText(b.BulletedListItem.RichText)
			writeLine(&sb, indent, "- "+text)
			if len(b.Children) > 0 {
				sb.WriteString(Convert(b.Children, indent+1))
			}

		case "numbered_list_item":
			numberedIdx++
			text := renderRichText(b.NumberedListItem.RichText)
			writeLine(&sb, indent, fmt.Sprintf("%d. %s", numberedIdx, text))
			if len(b.Children) > 0 {
				sb.WriteString(Convert(b.Children, indent+1))
			}

		case "to_do":
			text := renderRichText(b.ToDo.RichText)
			check := " "
			if b.ToDo.Checked {
				check = "x"
			}
			writeLine(&sb, indent, fmt.Sprintf("- [%s] %s", check, text))
			if len(b.Children) > 0 {
				sb.WriteString(Convert(b.Children, indent+1))
			}

		case "toggle":
			text := renderRichText(b.Toggle.RichText)
			writeLine(&sb, indent, "> **"+text+"**")
			if len(b.Children) > 0 {
				sb.WriteString(Convert(b.Children, indent+1))
			}
			sb.WriteString("\n")

		case "quote":
			text := renderRichText(b.Quote.RichText)
			for _, line := range strings.Split(text, "\n") {
				writeLine(&sb, indent, "> "+line)
			}
			sb.WriteString("\n")

		case "code":
			lang := b.Code.Language
			if lang == "plain text" {
				lang = ""
			}
			code := renderRichText(b.Code.RichText)
			writeLine(&sb, indent, "```"+lang)
			// Write code lines with no extra indent (code blocks are literal).
			for _, line := range strings.Split(code, "\n") {
				sb.WriteString(strings.Repeat("  ", indent) + line + "\n")
			}
			writeLine(&sb, indent, "```")
			if cap := renderRichText(b.Code.Caption); cap != "" {
				writeLine(&sb, indent, "*"+cap+"*")
			}
			sb.WriteString("\n")

		case "callout":
			icon := ""
			if b.Callout.Icon != nil && b.Callout.Icon.Emoji != "" {
				icon = b.Callout.Icon.Emoji + " "
			}
			text := renderRichText(b.Callout.RichText)
			writeLine(&sb, indent, "> "+icon+text)
			if len(b.Children) > 0 {
				sb.WriteString(Convert(b.Children, indent+1))
			}
			sb.WriteString("\n")

		case "divider":
			sb.WriteString("\n---\n\n")

		case "image":
			imgURL := b.Image.URL()
			alt := renderRichText(b.Image.Caption)
			if alt == "" {
				alt = "image"
			}
			writeLine(&sb, indent, fmt.Sprintf("![%s](%s)", alt, imgURL))
			sb.WriteString("\n")

		case "video":
			videoURL := b.Video.URL()
			cap := renderRichText(b.Video.Caption)
			if cap == "" {
				cap = videoURL
			}
			writeLine(&sb, indent, fmt.Sprintf("[%s](%s)", cap, videoURL))
			sb.WriteString("\n")

		case "bookmark":
			cap := renderRichText(b.Bookmark.Caption)
			if cap == "" {
				cap = b.Bookmark.URL
			}
			writeLine(&sb, indent, fmt.Sprintf("[%s](%s)", cap, b.Bookmark.URL))
			sb.WriteString("\n")

		case "link_preview":
			writeLine(&sb, indent, fmt.Sprintf("[%s](%s)", b.LinkPreview.URL, b.LinkPreview.URL))
			sb.WriteString("\n")

		case "embed":
			cap := renderRichText(b.Embed.Caption)
			if cap == "" {
				cap = b.Embed.URL
			}
			writeLine(&sb, indent, fmt.Sprintf("[%s](%s)", cap, b.Embed.URL))
			sb.WriteString("\n")

		case "equation":
			writeLine(&sb, indent, "$$"+b.Equation.Expression+"$$")
			sb.WriteString("\n")

		case "child_page":
			// Render as a link stub — fetching is done with `nogo get`.
			writeLine(&sb, indent, fmt.Sprintf("📄 **%s** *(child page)*", b.ChildPage.Title))
			sb.WriteString("\n")

		case "child_database":
			writeLine(&sb, indent, fmt.Sprintf("🗄 **%s** *(child database)*", b.ChildDatabase.Title))
			sb.WriteString("\n")

		case "table":
			// Children of a table block are table_row blocks.
			if i+1 < len(blocks) {
				// table rows are in b.Children
			}
			sb.WriteString(renderTable(b))
			sb.WriteString("\n")

		case "table_row":
			// Handled inside renderTable — skip standalone rows.

		default:
			// Unsupported block — emit a comment so content isn't silently lost.
			writeLine(&sb, indent, fmt.Sprintf("<!-- unsupported block type: %s -->", b.Type))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// DatabaseTable converts a Notion database (schema + rows) to a Markdown table.
// It extracts column names from the database properties and cell values from each row's properties.
func DatabaseTable(db *notion.Database, rows []notion.Page) string {
	if len(rows) == 0 {
		return "*(no rows)*\n"
	}

	// Build ordered column list from database properties.
	columns := dbColumns(db.Properties)
	if len(columns) == 0 {
		// Fallback: collect all property keys across all rows.
		seen := map[string]bool{}
		for _, row := range rows {
			for k := range row.Properties {
				if !seen[k] {
					seen[k] = true
					columns = append(columns, k)
				}
			}
		}
	}

	var sb strings.Builder

	// Header row.
	sb.WriteString("| ")
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(col)
	}
	sb.WriteString(" |\n")

	// Separator.
	sb.WriteString("| ")
	for i := range columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString("---")
	}
	sb.WriteString(" |\n")

	// Data rows.
	for _, row := range rows {
		sb.WriteString("| ")
		for i, col := range columns {
			if i > 0 {
			sb.WriteString(" | ")
			}
			sb.WriteString(cellValue(row.Properties[col]))
		}
		sb.WriteString(" |\n")
	}

	return sb.String()
}

// dbColumns extracts ordered column names from database properties.
func dbColumns(props map[string]interface{}) []string {
	// Notion returns properties as a map; order isn't guaranteed in Go,
	// but the JSON input is ordered. We rely on json.Decoder preserving
	// insertion order in Go maps — which it does for string keys.
	var cols []string
	for k, v := range props {
		// Skip computed/internal properties.
		if m, ok := v.(map[string]interface{}); ok {
			if t, ok := m["type"].(string); ok && t == "created_time" || t == "last_edited_time" || t == "created_by" || t == "last_edited_by" || t == "rollup" || t == "formula" {
				continue
			}
		}
		cols = append(cols, k)
	}
	return cols
}

// cellValue extracts a plain-text representation from a property value.
func cellValue(prop interface{}) string {
	if prop == nil {
		return ""
	}
	m, ok := prop.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", prop)
	}

	propType, _ := m["type"].(string)

	switch propType {
	case "title":
		return richTextSlice(m["title"])
	case "rich_text":
		return richTextSlice(m["rich_text"])
	case "select":
		return selectName(m["select"])
	case "multi_select":
		return multiSelectNames(m["multi_select"])
	case "number":
		return numberValue(m["number"])
	case "checkbox":
		return checkboxValue(m["checkbox"])
	case "date":
		return dateValue(m["date"])
	case "url":
		return stringValue(m["url"])
	case "email":
		return stringValue(m["email"])
	case "phone_number":
		return stringValue(m["phone_number"])
	case "status":
		return selectName(m["status"])
	case "people":
		return peopleNames(m["people"])
	case "relation":
		return relationIDs(m["relation"])
	case "files":
		return filesList(m["files"])
	default:
		// For unknown types, try rich_text or plain array.
		if rt := richTextSlice(m[propType]); rt != "" {
			return rt
		}
		return ""
	}
}

func richTextSlice(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, item := range arr {
		if obj, ok := item.(map[string]interface{}); ok {
			if pt, ok := obj["plain_text"].(string); ok {
				sb.WriteString(pt)
			}
		}
	}
	return sb.String()
}

func selectName(v interface{}) string {
	if m, ok := v.(map[string]interface{}); ok {
		if name, ok := m["name"].(string); ok {
			return name
		}
	}
	return ""
}

func multiSelectNames(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var names []string
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return strings.Join(names, ", ")
}

func numberValue(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func checkboxValue(v interface{}) string {
	if b, ok := v.(bool); ok {
		if b {
			return "✅"
		}
		return "☐"
	}
	return ""
}

func dateValue(v interface{}) string {
	if m, ok := v.(map[string]interface{}); ok {
		start, _ := m["start"].(string)
		return start
	}
	return ""
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func peopleNames(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var names []string
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return strings.Join(names, ", ")
}

func relationIDs(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var ids []string
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	return strings.Join(ids, ", ")
}

func filesList(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var urls []string
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			if f, ok := m["file"].(map[string]interface{}); ok {
				if u, ok := f["url"].(string); ok {
					urls = append(urls, u)
				}
			}
			if e, ok := m["external"].(map[string]interface{}); ok {
				if u, ok := e["url"].(string); ok {
					urls = append(urls, u)
				}
			}
		}
	}
	return strings.Join(urls, ", ")
}

// Frontmatter builds a YAML frontmatter block from page metadata.
func Frontmatter(title, pageID, pageURL, lastEdited string) string {
	return fmt.Sprintf("---\ntitle: %q\nnotion_id: %q\nnotion_url: %q\nlast_edited: %q\n---\n\n",
		title, pageID, pageURL, lastEdited)
}

// renderTable converts a table block (with children as rows) to a Markdown table.
func renderTable(b notion.Block) string {
	if len(b.Children) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, row := range b.Children {
		if row.TableRow == nil {
			continue
		}
		cells := make([]string, len(row.TableRow.Cells))
		for j, cell := range row.TableRow.Cells {
			cells[j] = renderRichText(cell)
		}
		sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		// After the first (header) row, emit the separator.
		if i == 0 {
			sep := make([]string, len(cells))
			for k := range sep {
				sep[k] = "---"
			}
			sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")
		}
	}
	return sb.String()
}

// renderRichText converts a slice of RichText segments into a Markdown string.
func renderRichText(rts []notion.RichText) string {
	var sb strings.Builder
	for _, rt := range rts {
		text := rt.PlainText
		if text == "" {
			continue
		}

		a := rt.Annotations

		// Apply inline code first (can't nest inside code).
		if a.Code {
			sb.WriteString("`" + text + "`")
			continue
		}

		// Apply link.
		if rt.Href != "" {
			text = "[" + text + "](" + rt.Href + ")"
		}

		// Apply formatting (order matters for readability).
		if a.Bold {
			text = "**" + text + "**"
		}
		if a.Italic {
			text = "*" + text + "*"
		}
		if a.Strikethrough {
			text = "~~" + text + "~~"
		}

		sb.WriteString(text)
	}
	return sb.String()
}

// writeLine writes a line with the appropriate indentation prefix.
func writeLine(sb *strings.Builder, indent int, line string) {
	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString(line)
	sb.WriteString("\n")
}
