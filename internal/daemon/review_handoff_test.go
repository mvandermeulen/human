package daemon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEngineeringKeysFromHandoff(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "happy path single key",
			body: "[human:ready-for-review]\nengineering: HUM-89\nbranch: main\ncommits: 2037e40",
			want: []string{"HUM-89"},
		},
		{
			name: "multiple keys with whitespace",
			body: "[human:ready-for-review]\nengineering: HUM-89,  HUM-90, HUM-91 \nbranch: main",
			want: []string{"HUM-89", "HUM-90", "HUM-91"},
		},
		{
			name: "body must start with header so quoted references don't trigger",
			body: "> [human:ready-for-review]\n> engineering: HUM-89",
			want: nil,
		},
		{
			name: "header present but engineering line missing",
			body: "[human:ready-for-review]\nbranch: main",
			want: nil,
		},
		{
			name: "not a handoff",
			body: "Looks good to me!",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEngineeringKeysFromHandoff(tt.body)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsReviewComplete(t *testing.T) {
	assert.True(t, IsReviewComplete("[human:review-complete]\nverdict: pass"))
	assert.True(t, IsReviewComplete("  [human:review-complete]\nverdict: pass"))
	assert.False(t, IsReviewComplete("[human:ready-for-review]"))
	assert.False(t, IsReviewComplete("plain comment"))
}
