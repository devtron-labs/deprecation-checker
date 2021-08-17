package pkg

import "testing"

func TestRegexMatch(t *testing.T) {
	type args struct {
		s       string
		pattern string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "ends with match",
			args: args{
				s:       "spec/jobTemplate/spec/template/metadata/creationTimestamp",
				pattern: "*creationTimestamp",
			},
			want: true,
		},
		{
			name: "starts with match",
			args: args{
				s:       "spec/jobTemplate/spec/template/metadata/creationTimestamp",
				pattern: "spec*",
			},
			want: true,
		},
		{
			name: "has match",
			args: args{
				s:       "spec/jobTemplate/spec/template/metadata/creationTimestamp",
				pattern: "*job*",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RegexMatch(tt.args.s, tt.args.pattern); got != tt.want {
				t.Errorf("RegexMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
