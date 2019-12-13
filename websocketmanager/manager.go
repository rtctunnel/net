package websocketmanager

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

type webSocket struct {
	// these properties are controlled by the manager
	active     int
	lastAccess time.Time

	mu   sync.Mutex
	conn *websocket.Conn
}

// A Manager manages websocket connections and automatically closes them if they are idle.
type Manager struct {
	cfg *config

	mu         sync.Mutex
	webSockets map[string]*webSocket

	closeOnce sync.Once
	closed    chan struct{}
}

func New(options ...Option) *Manager {
	mgr := &Manager{
		cfg:        getConfig(options...),
		webSockets: make(map[string]*webSocket),
		closed:     make(chan struct{}),
	}
	go mgr.cleaner()
	return mgr
}

func (mgr *Manager) cleaner() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		var now time.Time
		select {
		case <-mgr.closed:
			return
		case now = <-ticker.C:
		}

		cutoff := now.Add(mgr.cfg.idleTimeout)
		var toclose []*websocket.Conn
		mgr.mu.Lock()
		for wsurl, ws := range mgr.webSockets {
			if ws.active == 0 && ws.lastAccess.Before(cutoff) {
				toclose = append(toclose, ws.conn)
				delete(mgr.webSockets, wsurl)
			}
		}
		mgr.mu.Unlock()

		for _, conn := range toclose {
			if conn != nil {
				_ = conn.Close()
			}
		}
	}
}

func (mgr *Manager) Close() error {
	mgr.closeOnce.Do(func() {
		mgr.mu.Lock()
		for _, ws := range mgr.webSockets {
			_ = ws.conn.Close()
		}
		mgr.mu.Unlock()
		close(mgr.closed)
	})
	return nil
}

func (mgr *Manager) WithConnection(ctx context.Context, url string, cb func(conn *websocket.Conn, isNew bool) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
		case <-mgr.closed:
			cancel()
		}
	}()

	wsurl := url
	switch {
	case strings.HasPrefix(url, "https://"):
		wsurl = "wss://" + url[len("https://"):]
	case strings.HasPrefix(url, "http://"):
		wsurl = "ws://" + url[len("http://"):]
	}

	mgr.mu.Lock()
	ws, ok := mgr.webSockets[wsurl]
	if !ok {
		ws = new(webSocket)
		mgr.webSockets[wsurl] = ws
	}
	ws.active++
	mgr.mu.Unlock()

	defer func() {
		mgr.mu.Lock()
		ws.active--
		ws.lastAccess = time.Now()
		mgr.mu.Unlock()
	}()

	ws.mu.Lock()
	defer ws.mu.Unlock()

	isNew := ws.conn == nil
	if ws.conn == nil {
		hdrs := http.Header{
			"Origin": {url},
		}

		conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsurl, hdrs)
		if err != nil {
			var msg string
			if resp != nil && resp.Body != nil {
				bs, _ := ioutil.ReadAll(resp.Body)
				msg = string(bs)
			}
			return fmt.Errorf("error connecting to websocket (url=%s msg=%s): %w", wsurl, msg, err)
		}

		ws.conn = conn
	}

	err := cb(ws.conn, isNew)
	if err != nil {
		_ = ws.conn.Close()
		ws.conn = nil
	}

	return err
}
