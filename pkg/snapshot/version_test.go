package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"golang.org/x/xerrors"
)

func TestGetVersionFromProdSnapshot(t *testing.T) {
	snapshot := map[string]interface{}{
		"View": map[string]interface{}{
			"RunningTiltBuild": map[string]interface{}{
				"Version": "0.7.13",
				"Dev":     false,
			},
		},
	}

	expected := "v0.7.13"
	actual, err := GetVersionFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetVersionFromUISession(t *testing.T) {
	snapshot := map[string]interface{}{
		"view": map[string]interface{}{
			"uiSession": map[string]interface{}{
				"status": map[string]interface{}{
					"runningTiltBuild": map[string]interface{}{
						"version": "0.20.2",
					},
				},
			},
		},
	}

	expected := "v0.20.2"
	actual, err := GetVersionFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetVersionFromProdSnapshotV2(t *testing.T) {
	snapshot := map[string]interface{}{
		"view": map[string]interface{}{
			"runningTiltBuild": map[string]interface{}{
				"version": "0.7.13",
				"dev":     false,
			},
		},
	}

	expected := "v0.7.13"
	actual, err := GetVersionFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetVersionFromDevSnapshot(t *testing.T) {
	snapshot := map[string]interface{}{
		"View": map[string]interface{}{
			"RunningTiltBuild": map[string]interface{}{
				"Version":   "v0.10.13",
				"Dev":       true,
				"CommitSHA": "aaaaaa",
			},
		},
	}

	expected := "aaaaaa"
	actual, err := GetVersionFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetVersionFromDevSnapshotV2(t *testing.T) {
	snapshot := map[string]interface{}{
		"view": map[string]interface{}{
			"runningTiltBuild": map[string]interface{}{
				"version":   "v0.10.13",
				"dev":       true,
				"commitSHA": "aaaaaa",
			},
		},
	}

	expected := "aaaaaa"
	actual, err := GetVersionFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetVersionFromDevSnapshotDevIsFalse(t *testing.T) {
	snapshot := map[string]interface{}{
		"View": map[string]interface{}{
			"RunningTiltBuild": map[string]interface{}{
				"Version":   "0.10.13",
				"Dev":       false,
				"CommitSHA": "aaaaaa",
			},
		},
	}

	expected := "v0.10.13"
	actual, err := GetVersionFromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetEmptyVersionFromMalformedSnapshot(t *testing.T) {
	snapshot := map[string]interface{}{
		"Foo": map[string]interface{}{
			"Bar": "",
		},
	}

	expected := ""
	actual, err := GetVersionFromSnapshot(snapshot)
	if err == nil {
		t.Fatal("expected error")
	}

	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestShouldNotParseAsProtobuf(t *testing.T) {
	b, err := os.ReadFile("testdata/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}

	shouldParse, err := shouldParseAsProtobuf(b)
	if err != nil {
		t.Fatal(err)
	}

	if shouldParse != false {
		t.Errorf("Expected shouldParse to be false, got true")
	}
}

func TestShouldParseAsProtobuf(t *testing.T) {
	b, err := os.ReadFile("testdata/snapshot_new.json")
	if err != nil {
		t.Fatal(err)
	}

	shouldParse, err := shouldParseAsProtobuf(b)
	if err != nil {
		t.Fatal(err)
	}

	if shouldParse != true {
		t.Errorf("Expected shouldParse to be true, got false")
	}
}

func shouldParseAsProtobuf(b []byte) (bool, error) {
	snapMap := make(map[string]interface{})

	err := json.Unmarshal(b, &snapMap)
	if err != nil {
		msg := fmt.Sprintf("Error decoding snapshot as JSON: %v\n", err)
		err = xerrors.New(msg)
		return false, err
	}

	_, isV2 := snapMap["view"]

	return isV2, nil
}
