package notion

// RichText represents a segment of rich text in Notion.
type RichText struct {
	Type        string      `json:"type"`
	Text        *TextObject `json:"text,omitempty"`
	Annotations Annotations `json:"annotations"`
	PlainText   string      `json:"plain_text"`
	Href        string      `json:"href,omitempty"`
}

// TextObject is the inner text/link payload for RichText.
type TextObject struct {
	Content string `json:"content"`
	Link    *Link  `json:"link,omitempty"`
}

// Link holds a URL for inline links.
type Link struct {
	URL string `json:"url"`
}

// Annotations holds text formatting flags.
type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// File represents a Notion file/external URL (used in images, attachments).
type File struct {
	Type     string          `json:"type"`
	File     *FileObject     `json:"file,omitempty"`
	External *ExternalObject `json:"external,omitempty"`
}

type FileObject struct {
	URL string `json:"url"`
}

type ExternalObject struct {
	URL string `json:"url"`
}

func (f *File) URL() string {
	if f == nil {
		return ""
	}
	if f.File != nil {
		return f.File.URL
	}
	if f.External != nil {
		return f.External.URL
	}
	return ""
}

// Page represents a Notion page object.
type Page struct {
	ID             string                 `json:"id"`
	URL            string                 `json:"url"`
	Properties     map[string]interface{} `json:"properties"`
	LastEditedTime string                 `json:"last_edited_time"`
	CreatedTime    string                 `json:"created_time"`
}

// Block is a single Notion block with a dynamic content payload.
type Block struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"`
	HasChildren    bool        `json:"has_children"`
	LastEditedTime string      `json:"last_edited_time"`

	// Common block types — only the relevant one will be non-nil.
	Paragraph        *ParagraphBlock        `json:"paragraph,omitempty"`
	Heading1         *HeadingBlock          `json:"heading_1,omitempty"`
	Heading2         *HeadingBlock          `json:"heading_2,omitempty"`
	Heading3         *HeadingBlock          `json:"heading_3,omitempty"`
	BulletedListItem *ListItemBlock         `json:"bulleted_list_item,omitempty"`
	NumberedListItem *ListItemBlock         `json:"numbered_list_item,omitempty"`
	ToDo             *ToDoBlock             `json:"to_do,omitempty"`
	Toggle           *ToggleBlock           `json:"toggle,omitempty"`
	Quote            *QuoteBlock            `json:"quote,omitempty"`
	Code             *CodeBlock             `json:"code,omitempty"`
	Callout          *CalloutBlock          `json:"callout,omitempty"`
	Divider          *struct{}              `json:"divider,omitempty"`
	Image            *ImageBlock            `json:"image,omitempty"`
	Bookmark         *BookmarkBlock         `json:"bookmark,omitempty"`
	LinkPreview      *LinkPreviewBlock      `json:"link_preview,omitempty"`
	ChildPage        *ChildPageBlock        `json:"child_page,omitempty"`
	ChildDatabase    *ChildDatabaseBlock    `json:"child_database,omitempty"`
	Equation         *EquationBlock         `json:"equation,omitempty"`
	Table            *TableBlock            `json:"table,omitempty"`
	TableRow         *TableRowBlock         `json:"table_row,omitempty"`
	Video            *VideoBlock            `json:"video,omitempty"`
	Embed            *EmbedBlock            `json:"embed,omitempty"`

	// Children are fetched separately and attached here.
	Children []Block `json:"-"`
}

type ParagraphBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

type HeadingBlock struct {
	RichText     []RichText `json:"rich_text"`
	IsToggleable bool       `json:"is_toggleable"`
}

type ListItemBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

type ToDoBlock struct {
	RichText []RichText `json:"rich_text"`
	Checked  bool       `json:"checked"`
}

type ToggleBlock struct {
	RichText []RichText `json:"rich_text"`
}

type QuoteBlock struct {
	RichText []RichText `json:"rich_text"`
}

type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language string     `json:"language"`
	Caption  []RichText `json:"caption,omitempty"`
}

type CalloutBlock struct {
	RichText []RichText `json:"rich_text"`
	Icon     *Icon      `json:"icon,omitempty"`
}

type Icon struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

type ImageBlock struct {
	Type     string          `json:"type"`
	File     *FileObject     `json:"file,omitempty"`
	External *ExternalObject `json:"external,omitempty"`
	Caption  []RichText      `json:"caption,omitempty"`
}

func (b *ImageBlock) URL() string {
	if b.File != nil {
		return b.File.URL
	}
	if b.External != nil {
		return b.External.URL
	}
	return ""
}

type BookmarkBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption,omitempty"`
}

type LinkPreviewBlock struct {
	URL string `json:"url"`
}

type ChildPageBlock struct {
	Title string `json:"title"`
}

type ChildDatabaseBlock struct {
	Title string `json:"title"`
}

type EquationBlock struct {
	Expression string `json:"expression"`
}

type TableBlock struct {
	TableWidth      int  `json:"table_width"`
	HasColumnHeader bool `json:"has_column_header"`
	HasRowHeader    bool `json:"has_row_header"`
}

type TableRowBlock struct {
	Cells [][]RichText `json:"cells"`
}

type VideoBlock struct {
	Type     string          `json:"type"`
	File     *FileObject     `json:"file,omitempty"`
	External *ExternalObject `json:"external,omitempty"`
	Caption  []RichText      `json:"caption,omitempty"`
}

func (b *VideoBlock) URL() string {
	if b.File != nil {
		return b.File.URL
	}
	if b.External != nil {
		return b.External.URL
	}
	return ""
}

type EmbedBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption,omitempty"`
}

// Database represents a Notion database object.
type Database struct {
	ID          string                 `json:"id"`
	URL         string                 `json:"url"`
	Title       []RichText             `json:"title"`
	Properties  map[string]interface{}  `json:"properties"`
}

// DBQueryResult is the paginated response from querying a database.
type DBQueryResult struct {
	Object    string   `json:"object"`
	Results   []Page   `json:"results"`
	HasMore   bool     `json:"has_more"`
	NextCursor string  `json:"next_cursor"`
}

// ListResponse is the paginated response for listing blocks.
type ListResponse struct {
	Object  string  `json:"object"`
	Results []Block `json:"results"`
	HasMore bool    `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}
