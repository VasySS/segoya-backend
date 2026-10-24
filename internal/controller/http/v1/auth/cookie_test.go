//nolint:testpackage // Exercise private cookie parsing without expanding the production API.
package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCookieState(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, cookie, want string
		wantErr            error
		invalid            bool
	}{
		{name: "plain", cookie: "oauthState=state", want: "state"},
		{name: "among other cookies", cookie: "other=value; oauthState=state; last=value", want: "state"},
		{name: "escaped value", cookie: "oauthState=state%2Bvalue", want: "state+value"},
		{name: "blank header", invalid: true},
		{name: "unrelated cookie", cookie: "other=state", wantErr: ErrCookieNotFound},
		{name: "empty", cookie: "oauthState=", invalid: true},
		{name: "malformed encoding", cookie: "oauthState=%zz", invalid: true},
		{name: "malformed cookie", cookie: "oauthState=\"unterminated", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := (Handler{}).parseCookieState(tc.cookie)
			if tc.wantErr != nil || tc.invalid {
				require.Error(t, err)

				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
				}

				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
