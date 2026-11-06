package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMessageType(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "client ready",
			message: `{"event":"message","data":{"component":"pad","type":"CLIENT_READY","padId":"p","token":"t.x"}}`,
			want:    msgClientReady,
		},
		{
			name:    "timeslider changeset request",
			message: `{"event":"message","data":{"component":"pad","type":"CHANGESET_REQ","data":{"start":0,"granularity":1},"padId":"p"}}`,
			want:    msgChangesetReq,
		},
		{
			// Regression: the inserted text contains other message type
			// names. The old strings.Contains dispatch routed this as
			// CLIENT_READY and the edit was lost.
			name:    "user changes whose text contains other message types",
			message: `{"event":"message","data":{"type":"COLLABROOM","component":"pad","data":{"type":"USER_CHANGES","baseRev":3,"changeset":"Z:1>c*0+c$CLIENT_READY","apool":{"numToAttrib":{},"nextNum":0}}}}`,
			want:    msgUserChanges,
		},
		{
			name:    "chat message mentioning PAD_DELETE",
			message: `{"event":"message","data":{"type":"COLLABROOM","component":"pad","data":{"type":"CHAT_MESSAGE","message":{"text":"please don't PAD_DELETE or SAVE_REVISION"}}}}`,
			want:    msgChatMessage,
		},
		{
			name:    "pad delete",
			message: `{"event":"message","data":{"type":"COLLABROOM","component":"pad","data":{"type":"PAD_DELETE","data":{"padId":"p"}}}}`,
			want:    msgPadDelete,
		},
		{
			name:    "sheet op",
			message: `{"event":"message","data":{"type":"COLLABROOM","component":"sheet","data":{"type":"SHEET_OP","baseRev":1}}}`,
			want:    msgSheetOp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := parseMessageType([]byte(tt.message))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseMessageTypeRejectsMalformed(t *testing.T) {
	for _, m := range []string{
		`not json`,
		`{"event":"message","data":{}}`,
		`{"event":"message","data":{"type":"COLLABROOM"}}`,
		`{"event":"message","data":{"type":"COLLABROOM","data":{}}}`,
		// A bare string mentioning a type must not be routed anywhere.
		`"USER_CHANGES"`,
	} {
		_, _, err := parseMessageType([]byte(m))
		assert.Error(t, err, m)
	}
}

// PAD_DELETE must be decoded from the inner payload; decoding the envelope
// left PadID empty, so deleting a pad from the UI never worked.
func TestPadDeleteDecodedFromInnerPayload(t *testing.T) {
	msg := `{"event":"message","data":{"type":"COLLABROOM","component":"pad","data":{"type":"PAD_DELETE","data":{"padId":"myPad"}}}}`
	_, inner, err := parseMessageType([]byte(msg))
	require.NoError(t, err)
	var pd PadDelete
	require.NoError(t, json.Unmarshal(inner, &pd))
	assert.Equal(t, "PAD_DELETE", pd.Type)
	assert.Equal(t, "myPad", pd.Data.PadID)
}
