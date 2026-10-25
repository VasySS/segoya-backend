package melody_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"

	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport/melody"
)

func TestLobbyWebSocketPayloadLimit(t *testing.T) {
	t.Parallel()

	for _, size := range []int{1024, playarea.MaxBodyBytes + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()

			service := melody.NewWebSocketServiceWithLimit(playarea.MaxBodyBytes)

			t.Cleanup(func() { require.NoError(t, service.Close()) })
			service.SetMessageHandler(func(session transport.WebSocketSession, _ transport.WebSocketMessageInput) {
				_ = session.SendMessage("accepted", map[string]any{"received": true})
			})

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = service.HandleRequest(w, r)
			}))
			t.Cleanup(server.Close)
			config, err := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http"), server.URL)
			require.NoError(t, err)

			connection, err := config.DialContext(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = connection.Close() })
			require.NoError(t, connection.SetReadDeadline(time.Now().Add(5*time.Second)))

			payload, err := json.Marshal(strings.Repeat("x", size))
			require.NoError(t, err)
			require.NoError(t, websocket.JSON.Send(connection, transport.WebSocketMessageInput{
				Type: "settingsNew", Payload: payload,
			}))

			var message string

			err = websocket.Message.Receive(connection, &message)
			if size > playarea.MaxBodyBytes {
				require.ErrorIs(t, err, io.EOF)
				return
			}

			require.NoError(t, err)
			require.Contains(t, message, "accepted")
		})
	}
}
