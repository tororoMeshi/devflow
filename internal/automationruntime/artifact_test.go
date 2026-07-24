package automationruntime

import (
	"reflect"
	"testing"
)

func TestResolveArtifactTargets(t *testing.T) {
	artifacts := []ArtifactContract{{"a", false}, {"b", true}, {"c", false}}
	tests := []struct {
		outcome string
		refs    []string
		want    []ArtifactTarget
	}{
		{"completed", []string{"a"}, []ArtifactTarget{{"a", false}, {"b", true}}},
		{"blocked", []string{"c"}, []ArtifactTarget{{"c", false}}},
		{"failed", nil, []ArtifactTarget{}},
	}
	for _, tt := range tests {
		got, err := ResolveArtifactTargets(tt.outcome, artifacts, tt.refs)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s = %#v, %v", tt.outcome, got, err)
		}
	}
	for _, refs := range [][]string{{"missing"}, {"a", "a"}} {
		if _, err := ResolveArtifactTargets("completed", artifacts, refs); err == nil {
			t.Errorf("accepted refs %#v", refs)
		}
	}
}
