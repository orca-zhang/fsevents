//go:build darwin

package fsevents

import (
	"testing"
	"time"
)

func TestCreatePath(t *testing.T) {
	ref, err := createPaths([]string{"/a", "/b"})
	if err != nil {
		t.Fatal(err)
	}

	if e := 2; CFArrayLen(ref) != e {
		t.Errorf("got: %d wanted: %d", CFArrayLen(ref), e)
	}
}

func TestEventStream(t *testing.T) {
	eid := uint64(42)
	did := int32(12)
	paths := []string{"/a", "/b"}
	ref := setupStream(paths, 0, 0, eid, time.Duration(0), did)

	if e := getStreamRefEventID(ref); eid != e {
		t.Errorf("got: %d wanted: %d", e, eid)
	}

	if e := getStreamRefDeviceID(ref); did != e {
		t.Errorf("got: %d wanted: %d", e, did)
	}

	spaths := getStreamRefPaths(ref)
	for i := range paths {
		if paths[i] != spaths[i] {
			t.Errorf("pos %d got: %s wanted: %s", i, spaths[i], paths[i])
		}
	}
}

func TestDeviceID(t *testing.T) {
	// Verify compatible devide ID is returned
	// Probably a way to verify this UUID as well...

	did, err := DeviceForPath("/")
	if err != nil {
		t.Fatal(err)
	}

	if len(GetDeviceUUID(did)) == 0 {
		t.Fatal("failed to read device ID")
	}
}

func TestEventIDSinceNow(t *testing.T) {
	// from FSEvents.h:
	//   kFSEventStreamEventIdSinceNow = 0xFFFFFFFFFFFFFFFFULL
	const expected = uint64(0xFFFFFFFFFFFFFFFF)
	if eventIDSinceNow != expected {
		t.Errorf("got: %v wanted: %v", eventIDSinceNow, expected)
	}
}
