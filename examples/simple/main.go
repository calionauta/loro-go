// Command simple is a short tour of the loro-go API: writing to a few
// container types, syncing two documents, and reading values back.
package main

import (
	"fmt"
	"log"

	"github.com/calionauta/loro-go"
)

func main() {
	doc := loro.NewLoroDoc()

	// Maps hold keyed values. InsertAny takes plain Go values.
	settings := doc.GetMap(loro.AsContainerId("settings"))
	check(settings.InsertAny("theme", "dark"))
	check(settings.InsertAny("font-size", int64(14)))

	// Containers can nest. GetOrCreateLoroMap returns the child if it exists,
	// or creates and attaches one if it doesn't.
	users, err := settings.GetOrCreateLoroMap("users")
	check(err)
	alice, err := users.GetOrCreateLoroMap("alice")
	check(err)
	check(alice.InsertAny("name", "Alice"))

	// LoroText is a collaborative string. Edits at the same position from
	// different peers merge cleanly.
	note := doc.GetText(loro.AsContainerId("note"))
	check(note.Insert(0, "Hello, world!"))

	// Movable lists support reordering without losing identity.
	tasks := doc.GetMovableList(loro.AsContainerId("tasks"))
	check(tasks.PushAny("write docs"))
	check(tasks.PushAny("ship it"))
	check(tasks.Mov(1, 0))

	// Export a self-contained snapshot. Snapshots include full history and
	// are the easiest payload for a fresh peer to import.
	snapshot, err := doc.Export(loro.SnapshotMode())
	check(err)
	fmt.Printf("snapshot: %d bytes\n", len(snapshot))

	// A fresh peer imports the snapshot and sees the same state.
	other := loro.NewLoroDoc()
	if _, err := other.Import(snapshot); err != nil {
		log.Fatalf("import snapshot: %v", err)
	}

	// After the snapshot, sync incremental changes by exchanging update
	// payloads built against the remote peer's version vector.
	check(doc.GetMap(loro.AsContainerId("settings")).InsertAny("theme", "light"))
	delta, err := doc.Export(loro.UpdatesMode(other.StateVv()))
	check(err)
	if _, err := other.Import(delta); err != nil {
		log.Fatalf("import delta: %v", err)
	}
	fmt.Printf("delta: %d bytes\n", len(delta))

	// Read values back from the synced peer.
	theme, _ := other.GetMap(loro.AsContainerId("settings")).GetString("theme")
	otherNote := other.GetText(loro.AsContainerId("note"))
	fmt.Printf("theme=%s note=%q\n", theme, otherNote.String())

	// FindByPath walks containers by key/index without intermediate lookups.
	if v := other.FindByPath("settings", "users", "alice", "name"); v != nil {
		name, _ := loro.GetStringValue(&v)
		fmt.Printf("alice.name=%s\n", name)
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
