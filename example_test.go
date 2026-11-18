package loro_test

import (
	"fmt"
	"log"
	"sort"

	"github.com/calionauta/loro-go"
)

// Basic usage: create a document, write to a root map, and read a typed value
// back.
func Example() {
	doc := loro.NewLoroDoc()

	settings := doc.GetMap(loro.AsContainerId("settings"))
	if err := settings.InsertAny("theme", "dark"); err != nil {
		log.Fatal(err)
	}

	theme, ok := settings.GetString("theme")
	fmt.Println(theme, ok)
	// Output: dark true
}

// LoroText is a collaborative string. Insert, append, and slice operations
// are all index-based.
func ExampleLoroText() {
	doc := loro.NewLoroDoc()
	t := doc.GetText(loro.AsContainerId("note"))

	if err := t.Insert(0, "Hello, world!"); err != nil {
		log.Fatal(err)
	}
	if err := t.Insert(7, "Loro "); err != nil {
		log.Fatal(err)
	}

	fmt.Println(t)
	// Output: Hello, Loro world!
}

// LoroText implements io.Writer and provides Append/Appendf helpers, so it
// composes with the standard library's text utilities.
func ExampleLoroText_Append() {
	doc := loro.NewLoroDoc()
	t := doc.GetText(loro.AsContainerId("log"))

	if err := t.Append("started\n"); err != nil {
		log.Fatal(err)
	}
	if err := t.Appendf("user=%s id=%d\n", "alice", 42); err != nil {
		log.Fatal(err)
	}
	if _, err := fmt.Fprintln(t, "done"); err != nil {
		log.Fatal(err)
	}

	fmt.Print(t)
	// Output:
	// started
	// user=alice id=42
	// done
}

// GetOrCreateLoroMap returns the child container at key, attaching a fresh
// empty one if it does not already exist. The same helper exists for every
// child container type (List, MovableList, Text, Tree, Counter).
func ExampleLoroMap_GetOrCreateLoroMap() {
	doc := loro.NewLoroDoc()
	root := doc.GetMap(loro.AsContainerId("doc"))

	users, err := root.GetOrCreateLoroMap("users")
	if err != nil {
		log.Fatal(err)
	}
	alice, err := users.GetOrCreateLoroMap("alice")
	if err != nil {
		log.Fatal(err)
	}
	if err := alice.InsertAny("name", "Alice"); err != nil {
		log.Fatal(err)
	}

	name, _ := alice.GetString("name")
	fmt.Println(name)
	// Output: Alice
}

// All yields (index, *ValueOrContainer) pairs so callers can range over a list
// directly without materialising a slice. Iteration stops early on break.
func ExampleLoroList_All() {
	doc := loro.NewLoroDoc()
	l := doc.GetList(loro.AsContainerId("colors"))
	for _, c := range []string{"red", "green", "blue"} {
		if err := l.PushAny(c); err != nil {
			log.Fatal(err)
		}
	}

	for i, v := range l.All() {
		s, _ := loro.GetStringValue(&v)
		fmt.Printf("%d=%s\n", i, s)
	}
	// Output:
	// 0=red
	// 1=green
	// 2=blue
}

// LoroMovableList lets entries be reordered without losing their identity.
// Other peers see the move rather than a delete + reinsert.
func ExampleLoroMovableList_Mov() {
	doc := loro.NewLoroDoc()
	tasks := doc.GetMovableList(loro.AsContainerId("tasks"))
	for _, t := range []string{"write docs", "review PR", "ship it"} {
		if err := tasks.PushAny(t); err != nil {
			log.Fatal(err)
		}
	}

	if err := tasks.Mov(2, 0); err != nil {
		log.Fatal(err)
	}

	for i, v := range tasks.All() {
		s, _ := loro.GetStringValue(&v)
		fmt.Printf("%d=%s\n", i, s)
	}
	// Output:
	// 0=ship it
	// 1=write docs
	// 2=review PR
}

// FindByPath resolves a value by walking containers from the document root.
// String parts select map keys; integer parts select list indices. Returns
// nil when the path does not resolve.
func ExampleLoroDoc_FindByPath() {
	doc := loro.NewLoroDoc()

	root := doc.GetMap(loro.AsContainerId("root"))
	users, err := root.GetOrCreateLoroList("users")
	if err != nil {
		log.Fatal(err)
	}
	alice, err := users.InsertLoroMap(0)
	if err != nil {
		log.Fatal(err)
	}
	if err := alice.InsertAny("name", "Alice"); err != nil {
		log.Fatal(err)
	}

	v := doc.FindByPath("root", "users", 0, "name")
	name, _ := loro.GetStringValue(&v)
	fmt.Println(name)
	// Output: Alice
}

// Export with SnapshotMode produces a self-contained payload, including
// history, that a fresh peer can import to reach the same state.
func ExampleLoroDoc_Export() {
	src := loro.NewLoroDoc()
	if err := src.GetMap(loro.AsContainerId("m")).InsertAny("k", "v"); err != nil {
		log.Fatal(err)
	}

	snapshot, err := src.Export(loro.SnapshotMode())
	if err != nil {
		log.Fatal(err)
	}

	dst := loro.NewLoroDoc()
	if _, err := dst.Import(snapshot); err != nil {
		log.Fatal(err)
	}

	got, _ := dst.GetMap(loro.AsContainerId("m")).GetString("k")
	fmt.Println(got)
	// Output: v
}

// After a peer has the initial snapshot, follow-up changes can be shipped as
// updates built against that peer's version vector. Each peer applies the
// other's updates and both converge to the same state.
func ExampleLoroDoc_Export_updates() {
	a := loro.NewLoroDoc()
	b := loro.NewLoroDoc()

	if err := a.GetMap(loro.AsContainerId("m")).InsertAny("from-a", int64(1)); err != nil {
		log.Fatal(err)
	}
	if err := b.GetMap(loro.AsContainerId("m")).InsertAny("from-b", int64(2)); err != nil {
		log.Fatal(err)
	}

	aUpdates, err := a.Export(loro.UpdatesMode(b.StateVv()))
	if err != nil {
		log.Fatal(err)
	}
	bUpdates, err := b.Export(loro.UpdatesMode(a.StateVv()))
	if err != nil {
		log.Fatal(err)
	}
	if _, err := b.Import(aUpdates); err != nil {
		log.Fatal(err)
	}
	if _, err := a.Import(bUpdates); err != nil {
		log.Fatal(err)
	}

	// Both docs now contain both keys.
	m := a.GetMap(loro.AsContainerId("m"))
	keys := m.Keys()
	sort.Strings(keys)
	for _, k := range keys {
		v, _ := m.GetInt64(k)
		fmt.Printf("%s=%d\n", k, v)
	}
	// Output:
	// from-a=1
	// from-b=2
}

// LoroTree represents a hierarchy of nodes. Each node is identified by a
// TreeId; pass TreeParentRoot to create a top-level node or TreeParentNode
// to attach a child to an existing one.
func ExampleLoroTree() {
	doc := loro.NewLoroDoc()
	tr := doc.GetTree(loro.AsContainerId("tree"))

	root, err := tr.Create(loro.TreeParentRoot)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tr.Create(loro.TreeParentNode(root)); err != nil {
		log.Fatal(err)
	}
	if _, err := tr.Create(loro.TreeParentNode(root)); err != nil {
		log.Fatal(err)
	}

	var childCount int
	for range tr.AllChildren(loro.TreeParentNode(root)) {
		childCount++
	}
	fmt.Printf("roots=%d children=%d\n", len(tr.Roots()), childCount)
	// Output: roots=1 children=2
}

// Fork clones a document. Edits on the clone don't affect the original until
// the two are explicitly synced again.
func ExampleLoroDoc_Fork() {
	a := loro.NewLoroDoc()
	if err := a.GetMap(loro.AsContainerId("m")).InsertAny("k", "v1"); err != nil {
		log.Fatal(err)
	}

	b := a.Fork()
	if err := b.GetMap(loro.AsContainerId("m")).InsertAny("k", "v2"); err != nil {
		log.Fatal(err)
	}

	av, _ := a.GetMap(loro.AsContainerId("m")).GetString("k")
	bv, _ := b.GetMap(loro.AsContainerId("m")).GetString("k")
	fmt.Printf("a=%s b=%s\n", av, bv)
	// Output: a=v1 b=v2
}
