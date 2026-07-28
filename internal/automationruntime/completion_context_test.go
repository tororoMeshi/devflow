package automationruntime

import (
	"bytes"
	"testing"
)

func TestParseCompletionContext(t *testing.T) {
	valid := []byte(`{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","attempt_status":"active","is_current_attempt":true,"artifacts":[{"path":"out.txt","required":true,"status":"recorded","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":0}],"checks":[{"id":"unit","status":"passed","exit_code":0}],"approval":{"required":false,"status":"not_required","evidence_set_digest":null,"approved_evidence_set_digest":null},"completion":{"status":"ready","blocker":null}}`)
	if got, err := parseCompletionContext(valid, "run_1", "build", "attempt_1"); err != nil || !bytes.Equal(got, valid) {
		t.Fatalf("parseCompletionContext() = %s, %v", got, err)
	}
	empty := []byte(`{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","attempt_status":"closed","is_current_attempt":false,"artifacts":[],"checks":[],"approval":{"required":false,"status":"not_required","evidence_set_digest":null,"approved_evidence_set_digest":null},"completion":{"status":"not_applicable","blocker":{"code":"attempt_closed","subject_id":null}}}`)
	if got, err := parseCompletionContext(empty, "run_1", "build", "attempt_1"); err != nil || !bytes.Equal(got, empty) {
		t.Fatalf("empty parseCompletionContext() = %s, %v", got, err)
	}
	for _, data := range [][]byte{
		bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"unknown":true`), 1),
		bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema\\u005fversion":1`), 1),
		append(append([]byte(nil), valid...), []byte(` {}`)...),
		bytes.Replace(valid, []byte(`"flow_run_id":"run_1"`), []byte(`"flow_run_id":null`), 1),
		bytes.Replace(valid, []byte(`"flow_run_id":"run_1"`), []byte(`"flow_run_id":"other"`), 1),
		bytes.Replace(valid, []byte(`"path":"out.txt"`), []byte(`"path":"/out.txt"`), 1),
		bytes.Replace(valid, []byte(`"artifacts":[`), []byte(`"artifacts":null`), 1),
	} {
		if _, err := parseCompletionContext(data, "run_1", "build", "attempt_1"); err == nil {
			t.Fatalf("accepted invalid completion context: %s", data)
		}
	}
}
