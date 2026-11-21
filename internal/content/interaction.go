package content

// Interaction is a reader's response from the open web or (later) Backfeed.
// SourceURL identifies it across feeds. Text and author metadata are plain text.
type Interaction struct {
	Kind       InteractionKind
	SourceURL  string
	AuthorName string
	AuthorURL  string
	PhotoURL   string
	Text       string
	RSVP       string
}

type InteractionKind string

const (
	Like     InteractionKind = "like"
	Repost   InteractionKind = "repost"
	Reply    InteractionKind = "reply"
	Mention  InteractionKind = "mention"
	Bookmark InteractionKind = "bookmark"
	RSVP     InteractionKind = "rsvp"
)
