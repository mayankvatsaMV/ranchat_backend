package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func testWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	serverConnections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnections <- conn
	}))
	t.Cleanup(server.Close)

	client, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial test websocket: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	select {
	case serverConn := <-serverConnections:
		t.Cleanup(func() { _ = serverConn.Close() })
		return serverConn, client
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server websocket connection")
		return nil, nil
	}
}

func TestUnregisterOldConnectionKeepsReplacementRegistered(t *testing.T) {
	hub := NewWebSocketHub()
	oldServerConn, _ := testWebSocketPair(t)
	newServerConn, newClientConn := testWebSocketPair(t)

	hub.Register("receiver", oldServerConn)
	hub.Register("receiver", newServerConn)
	hub.Unregister("receiver", oldServerConn)

	want := map[string]string{"event": "new_message"}
	if !hub.SendToUser("receiver", want) {
		t.Fatal("SendToUser returned false for the replacement connection")
	}

	_ = newClientConn.SetReadDeadline(time.Now().Add(time.Second))
	var got map[string]string
	if err := newClientConn.ReadJSON(&got); err != nil {
		t.Fatalf("read message from replacement connection: %v", err)
	}
	if got["event"] != want["event"] {
		t.Fatalf("got event %q, want %q", got["event"], want["event"])
	}
}
