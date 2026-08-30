package daemon

import (
	"encoding/json"
	"sync"
	"time"
)

// event is one SSE message. There are three, all sent on a document's stream:
//
//   - refresh: the whole document as raw Markdown, plus an optional line and
//     viewport ratio. The browser re-renders, and scrolls only if a line came
//     with it.
//   - scroll: a line and an optional viewport ratio, with no content. The
//     browser only scrolls.
//   - close: the document is no longer open. The browser says so and stops
//     reconnecting.
//
// data is already JSON encoded on a single line: a bare newline inside `data:`
// would be interpreted by the SSE framing.
type event struct {
	name string
	data string
}

func newEvent(name string, payload any) event {
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte("{}")
	}
	return event{name: name, data: string(data)}
}

// subscriber is one connected browser (one SSE request).
type subscriber struct {
	ch chan event
}

// hub fans events out to the browsers watching each document.
type hub struct {
	mu   sync.Mutex
	subs map[string]map[*subscriber]bool // doc id -> subscribers

	// lastEmpty is when the number of connections last dropped to zero, used
	// for the idle shutdown.
	lastEmpty time.Time
}

func newHub() *hub {
	return &hub{
		subs:      map[string]map[*subscriber]bool{},
		lastEmpty: time.Now(),
	}
}

func (h *hub) subscribe(id string) *subscriber {
	s := &subscriber{ch: make(chan event, 8)}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[id] == nil {
		h.subs[id] = map[*subscriber]bool{}
	}
	h.subs[id][s] = true
	return s
}

func (h *hub) unsubscribe(id string, s *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs := h.subs[id]; subs != nil {
		delete(subs, s)
		if len(subs) == 0 {
			delete(h.subs, id)
		}
	}
	close(s.ch)
	if len(h.subs) == 0 {
		h.lastEmpty = time.Now()
	}
}

func (h *hub) broadcast(id string, ev event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[id] {
		select {
		case s.ch <- ev:
		default:
			// A browser that cannot keep up gets dropped rather than
			// blocking the daemon. Its EventSource will reconnect and
			// receive the current content again.
			delete(h.subs[id], s)
			close(s.ch)
		}
	}
	if len(h.subs[id]) == 0 {
		delete(h.subs, id)
		if len(h.subs) == 0 {
			h.lastEmpty = time.Now()
		}
	}
}

// idleSince reports how long no browser has been connected. It returns zero
// while at least one connection is alive.
func (h *hub) idleSince() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) > 0 {
		return 0
	}
	return time.Since(h.lastEmpty)
}
