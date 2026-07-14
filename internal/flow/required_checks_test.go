package flow

import "testing"

func TestRequiredChecksValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		checks string
		want   ErrorCode
	}{
		{name: "accepts valid checks", checks: `required_checks: ["go-test", "go_vet"]`},
		{name: "rejects empty check", checks: `required_checks: [""]`, want: ErrorMissingRequiredCheckID},
		{name: "rejects invalid check", checks: `required_checks: ["go test"]`, want: ErrorInvalidRequiredCheckID},
		{name: "rejects duplicate check", checks: `required_checks: ["go-test", "go-test"]`, want: ErrorDuplicateRequiredCheckID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load([]byte(`flow: { id: "test", title: "Test", steps: [{ id: "step", title: "Step", instruction: "Do.", ` + tt.checks + ` }] }`))
			if tt.want == "" {
				if err != nil || len(got.Steps[0].RequiredChecks) != 2 {
					t.Fatalf("flow=%#v err=%v", got, err)
				}
				return
			}
			validation, ok := err.(*ValidationError)
			if !ok || validation.Code != tt.want {
				t.Fatalf("err=%v, want %s", err, tt.want)
			}
		})
	}
}
