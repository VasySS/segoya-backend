package websocket_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/infrastructure/transport"
	wsfake "github.com/VasySS/segoya-backend/tests/websocket"
)

func TestConcurrentRecordingsHaveDistinctStableSessionIDs(t *testing.T) {
	t.Parallel()

	session, other := &wsfake.Session{}, &wsfake.Session{}
	service := &wsfake.Service{}
	service.SetSessions(session, other)
	wantID := session.ID()
	ids := make(chan string, 32)

	var workers sync.WaitGroup
	for i := range 32 {
		workers.Go(func() {
			ids <- session.ID()

			session.Set(fmt.Sprintf("key-%d", i), i)
			session.SetBroadcastID("room")
			session.SendError("test error")
			_ = session.SendMessage("reply", map[string]any{"number": i})
			_ = service.Broadcast("room", transport.WebSocketMessageOutput{Type: "event"})
			_ = service.BroadcastOthers("room", session, transport.WebSocketMessageOutput{Type: "other event"})
			_ = session.Snapshot()
			_ = service.Broadcasts()
			_ = service.Sessions()
		})
	}

	workers.Wait()
	close(ids)

	for id := range ids {
		assert.Equal(t, wantID, id)
	}

	assert.NotEmpty(t, wantID)
	assert.NotEqual(t, wantID, other.ID())

	state := session.Snapshot()
	assert.Len(t, state.Values, 32)
	assert.Len(t, state.Errors, 32)
	assert.Len(t, state.Messages, 32)
	assert.Equal(t, "room", state.BroadcastID)
	assert.Len(t, service.Broadcasts(), 64)
}

func TestSnapshotsDoNotExposeRecordingContainers(t *testing.T) {
	t.Parallel()

	session := &wsfake.Session{}
	service := &wsfake.Service{}
	service.SetSessions(session)
	session.Set("key", "original")
	session.SendError("original")

	payload := map[string]any{"key": "original"}
	require.NoError(t, session.SendMessage("reply", payload))
	require.NoError(t, service.BroadcastOthers("room", session, transport.WebSocketMessageOutput{
		Type: "event", Payload: payload,
	}))

	payload["key"] = "changed after recording"
	state := session.Snapshot()
	state.Values["key"] = "changed snapshot"
	state.Errors[0] = "changed snapshot"
	state.Messages[0].Payload["key"] = "changed snapshot"
	service.Broadcasts()[0].Message.Payload["key"] = "changed snapshot"
	service.Sessions()[0] = nil

	assert.Equal(t, "original", session.Snapshot().Values["key"])
	assert.Equal(t, []string{"original"}, session.Snapshot().Errors)
	assert.Equal(t, "original", session.Snapshot().Messages[0].Payload["key"])
	assert.Equal(t, "original", service.Broadcasts()[0].Message.Payload["key"])
	assert.Equal(t, session.ID(), service.Broadcasts()[0].ExcludedSessionID)
	assert.Equal(t, "room", service.Broadcasts()[0].RoomID)
	assert.Same(t, session, service.Sessions()[0])
}
