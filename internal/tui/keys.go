package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap lists every key binding in one place. The help component reads
// the WithHelp texts, and disabled bindings are hidden from help
// automatically, so help always matches what the keys really do.
type keyMap struct {
	NextTab, PrevTab, Tab1, Tab2 key.Binding

	Up, Down, Filter key.Binding

	Add, Details, Unblock, Extend, Select, SelectAll key.Binding
	BlockGroup, UnblockGroup, Undo                   key.Binding

	Reload, Help, Quit, ForceQuit key.Binding
}

func newKeyMap() keyMap {
	k := keyMap{
		NextTab: key.NewBinding(key.WithKeys("tab", "right", "l"), key.WithHelp("tab", "next tab")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab", "left", "h"), key.WithHelp("shift+tab", "prev tab")),
		Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1/2", "go to tab")),
		Tab2:    key.NewBinding(key.WithKeys("2")),

		// Up/Down/Filter are handled by the list itself; listed for help.
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),

		// Blocked tab. Unblock and Extend act on the selected (marked)
		// sites if there are any, otherwise on the one under the cursor.
		Add:       key.NewBinding(key.WithKeys("a", "n"), key.WithHelp("a", "add sites")),
		Details:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		Unblock:   key.NewBinding(key.WithKeys("d", "x", "delete"), key.WithHelp("d", "unblock")),
		Extend:    key.NewBinding(key.WithKeys("e", "+"), key.WithHelp("e", "extend timer")),
		Select:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
		SelectAll: key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "select all/none")),

		// Groups tab.
		BlockGroup:   key.NewBinding(key.WithKeys("enter", "b"), key.WithHelp("enter", "block group")),
		UnblockGroup: key.NewBinding(key.WithKeys("u", "d"), key.WithHelp("u", "unblock group")),

		Undo:      key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "undo last change")),
		Reload:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more")),
		Quit:      key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
	}
	k.update(tabBlocked, true)
	return k
}

// update enables only the bindings that make sense on this tab and mode.
func (k *keyMap) update(t tab, canWrite bool) {
	isList := t == tabBlocked || t == tabGroups
	k.Filter.SetEnabled(isList)
	k.Up.SetEnabled(true)
	k.Down.SetEnabled(true)

	k.Add.SetEnabled(canWrite)
	k.Undo.SetEnabled(canWrite)
	k.Details.SetEnabled(t == tabBlocked) // viewing works read-only too
	k.Unblock.SetEnabled(canWrite && t == tabBlocked)
	k.Extend.SetEnabled(canWrite && t == tabBlocked)
	k.Select.SetEnabled(canWrite && t == tabBlocked)
	k.SelectAll.SetEnabled(canWrite && t == tabBlocked)
	k.BlockGroup.SetEnabled(canWrite && t == tabGroups)
	k.UnblockGroup.SetEnabled(canWrite && t == tabGroups)
}

// ShortHelp is the one-line help at the bottom (help.KeyMap interface).
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Add, k.Details, k.Select, k.Unblock, k.BlockGroup, k.UnblockGroup, k.Filter, k.Help, k.Quit}
}

// FullHelp is shown after pressing "?", in columns.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Filter},
		{k.Add, k.Details, k.Unblock, k.Extend, k.Undo},
		{k.Select, k.SelectAll, k.BlockGroup, k.UnblockGroup},
		{k.NextTab, k.PrevTab, k.Tab1},
		{k.Reload, k.Help, k.Quit},
	}
}
