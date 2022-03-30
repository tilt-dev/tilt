package snapshot

import (
	"fmt"

	"github.com/mattn/go-jsonpointer"
)

// Functions to determine Tilt version from a Snapshot

// This code was copy/pasted from TFT
// TODO: have TFT call this instead of having its own copy of the code

func GetVersionFromSnapshot(snapshot map[string]interface{}) (string, error) {
	view, hasView := snapshot["view"].(map[string]interface{})
	if !hasView {
		return GetVersionOrSHAFromSnapshotV1(snapshot)
	}

	_, hasUISession := view["uiSession"]
	if !hasUISession {
		return GetVersionOrSHAFromSnapshotV2(snapshot)
	}

	return GetVersionOrSHAFromSnapshotV3(snapshot)
}

func GetVersionOrSHAFromSnapshotV3(snapshot map[string]interface{}) (string, error) {
	v, err := jsonpointer.Get(snapshot, "/view/uiSession/status/runningTiltBuild/version")
	if err != nil {
		return "", fmt.Errorf("Unable to parse Version: %s", err.Error())
	}
	devVal, err := jsonpointer.Get(snapshot, "/view/uiSession/status/runningTiltBuild/dev")
	isDev := false
	if err == nil {
		devValBool, ok := devVal.(bool)
		if ok {
			isDev = devValBool
		}
	}
	s, err := jsonpointer.Get(snapshot, "/view/uiSession/status/runningTiltBuild/commitSHA")
	sha := ""
	if err != nil {
		// this is OK, old snapshots won't have this
	} else {
		sha = fmt.Sprintf("%s", s)
	}
	version := fmt.Sprintf("%s", v)

	if sha != "" && isDev {
		return sha, nil
	}

	if version == "" {
		return "", fmt.Errorf("Unable to parse Version: empty version")
	}

	return fmt.Sprintf("v%s", version), nil
}

func GetVersionOrSHAFromSnapshotV2(snapshot map[string]interface{}) (string, error) {
	v, err := jsonpointer.Get(snapshot, "/view/runningTiltBuild/version")
	if err != nil {
		return "", fmt.Errorf("Unable to parse Version: %s", err.Error())
	}
	isDev := false
	if jsonpointer.Has(snapshot, "/view/runningTiltBuild/dev") {
		dev, err := jsonpointer.Get(snapshot, "/view/runningTiltBuild/dev")
		if err != nil {
			return "", fmt.Errorf("Unable to find Dev flag: %s", err.Error())
		}
		var ok bool
		isDev, ok = dev.(bool)
		if !ok {
			return "", fmt.Errorf("Unable to parse dev (%v) as bool", dev)
		}
	}
	s, err := jsonpointer.Get(snapshot, "/view/runningTiltBuild/commitSHA")
	sha := ""
	if err != nil {
		// this is OK, old snapshots won't have this
	} else {
		sha = fmt.Sprintf("%s", s)
	}
	version := fmt.Sprintf("%s", v)

	if sha != "" && isDev {
		return sha, nil
	}

	if version == "" {
		return "", fmt.Errorf("Unable to parse Version: empty version")
	}

	return fmt.Sprintf("v%s", version), nil
}

func GetVersionOrSHAFromSnapshotV1(snapshot map[string]interface{}) (string, error) {
	v, err := jsonpointer.Get(snapshot, "/View/RunningTiltBuild/Version")
	if err != nil {
		return "", fmt.Errorf("Unable to parse Version: %s", err.Error())
	}
	dev, err := jsonpointer.Get(snapshot, "/View/RunningTiltBuild/Dev")
	if err != nil {
		return "", fmt.Errorf("Unable to find Dev flag: %s", err.Error())
	}
	isDev, ok := dev.(bool)
	if !ok {
		return "", fmt.Errorf("Unable to parse dev (%v) as bool", dev)
	}
	s, err := jsonpointer.Get(snapshot, "/View/RunningTiltBuild/CommitSHA")
	sha := ""
	if err != nil {
		// this is OK, old snapshots won't have this
	} else {
		sha = fmt.Sprintf("%s", s)
	}
	version := fmt.Sprintf("%s", v)

	if sha != "" && isDev {
		return sha, nil
	}

	if version == "" {
		return "", fmt.Errorf("Unable to parse Version: empty version")
	}

	return fmt.Sprintf("v%s", version), nil
}
